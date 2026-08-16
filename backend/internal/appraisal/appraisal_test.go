package appraisal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/pricing"
	"github.com/isAdamBailey/face-value/backend/internal/vision"
)

// fakeQuerier is a minimal db.Querier that only implements the methods the
// appraisal pipeline actually calls; every other method panics if hit,
// which would indicate the pipeline reached further than a test expects.
type fakeQuerier struct {
	db.Querier

	mu sync.Mutex

	identifyCalls []db.SetSearchIdentificationParams
	failedCalls   []db.SetSearchFailedParams

	identifyErr error
}

func (f *fakeQuerier) SetSearchIdentification(_ context.Context, arg db.SetSearchIdentificationParams) (db.Search, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.identifyCalls = append(f.identifyCalls, arg)
	if f.identifyErr != nil {
		return db.Search{}, f.identifyErr
	}
	return db.Search{ID: arg.ID}, nil
}

func (f *fakeQuerier) SetSearchFailed(_ context.Context, arg db.SetSearchFailedParams) (db.Search, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failedCalls = append(f.failedCalls, arg)
	return db.Search{ID: arg.ID}, nil
}

func (f *fakeQuerier) lastFailedMessage() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.failedCalls) == 0 {
		return ""
	}
	last := f.failedCalls[len(f.failedCalls)-1]
	if last.ErrorMessage == nil {
		return ""
	}
	return *last.ErrorMessage
}

func (f *fakeQuerier) failedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.failedCalls)
}

type fakeVisionProvider struct {
	ident vision.Identification
	model string
	err   error
	delay time.Duration
}

func (p *fakeVisionProvider) Identify(ctx context.Context, _ []byte, _ string) (vision.Identification, string, error) {
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return vision.Identification{}, "", ctx.Err()
		}
	}
	if p.err != nil {
		return vision.Identification{}, "", p.err
	}
	return p.ident, p.model, nil
}

type panicVisionProvider struct{}

func (panicVisionProvider) Identify(context.Context, []byte, string) (vision.Identification, string, error) {
	panic("boom")
}

type fakePricingSource struct {
	comps []pricing.Comp
	err   error
}

func (s *fakePricingSource) Name() string { return "fake_source" }

func (s *fakePricingSource) Find(context.Context, pricing.Query) ([]pricing.Comp, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.comps, nil
}

// failingTransactor errors on Begin, so the transactional insert-comps path
// can be exercised without a real database.
type failingTransactor struct {
	err error
}

func (t *failingTransactor) Begin(context.Context) (pgx.Tx, error) {
	return nil, t.err
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestService_Run_IdentifyErrorMarksFailed(t *testing.T) {
	q := &fakeQuerier{}
	svc := NewService(&failingTransactor{}, q, &fakeVisionProvider{err: errors.New("vision unavailable")}, &fakePricingSource{}, Config{MaxConcurrent: 1})

	svc.Start(uuid.New(), []byte("img"), "image/jpeg")

	waitFor(t, func() bool { return q.failedCount() == 1 })
	if msg := q.lastFailedMessage(); msg == "" {
		t.Error("expected a non-empty error_message")
	}
}

func TestService_Run_PricingErrorMarksFailed(t *testing.T) {
	q := &fakeQuerier{}
	svc := NewService(&failingTransactor{}, q,
		&fakeVisionProvider{ident: vision.Identification{SearchQuery: "widget"}, model: "test-model"},
		&fakePricingSource{err: errors.New("ebay down")},
		Config{MaxConcurrent: 1})

	svc.Start(uuid.New(), []byte("img"), "image/jpeg")

	waitFor(t, func() bool { return q.failedCount() == 1 })
	if len(q.identifyCalls) != 1 {
		t.Errorf("identify calls = %d, want 1 (identification should still be saved before pricing fails)", len(q.identifyCalls))
	}
}

func TestService_Run_TransactionBeginErrorMarksFailed(t *testing.T) {
	q := &fakeQuerier{}
	svc := NewService(&failingTransactor{err: errors.New("pool exhausted")}, q,
		&fakeVisionProvider{ident: vision.Identification{SearchQuery: "widget"}, model: "test-model"},
		&fakePricingSource{comps: []pricing.Comp{{ExternalID: "1", Title: "Widget", Price: decimal.RequireFromString("10.00"), Currency: "USD"}}},
		Config{MaxConcurrent: 1})

	svc.Start(uuid.New(), []byte("img"), "image/jpeg")

	waitFor(t, func() bool { return q.failedCount() == 1 })
}

func TestService_Run_PanicIsRecoveredAndMarksFailed(t *testing.T) {
	q := &fakeQuerier{}
	svc := NewService(&failingTransactor{}, q, panicVisionProvider{}, &fakePricingSource{}, Config{MaxConcurrent: 1})

	svc.Start(uuid.New(), []byte("img"), "image/jpeg")

	waitFor(t, func() bool { return q.failedCount() == 1 })
}

func TestService_Run_ConcurrencyIsBounded(t *testing.T) {
	q := &fakeQuerier{}
	var running, maxRunning int32
	var mu sync.Mutex

	provider := &trackingVisionProvider{
		delay: 80 * time.Millisecond,
		onStart: func() {
			mu.Lock()
			running++
			if running > maxRunning {
				maxRunning = running
			}
			mu.Unlock()
		},
		onDone: func() {
			mu.Lock()
			running--
			mu.Unlock()
		},
	}

	svc := NewService(&failingTransactor{}, q, provider, &fakePricingSource{}, Config{MaxConcurrent: 2})

	for i := 0; i < 5; i++ {
		svc.Start(uuid.New(), []byte("img"), "image/jpeg")
	}

	waitFor(t, func() bool { return q.failedCount() == 5 })

	mu.Lock()
	defer mu.Unlock()
	if maxRunning > 2 {
		t.Errorf("max concurrent pipelines = %d, want <= 2 (MaxConcurrent)", maxRunning)
	}
}

type trackingVisionProvider struct {
	delay   time.Duration
	onStart func()
	onDone  func()
}

func (p *trackingVisionProvider) Identify(ctx context.Context, _ []byte, _ string) (vision.Identification, string, error) {
	p.onStart()
	defer p.onDone()
	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
	}
	return vision.Identification{}, "", errors.New("intentional failure after delay")
}
