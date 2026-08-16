// Package pricing defines the seam between the appraisal pipeline and
// whatever service supplies comparable listing prices. Implement a new
// Source (e.g. a sold-comps provider) and swap it in via config — no
// handler changes required.
package pricing

import (
	"context"

	"github.com/shopspring/decimal"
)

// Comp is a single comparable listing returned by a Source. Excluded is not
// set by a Source — Summarize sets it when a comp falls outside the IQR
// fences. Outliers are stored and shown dimmed, never discarded.
type Comp struct {
	ExternalID    string
	Title         string
	Price         decimal.Decimal
	Currency      string
	Condition     string
	BuyingOption  string
	ItemURL       string
	ThumbnailURL  string
	SellerCountry string
	Excluded      bool
}

// Query describes a search for comparable listings.
type Query struct {
	Text        string
	CategoryID  string // optional
	Marketplace string // e.g. "EBAY_US"
	Limit       int
}

// Source finds comparable listings for a Query.
type Source interface {
	// Name identifies the source, e.g. "ebay_browse". Persisted on
	// searches.price_source.
	Name() string
	// Find returns comps matching q. It never returns a price as a float —
	// callers must not parse Comp.Price with strconv.ParseFloat.
	Find(ctx context.Context, q Query) ([]Comp, error)
}
