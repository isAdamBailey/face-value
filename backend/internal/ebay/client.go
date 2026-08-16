// Package ebay implements a client for the eBay Browse API and the
// pricing.Source it satisfies.
package ebay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	maxSearchAttempts = 3
	searchBaseBackoff = 500 * time.Millisecond
)

// Client is a low-level eBay Browse API client handling application-token
// auth and 429 backoff. Use Source to satisfy pricing.Source.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	clientID     string
	clientSecret string

	tokenMu        sync.Mutex
	token          string
	tokenExpiresAt time.Time
}

// NewClient returns a Client for the given base URL (production or sandbox)
// and application credentials.
func NewClient(baseURL, clientID, clientSecret string) *Client {
	return &Client{
		httpClient:   http.DefaultClient,
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// SetHTTPClient overrides the HTTP client used for requests. Intended for
// tests.
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

type itemSummary struct {
	ItemID string `json:"itemId"`
	Title  string `json:"title"`
	Price  struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"price"`
	Condition     string   `json:"condition"`
	BuyingOptions []string `json:"buyingOptions"`
	ItemWebURL    string   `json:"itemWebUrl"`
	Image         struct {
		ImageURL string `json:"imageUrl"`
	} `json:"image"`
	ItemLocation struct {
		Country string `json:"country"`
	} `json:"itemLocation"`
}

type searchResponse struct {
	ItemSummaries []itemSummary `json:"itemSummaries"`
}

// search calls GET /buy/browse/v1/item_summary/search, retrying 429s with
// exponential backoff up to maxSearchAttempts.
func (c *Client) search(ctx context.Context, marketplace, query string, limit int) ([]itemSummary, error) {
	var lastErr error
	for attempt := 0; attempt < maxSearchAttempts; attempt++ {
		if attempt > 0 {
			wait := searchBaseBackoff * time.Duration(math.Pow(2, float64(attempt-1)))
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		items, status, err := c.doSearch(ctx, marketplace, query, limit)
		if err == nil {
			return items, nil
		}
		lastErr = err
		if status != http.StatusTooManyRequests {
			return nil, err
		}
		log.Printf("ebay: search rate-limited (attempt %d/%d), backing off", attempt+1, maxSearchAttempts)
	}
	return nil, fmt.Errorf("ebay: search failed after %d attempts: %w", maxSearchAttempts, lastErr)
}

func (c *Client) doSearch(ctx context.Context, marketplace, query string, limit int) ([]itemSummary, int, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("get access token: %w", err)
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("filter", "buyingOptions:{FIXED_PRICE|BEST_OFFER}")
	q.Set("sort", "price")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/buy/browse/v1/item_summary/search?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", marketplace)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		log.Printf("ebay: rate limit remaining: %s", remaining)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, resp.StatusCode, fmt.Errorf("ebay: rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("ebay: unexpected status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode search response: %w", err)
	}

	return sr.ItemSummaries, resp.StatusCode, nil
}
