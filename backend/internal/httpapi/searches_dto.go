package httpapi

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/isAdamBailey/face-value/backend/internal/db"
)

// searchSummary is one row in the list/grid response.
type searchSummary struct {
	ID               string  `json:"id"`
	Status           string  `json:"status"`
	ErrorMessage     *string `json:"error_message,omitempty"`
	ImageURL         string  `json:"image_url"`
	Title            *string `json:"title,omitempty"`
	Brand            *string `json:"brand,omitempty"`
	Model            *string `json:"model,omitempty"`
	CompCount        *int32  `json:"comp_count,omitempty"`
	Currency         *string `json:"currency,omitempty"`
	PriceTrimmedMean *string `json:"price_trimmed_mean,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

// compDTO is one comparable listing in a search detail response. Excluded
// (outlier) comps are included, not filtered out — the UI shows them
// dimmed with a toggle, per spec.
type compDTO struct {
	ExternalID    string  `json:"external_id"`
	Title         string  `json:"title"`
	Price         string  `json:"price"`
	Currency      string  `json:"currency"`
	Condition     *string `json:"condition,omitempty"`
	BuyingOption  *string `json:"buying_option,omitempty"`
	ItemURL       *string `json:"item_url,omitempty"`
	ThumbnailURL  *string `json:"thumbnail_url,omitempty"`
	SellerCountry *string `json:"seller_country,omitempty"`
	Excluded      bool    `json:"excluded"`
}

// searchDetail is the full response for GET /api/searches/{id}, and the
// poll target while a search is pending/identifying/pricing.
type searchDetail struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	ErrorMessage     *string   `json:"error_message,omitempty"`
	ImageURL         string    `json:"image_url"`
	Title            *string   `json:"title,omitempty"`
	Brand            *string   `json:"brand,omitempty"`
	Model            *string   `json:"model,omitempty"`
	Category         *string   `json:"category,omitempty"`
	ConditionNotes   *string   `json:"condition_notes,omitempty"`
	SearchQuery      *string   `json:"search_query,omitempty"`
	Confidence       *string   `json:"confidence,omitempty"`
	LowConfidence    bool      `json:"low_confidence"`
	PriceSource      *string   `json:"price_source,omitempty"`
	Currency         *string   `json:"currency,omitempty"`
	CompCount        *int32    `json:"comp_count,omitempty"`
	PriceMean        *string   `json:"price_mean,omitempty"`
	PriceMedian      *string   `json:"price_median,omitempty"`
	PriceMin         *string   `json:"price_min,omitempty"`
	PriceMax         *string   `json:"price_max,omitempty"`
	PriceTrimmedMean *string   `json:"price_trimmed_mean,omitempty"`
	Comps            []compDTO `json:"comps"`
	CreatedAt        string    `json:"created_at"`
	CompletedAt      *string   `json:"completed_at,omitempty"`
}

const lowConfidenceThreshold = 0.35

// imageURLFor returns a presigned GET URL for a search's image, logging
// and returning "" on failure rather than failing the whole response — a
// broken thumbnail is recoverable by a refetch, a 500 on the whole list
// isn't.
func (h *Handler) imageURLFor(ctx context.Context, imageKey string) string {
	url, err := h.imageStore.URL(ctx, imageKey)
	if err != nil {
		log.Printf("httpapi: presign image url for %q: %v", imageKey, err)
		return ""
	}
	return url
}

func (h *Handler) searchSummaryFromRow(ctx context.Context, row db.Search) searchSummary {
	return searchSummary{
		ID:               db.FromUUID(row.ID).String(),
		Status:           row.Status,
		ErrorMessage:     row.ErrorMessage,
		ImageURL:         h.imageURLFor(ctx, row.ImageKey),
		Title:            row.Title,
		Brand:            row.Brand,
		Model:            row.Model,
		CompCount:        row.CompCount,
		Currency:         row.Currency,
		PriceTrimmedMean: numericPtrString(row.PriceTrimmedMean),
		CreatedAt:        row.CreatedAt.Time.Format(rfc3339Milli),
	}
}

func (h *Handler) searchDetailFromRow(ctx context.Context, row db.Search, comps []db.Comp) searchDetail {
	detail := searchDetail{
		ID:               db.FromUUID(row.ID).String(),
		Status:           row.Status,
		ErrorMessage:     row.ErrorMessage,
		ImageURL:         h.imageURLFor(ctx, row.ImageKey),
		Title:            row.Title,
		Brand:            row.Brand,
		Model:            row.Model,
		Category:         row.Category,
		ConditionNotes:   row.ConditionNotes,
		SearchQuery:      row.SearchQuery,
		Confidence:       numericPtrString(row.Confidence),
		PriceSource:      row.PriceSource,
		Currency:         row.Currency,
		CompCount:        row.CompCount,
		PriceMean:        numericPtrString(row.PriceMean),
		PriceMedian:      numericPtrString(row.PriceMedian),
		PriceMin:         numericPtrString(row.PriceMin),
		PriceMax:         numericPtrString(row.PriceMax),
		PriceTrimmedMean: numericPtrString(row.PriceTrimmedMean),
		Comps:            make([]compDTO, len(comps)),
		CreatedAt:        row.CreatedAt.Time.Format(rfc3339Milli),
		CompletedAt:      timePtrString(db.FromTimestamptz(row.CompletedAt)),
	}

	if row.Confidence.Valid {
		conf, _ := db.FromNumeric(row.Confidence).Float64()
		emptyQuery := row.SearchQuery == nil || *row.SearchQuery == ""
		detail.LowConfidence = conf < lowConfidenceThreshold || emptyQuery
	}

	for i, c := range comps {
		detail.Comps[i] = compDTO{
			ExternalID:    c.ExternalID,
			Title:         c.Title,
			Price:         db.FromNumeric(c.Price).String(),
			Currency:      c.Currency,
			Condition:     c.Condition,
			BuyingOption:  c.BuyingOption,
			ItemURL:       c.ItemUrl,
			ThumbnailURL:  c.ThumbnailUrl,
			SellerCountry: c.SellerCountry,
			Excluded:      c.Excluded,
		}
	}

	return detail
}

const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

// numericPtrString renders a nullable NUMERIC column as an exact decimal
// string (never a JSON number, which risks float precision loss client
// side), or nil if the column is NULL.
func numericPtrString(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	s := db.FromNumeric(n).String()
	return &s
}

func timePtrString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(rfc3339Milli)
	return &s
}
