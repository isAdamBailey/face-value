// Package appraisal orchestrates the pipeline that ties vision
// identification, pricing, and storage together for a single search.
package appraisal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/pricing"
	"github.com/isAdamBailey/face-value/backend/internal/vision"
)

const (
	visionTimeout     = 60 * time.Second
	pricingTimeout    = 30 * time.Second
	overallTimeout    = 2 * time.Minute
	markFailedTimeout = 10 * time.Second
)

// Transactor begins a transaction. *pgxpool.Pool satisfies this.
type Transactor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Config holds the tunables for a Service.
type Config struct {
	// Marketplace is the eBay marketplace ID used when a search doesn't
	// specify one, e.g. "EBAY_US".
	Marketplace string
	// CompLimit caps how many comps a single pricing.Source.Find call
	// requests.
	CompLimit int
	// MaxConcurrent bounds how many pipelines run at once, so a single VPS
	// doesn't fan out unbounded goroutines against two rate-limited APIs.
	MaxConcurrent int
}

// Service orchestrates the vision -> pricing -> stats pipeline for
// searches.
type Service struct {
	tx            Transactor
	queries       db.Querier
	vision        vision.Provider
	pricingSource pricing.Source
	marketplace   string
	compLimit     int
	sem           chan struct{}
}

// NewService constructs a Service. tx is used only to begin the
// transaction that atomically inserts comps and completes a search;
// queries handles every other read/write.
func NewService(tx Transactor, queries db.Querier, visionProvider vision.Provider, pricingSource pricing.Source, cfg Config) *Service {
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		tx:            tx,
		queries:       queries,
		vision:        visionProvider,
		pricingSource: pricingSource,
		marketplace:   cfg.Marketplace,
		compLimit:     cfg.CompLimit,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// Start launches the pipeline for searchID in a new goroutine with a
// detached context — the HTTP request that triggered it returns
// immediately (202) and may finish long before the pipeline does.
// imageBytes is the already-preprocessed (resized, EXIF-stripped) image.
func (s *Service) Start(searchID uuid.UUID, imageBytes []byte, mime string) {
	go s.run(searchID, imageBytes, mime)
}

// MarkStaleFailed marks any search stuck in a non-terminal status for more
// than 5 minutes as failed. Call once at boot to clean up rows orphaned by
// a restart mid-pipeline.
func (s *Service) MarkStaleFailed(ctx context.Context) error {
	return s.queries.MarkStaleSearchesFailed(ctx)
}

func (s *Service) run(searchID uuid.UUID, imageBytes []byte, mime string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("appraisal: panic in pipeline for %s: %v", searchID, r)
			s.markFailed(searchID, fmt.Sprintf("internal error: %v", r))
		}
	}()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	ident, model, err := s.identify(ctx, imageBytes, mime)
	if err != nil {
		s.markFailed(searchID, fmt.Sprintf("identify: %v", err))
		return
	}

	if err := s.saveIdentification(ctx, searchID, ident, model); err != nil {
		s.markFailed(searchID, fmt.Sprintf("save identification: %v", err))
		return
	}

	comps, err := s.price(ctx, ident.SearchQuery)
	if err != nil {
		s.markFailed(searchID, fmt.Sprintf("price: %v", err))
		return
	}

	if err := s.completeWithComps(ctx, searchID, comps); err != nil {
		s.markFailed(searchID, fmt.Sprintf("save results: %v", err))
		return
	}
}

func (s *Service) identify(ctx context.Context, img []byte, mime string) (vision.Identification, string, error) {
	ctx, cancel := context.WithTimeout(ctx, visionTimeout)
	defer cancel()
	return s.vision.Identify(ctx, img, mime)
}

func (s *Service) price(ctx context.Context, query string) ([]pricing.Comp, error) {
	ctx, cancel := context.WithTimeout(ctx, pricingTimeout)
	defer cancel()
	return s.pricingSource.Find(ctx, pricing.Query{
		Text:        query,
		Marketplace: s.marketplace,
		Limit:       s.compLimit,
	})
}

