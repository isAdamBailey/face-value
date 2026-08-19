package serpapi

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/isAdamBailey/face-value/backend/internal/ebay"
	"github.com/isAdamBailey/face-value/backend/internal/pricing"
)

// Source implements pricing.Source using SerpApi's eBay search engine
// (scraped active eBay listings, not sold comps).
type Source struct {
	client *Client
}

// NewSource returns a pricing.Source backed by client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Name implements pricing.Source.
func (s *Source) Name() string { return "serpapi_ebay" }

// marketplaceDomain maps an eBay marketplace ID (the same values used by
// ebay.Source/EBAY_MARKETPLACE_ID) to the eBay storefront domain SerpApi
// expects.
var marketplaceDomain = map[string]string{
	"EBAY_US": "ebay.com",
	"EBAY_GB": "ebay.co.uk",
	"EBAY_DE": "ebay.de",
	"EBAY_AU": "ebay.com.au",
	"EBAY_CA": "ebay.ca",
}

func domainFor(marketplace string) string {
	if d, ok := marketplaceDomain[marketplace]; ok {
		return d
	}
	return "ebay.com"
}

// Find implements pricing.Source.
func (s *Source) Find(ctx context.Context, q pricing.Query) ([]pricing.Comp, error) {
	domain := domainFor(q.Marketplace)
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	results, err := s.client.search(ctx, domain, q.Text, limit)
	if err != nil {
		return nil, fmt.Errorf("serpapi ebay search %q: %w", q.Text, err)
	}

	currency := ebay.CurrencyFor(q.Marketplace)
	comps := make([]pricing.Comp, 0, len(results))
	for _, r := range results {
		if r.Bids != "" {
			// A live auction's current bid isn't a stable price signal —
			// mirrors ebay.Source's isAuction exclusion.
			continue
		}

		price, ok := parseMoney(r.Price.Raw)
		if !ok {
			log.Printf("serpapi: skipping comp %q: invalid price %q", r.Link, r.Price.Raw)
			continue
		}

		comps = append(comps, pricing.Comp{
			ExternalID:   itemIDFromLink(r.Link),
			Title:        r.Title,
			Price:        price,
			Currency:     currency,
			Condition:    r.Condition,
			BuyingOption: "FIXED_PRICE",
			ItemURL:      r.Link,
			ThumbnailURL: r.Thumbnail,
		})
	}

	return comps, nil
}

var moneyRe = regexp.MustCompile(`[\d,]+\.?\d*`)

// parseMoney extracts a decimal amount from a raw price string like
// "$19.99" or "US $1,250.00". It never uses strconv.ParseFloat.
func parseMoney(raw string) (decimal.Decimal, bool) {
	match := moneyRe.FindString(raw)
	if match == "" {
		return decimal.Decimal{}, false
	}
	cleaned := strings.ReplaceAll(match, ",", "")
	d, err := decimal.NewFromString(cleaned)
	if err != nil {
		return decimal.Decimal{}, false
	}
	return d, true
}

var itemIDRe = regexp.MustCompile(`/itm/(?:[^/?]+/)?(\d+)`)

// itemIDFromLink extracts the eBay item ID from a listing URL, falling back
// to the full link if the expected /itm/<id> shape isn't found.
func itemIDFromLink(link string) string {
	if m := itemIDRe.FindStringSubmatch(link); len(m) == 2 {
		return m[1]
	}
	return link
}
