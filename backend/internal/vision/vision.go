// Package vision identifies physical objects in photographs via a
// vision-language model, for use as eBay search input.
package vision

import "context"

// lowConfidenceThreshold is the minimum self-reported confidence below
// which an identification is treated as too uncertain to present without
// review.
const lowConfidenceThreshold = 0.35

// Identification is a vision model's read on a photographed item.
type Identification struct {
	Title          string   `json:"title"`
	Brand          string   `json:"brand"`
	Model          string   `json:"model"`
	Category       string   `json:"category"`
	ConditionNotes string   `json:"condition_notes"`
	SearchQuery    string   `json:"search_query"`
	Keywords       []string `json:"keywords"`
	Confidence     float64  `json:"confidence"`
}

// LowConfidence reports whether callers should prompt the user to edit the
// search query and re-run pricing rather than presenting a result
// confidently.
func (i Identification) LowConfidence() bool {
	return i.Confidence < lowConfidenceThreshold || i.SearchQuery == ""
}

// Provider identifies the item in a photograph. It returns the
// identification, the model identifier that produced it (for
// searches.vision_model), and an error.
type Provider interface {
	Identify(ctx context.Context, img []byte, mime string) (Identification, string, error)
}
