package ebay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/isAdamBailey/face-value/backend/internal/pricing"
)

func newTestServer(t *testing.T, tokenHandler, searchHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/oauth2/token", tokenHandler)
	mux.HandleFunc("/buy/browse/v1/item_summary/search", searchHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newSource(t *testing.T, srv *httptest.Server) *Source {
	t.Helper()
	client := NewClient(srv.URL, "test-id", "test-secret")
	client.SetHTTPClient(srv.Client())
	return NewSource(client)
}

func tokenOK(expiresIn int64) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"expires_in":   expiresIn,
		})
	}
}

func writeItemSummaries(w http.ResponseWriter, items []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"itemSummaries": items})
}

func fixedPriceItem(id, value, currency string) map[string]any {
	return map[string]any{
		"itemId":        id,
		"title":         "Widget " + id,
		"price":         map[string]string{"value": value, "currency": currency},
		"condition":     "Used",
		"buyingOptions": []string{"FIXED_PRICE"},
		"itemWebUrl":    "https://ebay.example/" + id,
		"image":         map[string]string{"imageUrl": "https://img.example/" + id},
		"itemLocation":  map[string]string{"country": "US"},
	}
}

func TestSource_Find_HappyPath(t *testing.T) {
	var searchCalls int32
	srv := newTestServer(t, tokenOK(7200), func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&searchCalls, 1)
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("X-EBAY-C-MARKETPLACE-ID"); got != "EBAY_US" {
			t.Errorf("marketplace header = %q, want EBAY_US", got)
		}
		writeItemSummaries(w, []map[string]any{fixedPriceItem("1", "19.99", "USD")})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_US", Limit: 50})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1", len(comps))
	}
	if got := comps[0].Price.String(); got != "19.99" {
		t.Errorf("price = %s, want 19.99", got)
	}
	if comps[0].ExternalID != "1" {
		t.Errorf("ExternalID = %q, want 1", comps[0].ExternalID)
	}
	if searchCalls != 1 {
		t.Errorf("search calls = %d, want 1", searchCalls)
	}
}

func TestSource_Find_ExcludesAuctionsAndMixedCurrency(t *testing.T) {
	srv := newTestServer(t, tokenOK(7200), func(w http.ResponseWriter, _ *http.Request) {
		writeItemSummaries(w, []map[string]any{
			fixedPriceItem("usd-1", "10.00", "USD"),
			{
				"itemId": "auction-1", "title": "Auction item",
				"price":         map[string]string{"value": "999.00", "currency": "USD"},
				"buyingOptions": []string{"AUCTION"},
			},
			fixedPriceItem("eur-1", "9.00", "EUR"),
		})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_US"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1 (only the USD fixed-price comp)", len(comps))
	}
	if comps[0].ExternalID != "usd-1" {
		t.Errorf("ExternalID = %q, want usd-1", comps[0].ExternalID)
	}
}

func TestSource_Find_EmptyResultsRetriesWithShortenedQuery(t *testing.T) {
	var queries []string
	srv := newTestServer(t, tokenOK(7200), func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		queries = append(queries, q)
		if q == "brand model rare edition" {
			writeItemSummaries(w, nil)
			return
		}
		writeItemSummaries(w, []map[string]any{fixedPriceItem("1", "42.00", "USD")})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "brand model rare edition", Marketplace: "EBAY_US"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1 after fallback", len(comps))
	}
	if len(queries) != 2 {
		t.Fatalf("search called %d times, want 2 (original + fallback)", len(queries))
	}
	if queries[0] != "brand model rare edition" {
		t.Errorf("first query = %q", queries[0])
	}
	if queries[1] != "brand model rare" {
		t.Errorf("fallback query = %q, want shortened by one keyword", queries[1])
	}
}

func TestSource_Find_RateLimitedThenSuccess(t *testing.T) {
	var attempts int32
	srv := newTestServer(t, tokenOK(7200), func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeItemSummaries(w, []map[string]any{fixedPriceItem("1", "5.00", "USD")})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_US"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1", len(comps))
	}
	if attempts != 2 {
		t.Errorf("search attempts = %d, want 2 (429 then success)", attempts)
	}
}

func TestSource_Find_RefreshesExpiredToken(t *testing.T) {
	var tokenCalls, searchCalls int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		// expires_in=0 forces every subsequent call to treat the cached
		// token as expired, exercising the refresh path without a sleep.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"expires_in":   0,
		})
	}, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&searchCalls, 1)
		writeItemSummaries(w, []map[string]any{fixedPriceItem("1", "1.00", "USD")})
	})

	source := newSource(t, srv)
	ctx := context.Background()
	if _, err := source.Find(ctx, pricing.Query{Text: "widget", Marketplace: "EBAY_US"}); err != nil {
		t.Fatalf("first Find: %v", err)
	}
	if _, err := source.Find(ctx, pricing.Query{Text: "widget", Marketplace: "EBAY_US"}); err != nil {
		t.Fatalf("second Find: %v", err)
	}

	if tokenCalls != 2 {
		t.Errorf("token calls = %d, want 2 (token refetched after expiry)", tokenCalls)
	}
	if searchCalls != 2 {
		t.Errorf("search calls = %d, want 2", searchCalls)
	}
}