func (s *Service) saveIdentification(ctx context.Context, searchID uuid.UUID, ident vision.Identification, model string) error {
	rawJSON, err := json.Marshal(ident)
	if err != nil {
		return fmt.Errorf("marshal identification: %w", err)
	}

	confidence, err := db.ToNumericFromFloat64(ident.Confidence)
	if err != nil {
		return fmt.Errorf("convert confidence: %w", err)
	}

	_, err = s.queries.SetSearchIdentification(ctx, db.SetSearchIdentificationParams{
		ID:             db.ToUUID(searchID),
		Title:          nilIfEmpty(ident.Title),
		Brand:          nilIfEmpty(ident.Brand),
		Model:          nilIfEmpty(ident.Model),
		Category:       nilIfEmpty(ident.Category),
		ConditionNotes: nilIfEmpty(ident.ConditionNotes),
		SearchQuery:    nilIfEmpty(ident.SearchQuery),
		VisionModel:    &model,
		VisionRaw:      rawJSON,
		Confidence:     confidence,
	})
	if err != nil {
		return fmt.Errorf("update search: %w", err)
	}
	return nil
}

// completeWithComps computes stats, inserts comps, and updates the pricing
// rollup in a single transaction — a search must never end up with some
// comps written and no rollup, or a rollup with no comps behind it.
func (s *Service) completeWithComps(ctx context.Context, searchID uuid.UUID, comps []pricing.Comp) error {
	stats := pricing.Summarize(comps)

	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)

	var currency *string
	if len(comps) > 0 {
		currency = &comps[0].Currency
	}

	for _, c := range comps {
		price, err := db.ToNumericFromDecimal(c.Price)
		if err != nil {
			return fmt.Errorf("convert comp %s price: %w", c.ExternalID, err)
		}

		if _, err := qtx.CreateComp(ctx, db.CreateCompParams{
			SearchID:      db.ToUUID(searchID),
			ExternalID:    c.ExternalID,
			Title:         c.Title,
			Price:         price,
			Currency:      c.Currency,
			Condition:     nilIfEmpty(c.Condition),
			BuyingOption:  nilIfEmpty(c.BuyingOption),
			ItemUrl:       nilIfEmpty(c.ItemURL),
			ThumbnailUrl:  nilIfEmpty(c.ThumbnailURL),
			SellerCountry: nilIfEmpty(c.SellerCountry),
			Excluded:      c.Excluded,
		}); err != nil {
			return fmt.Errorf("insert comp %s: %w", c.ExternalID, err)
		}
	}

	mean, err := db.ToNumericFromDecimal(stats.Mean)
	if err != nil {
		return fmt.Errorf("convert mean: %w", err)
	}
	median, err := db.ToNumericFromDecimal(stats.Median)
	if err != nil {
		return fmt.Errorf("convert median: %w", err)
	}
	minPrice, err := db.ToNumericFromDecimal(stats.Min)
	if err != nil {
		return fmt.Errorf("convert min: %w", err)
	}
	maxPrice, err := db.ToNumericFromDecimal(stats.Max)
	if err != nil {
		return fmt.Errorf("convert max: %w", err)
	}
	trimmedMean, err := db.ToNumericFromDecimal(stats.TrimmedMean)
	if err != nil {
		return fmt.Errorf("convert trimmed mean: %w", err)
	}

	compCount := int32(stats.Count)
	priceSource := s.pricingSource.Name()

	if _, err := qtx.SetSearchComplete(ctx, db.SetSearchCompleteParams{
		ID:               db.ToUUID(searchID),
		PriceSource:      &priceSource,
		Currency:         currency,
		CompCount:        &compCount,
		PriceMean:        mean,
		PriceMedian:      median,
		PriceMin:         minPrice,
		PriceMax:         maxPrice,
		PriceTrimmedMean: trimmedMean,
	}); err != nil {
		return fmt.Errorf("set search complete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// markFailed always uses a fresh, short-lived context — by the time a
// pipeline stage has failed, the context it was using may already be
// canceled or past its deadline.
func (s *Service) markFailed(searchID uuid.UUID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), markFailedTimeout)
	defer cancel()

	if _, err := s.queries.SetSearchFailed(ctx, db.SetSearchFailedParams{
		ID:           db.ToUUID(searchID),
		ErrorMessage: &message,
	}); err != nil {
		log.Printf("appraisal: mark search %s failed: %v", searchID, err)
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
