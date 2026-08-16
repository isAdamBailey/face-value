package pricing

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
)

func compsFromPrices(prices ...string) []Comp {
	out := make([]Comp, len(prices))
	for i, p := range prices {
		out[i] = Comp{
			ExternalID: fmt.Sprintf("c%d", i),
			Price:      decimal.RequireFromString(p),
			Currency:   "USD",
		}
	}
	return out
}

func excludedCount(comps []Comp) int {
	n := 0
	for _, c := range comps {
		if c.Excluded {
			n++
		}
	}
	return n
}

func TestSummarize(t *testing.T) {
	tests := []struct {
		name              string
		prices            []string
		wantCount         int
		wantLowSample     bool
		wantMean          string
		wantMedian        string
		wantMin           string
		wantMax           string
		wantTrimmedMean   string
		wantExcludedCount int
	}{
		{
			name:          "empty",
			prices:        nil,
			wantCount:     0,
			wantLowSample: true,
		},
		{
			name:            "n=1",
			prices:          []string{"50"},
			wantCount:       1,
			wantLowSample:   true,
			wantMean:        "50",
			wantMedian:      "50",
			wantMin:         "50",
			wantMax:         "50",
			wantTrimmedMean: "50",
		},
		{
			name:            "n=2",
			prices:          []string{"60", "40"},
			wantCount:       2,
			wantLowSample:   true,
			wantMean:        "50",
			wantMedian:      "50",
			wantMin:         "40",
			wantMax:         "60",
			wantTrimmedMean: "50",
		},
		{
			name:              "tight cluster",
			prices:            []string{"12", "10", "14", "11", "13"},
			wantCount:         5,
			wantMean:          "12",
			wantMedian:        "12",
			wantMin:           "10",
			wantMax:           "14",
			wantTrimmedMean:   "12",
			wantExcludedCount: 0,
		},
		{
			name:              "one extreme high outlier",
			prices:            []string{"12", "10", "500", "11", "13"},
			wantCount:         5,
			wantMean:          "109.2",
			wantMedian:        "12",
			wantMin:           "10",
			wantMax:           "500",
			wantTrimmedMean:   "11.5",
			wantExcludedCount: 1,
		},
		{
			// Two genuine clusters (parts vs working units), not
			// outliers relative to each other. IQR trimming correctly
			// does not treat either cluster as noise — the fences are
			// wide because the spread is real — so nothing is excluded
			// and the headline number sits between the clusters rather
			// than blowing up or collapsing to one side.
			name:              "bimodal parts vs working",
			prices:            []string{"5", "6", "7", "95", "100", "105"},
			wantCount:         6,
			wantMean:          "53",
			wantMedian:        "51",
			wantMin:           "5",
			wantMax:           "105",
			wantTrimmedMean:   "53",
			wantExcludedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comps := compsFromPrices(tc.prices...)
			stats := Summarize(comps)

			if stats.Count != tc.wantCount {
				t.Errorf("Count = %d, want %d", stats.Count, tc.wantCount)
			}
			if stats.LowSample != tc.wantLowSample {
				t.Errorf("LowSample = %v, want %v", stats.LowSample, tc.wantLowSample)
			}
			if tc.wantCount == 0 {
				return
			}
			if got := stats.Mean.String(); got != tc.wantMean {
				t.Errorf("Mean = %s, want %s", got, tc.wantMean)
			}
			if got := stats.Median.String(); got != tc.wantMedian {
				t.Errorf("Median = %s, want %s", got, tc.wantMedian)
			}
			if got := stats.Min.String(); got != tc.wantMin {
				t.Errorf("Min = %s, want %s", got, tc.wantMin)
			}
			if got := stats.Max.String(); got != tc.wantMax {
				t.Errorf("Max = %s, want %s", got, tc.wantMax)
			}
			if got := stats.TrimmedMean.String(); got != tc.wantTrimmedMean {
				t.Errorf("TrimmedMean = %s, want %s", got, tc.wantTrimmedMean)
			}
			if got := excludedCount(comps); got != tc.wantExcludedCount {
				t.Errorf("excluded count = %d, want %d", got, tc.wantExcludedCount)
			}
		})
	}
}

func TestSummarize_LowSampleTrimmedMeanIsMedian(t *testing.T) {
	for n := 0; n < lowSampleThreshold; n++ {
		prices := make([]string, n)
		for i := range prices {
			prices[i] = fmt.Sprintf("%d", (i+1)*10)
		}
		comps := compsFromPrices(prices...)
		stats := Summarize(comps)

		if !stats.LowSample {
			t.Errorf("n=%d: LowSample = false, want true", n)
		}
		if n > 0 && !stats.TrimmedMean.Equal(stats.Median) {
			t.Errorf("n=%d: TrimmedMean = %s, want it to equal Median %s", n, stats.TrimmedMean, stats.Median)
		}
	}
}

func TestSummarize_ExcludedCompsAreNotDiscarded(t *testing.T) {
	comps := compsFromPrices("12", "10", "500", "11", "13")
	stats := Summarize(comps)

	if stats.Count != len(comps) {
		t.Errorf("Count = %d, want %d (outliers are flagged, not removed)", stats.Count, len(comps))
	}

	var found bool
	for _, c := range comps {
		if c.Price.Equal(decimal.RequireFromString("500")) {
			found = true
			if !c.Excluded {
				t.Error("outlier comp should be Excluded = true")
			}
		}
	}
	if !found {
		t.Fatal("outlier comp missing from input slice after Summarize")
	}
}
