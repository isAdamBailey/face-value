package ebay

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/isAdamBailey/face-value/backend/internal/pricing"
)

// Source implements pricing.Source using the eBay Browse API (active
// listings, not sold comps).
type Source struct {
	client *Client
}

// NewSource returns a pricing.Source backed by client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Name implements pricing.Source.
func (s *Source) Name() string { return "ebay_browse" }

// marketplaceCurrency maps an eBay marketplace ID to the currency its
// listings are denominated in. There is no FX conversion in v1: comps in
// any other currency are skipped rather than converted.
var marketplaceCurrency = map[string]string{
	"EBAY_US": "USD",
	"EBAY_GB": "GBP",
	"EBAY_DE": "EUR",
	"EBAY_AU": "AUD",
	"EBAY_CA": "CAD",
}

func currencyFor(marketplace string) string {
	if c, ok := marketplaceCurrency[marketplace]; ok {
		return c
	}
	return "USD"
}

// Find implements pricing.Source. If the initial search returns no results,
// it retries once with a shortened query (dropping the last keyword) before
// giving up.
func (s *Source) Find(ctx context.Context, q pricing.Query) ([]pricing.Comp, error) {
	marketplace := q.Marketplace
	if marketplace == "" {
		marketplace = "EBAY_US"
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	items, err := s.client.search(ctx, marketplace, q.Text, limit)
	if err != nil {
		return nil, fmt.Errorf("ebay search %q: %w", q.Text, err)
	}

	if len(items) == 0 {
		if fallback := shortenQuery(q.Text); fallback != "" {
			log.Printf("ebay: no results for %q, retrying with %q", q.Text, fallback)
			items, err = s.client.search(ctx, marketplace, fallback, limit)
			if err != nil {
				return nil, fmt.Errorf("ebay search %q (fallback for %q): %w", fallback, q.Text, err)
			}
		} else {
			log.Printf("ebay: no results for %q, no shorter query to retry", q.Text)
		}
	}

	expectedCurrency := currencyFor(marketplace)
	comps := make([]pricing.Comp, 0, len(items))
	for _, item := range items {
		if isAuction(item.BuyingOptions) {
			continue
		}
		if item.Price.Currency != expectedCurrency {
			continue
		}

		price, err := decimal.NewFromString(item.Price.Value)
		if err != nil {
			log.Printf("ebay: skipping comp %s: invalid price %q", item.ItemID, item.Price.Value)
			continue
		}

		comps = append(comps, pricing.Comp{
			ExternalID:    item.ItemID,
			Title:         item.Title,
			Price:         price,
			Currency:      item.Price.Currency,
			Condition:     item.Condition,
			BuyingOption:  primaryBuyingOption(item.BuyingOptions),
			ItemURL:       item.ItemWebURL,
			ThumbnailURL:  item.Image.ImageURL,
			SellerCountry: item.ItemLocation.Country,
		})
	}

	return comps, nil
}

// isAuction reports whether buyingOptions marks a listing as a live auction.
// A current bid is not a stable price signal, so these are excluded even
// when the search filter already requests FIXED_PRICE|BEST_OFFER.
func isAuction(buyingOptions []string) bool {
	for _, o := range buyingOptions {
		if o == "AUCTION" {
			return true
		}
	}
	return false
}

func primaryBuyingOption(buyingOptions []string) string {
	if len(buyingOptions) == 0 {
		return ""
	}
	return buyingOptions[0]
}

// shortenQuery drops the last keyword from q, or returns "" if q is already
// a single word.
func shortenQuery(q string) string {
	words := strings.Fields(q)
	if len(words) <= 1 {
		return ""
	}
	return strings.Join(words[:len(words)-1], " ")
}
