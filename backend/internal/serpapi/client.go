// Package serpapi implements a client for SerpApi's eBay search engine and
// the pricing.Source it satisfies. It exists as a drop-in for ebay.Source:
// same "active eBay listings" semantics, but sourced by scraping eBay search
// result pages instead of calling the eBay Browse API directly — useful when
// an eBay developer application hasn't been approved.
package serpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Client is a low-level client for SerpApi's "ebay" search engine.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient returns a Client for the given SerpApi base URL and API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
	}
}

// SetHTTPClient overrides the HTTP client used for requests. Intended for
// tests.
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

type organicResult struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	Price struct {
		Raw string `json:"raw"`
	} `json:"price"`
	Condition string `json:"condition"`
	Thumbnail string `json:"thumbnail"`
	Bids      string `json:"bids"`
}

type searchResponse struct {
	OrganicResults []organicResult `json:"organic_results"`
	SearchMetadata struct {
		Status string `json:"status"`
	} `json:"search_metadata"`
	Error string `json:"error"`
}

// search calls GET /search.json?engine=ebay for query against ebayDomain
// (e.g. "ebay.com"), returning up to limit results.
func (c *Client) search(ctx context.Context, ebayDomain, query string, limit int) ([]organicResult, error) {
	q := url.Values{}
	q.Set("engine", "ebay")
	q.Set("ebay_domain", ebayDomain)
	q.Set("_nkw", query)
	q.Set("api_key", c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/search.json?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serpapi: unexpected status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	if sr.Error != "" {
		return nil, fmt.Errorf("serpapi: %s", sr.Error)
	}

	results := sr.OrganicResults
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
