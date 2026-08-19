package serpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/isAdamBailey/face-value/backend/internal/pricing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newSource(t *testing.T, srv *httptest.Server) *Source {
	t.Helper()
	client := NewClient(srv.URL, "test-key")
	client.SetHTTPClient(srv.Client())
	return NewSource(client)
}

func writeResults(w http.ResponseWriter, results []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"organic_results": results})
}

func fixedPriceResult(id, raw string) map[string]any {
	return map[string]any{
		"title":     "Widget " + id,
		"link":      "https://www.ebay.com/itm/" + id,
		"price":     map[string]string{"raw": raw},
		"condition": "Used",
		"thumbnail": "https://img.example/" + id,
	}
}

func TestSource_Find_HappyPath(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("api_key"); got != "test-key" {
			t.Errorf("api_key = %q, want test-key", got)
		}
		if got := r.URL.Query().Get("ebay_domain"); got != "ebay.com" {
			t.Errorf("ebay_domain = %q, want ebay.com", got)
		}
		writeResults(w, []map[string]any{fixedPriceResult("1234567890", "$19.99")})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_US"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1", len(comps))
	}
	if got := comps[0].Price.String(); got != "19.99" {
		t.Errorf("price = %s, want 19.99", got)
	}
	if comps[0].ExternalID != "1234567890" {
		t.Errorf("ExternalID = %q, want 1234567890", comps[0].ExternalID)
	}
	if comps[0].Currency != "USD" {
		t.Errorf("Currency = %q, want USD", comps[0].Currency)
	}
}

func TestSource_Find_ExcludesAuctionsAndBadPrices(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeResults(w, []map[string]any{
			fixedPriceResult("1", "$10.00"),
			{
				"title": "Auction item",
				"link":  "https://www.ebay.com/itm/2",
				"price": map[string]string{"raw": "$999.00"},
				"bids":  "5 bids",
			},
			{
				"title": "No price",
				"link":  "https://www.ebay.com/itm/3",
				"price": map[string]string{"raw": ""},
			},
		})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_US"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1 (auction and unparsable price excluded)", len(comps))
	}
	if comps[0].ExternalID != "1" {
		t.Errorf("ExternalID = %q, want 1", comps[0].ExternalID)
	}
}

func TestSource_Find_MapsMarketplaceToDomain(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ebay_domain"); got != "ebay.co.uk" {
			t.Errorf("ebay_domain = %q, want ebay.co.uk", got)
		}
		writeResults(w, []map[string]any{fixedPriceResult("1", "£15.00")})
	})

	comps, err := newSource(t, srv).Find(context.Background(), pricing.Query{Text: "widget", Marketplace: "EBAY_GB"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len(comps) = %d, want 1", len(comps))
	}
	if comps[0].Currency != "GBP" {
		t.Errorf("Currency = %q, want GBP", comps[0].Currency)
	}
	if got := comps[0].Price.String(); got != "15" {
		t.Errorf("price = %s, want 15", got)
	}
}
