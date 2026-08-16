package pricing

import (
	"math"
	"sort"

	"github.com/shopspring/decimal"
)

// lowSampleThreshold is the comp count below which there isn't enough data
// to tell an outlier from the market, so IQR trimming is skipped in favor
// of the median.
const lowSampleThreshold = 3

// iqrFenceMultiplier is the standard Tukey fence multiplier for outlier
// detection.
const iqrFenceMultiplier = 1.5

// Stats is the price rollup computed by Summarize.
type Stats struct {
	Count       int
	Mean        decimal.Decimal
	Median      decimal.Decimal
	Min         decimal.Decimal
	Max         decimal.Decimal
	TrimmedMean decimal.Decimal
	LowSample   bool
}

// Summarize computes price statistics over comps, sorting them by price in
// place and setting Excluded on any comp outside the IQR fences.
//
// eBay active listings have a long right tail (aspirational pricing,
// mislabeled bundles) and a short left tail (parts/broken units). A raw
// mean is consistently wrong high; the median is robust but discards
// information; an IQR-trimmed mean of the non-excluded comps is the
// reasonable middle, and is what Stats.TrimmedMean reports — the headline
// number — once there are enough comps to trust it.
func Summarize(comps []Comp) Stats {
	if len(comps) == 0 {
		return Stats{LowSample: true}
	}

	sort.Slice(comps, func(i, j int) bool {
		return comps[i].Price.LessThan(comps[j].Price)
	})

	prices := make([]decimal.Decimal, len(comps))
	for i := range comps {
		comps[i].Excluded = false
		prices[i] = comps[i].Price
	}

	mean := meanOf(prices)
	median := medianOf(prices)
	minPrice := prices[0]
	maxPrice := prices[len(prices)-1]

	if len(comps) < lowSampleThreshold {
		return Stats{
			Count:       len(comps),
			Mean:        mean,
			Median:      median,
			Min:         minPrice,
			Max:         maxPrice,
			TrimmedMean: median,
			LowSample:   true,
		}
	}

	q1 := percentile(prices, 0.25)
	q3 := percentile(prices, 0.75)
	iqr := q3.Sub(q1)
	fence := iqr.Mul(decimal.NewFromFloat(iqrFenceMultiplier))
	lowerFence := q1.Sub(fence)
	upperFence := q3.Add(fence)

	var included []decimal.Decimal
	for i := range comps {
		p := comps[i].Price
		if p.LessThan(lowerFence) || p.GreaterThan(upperFence) {
			comps[i].Excluded = true
			continue
		}
		included = append(included, p)
	}

	// All comps could theoretically fall outside the fences only if IQR is
	// zero and a value ties the boundary in a way float rounding upsets;
	// fall back to the median rather than divide by zero.
	trimmedMean := median
	if len(included) > 0 {
		trimmedMean = meanOf(included)
	}

	return Stats{
		Count:       len(comps),
		Mean:        mean,
		Median:      median,
		Min:         minPrice,
		Max:         maxPrice,
		TrimmedMean: trimmedMean,
	}
}

func meanOf(prices []decimal.Decimal) decimal.Decimal {
	sum := decimal.Zero
	for _, p := range prices {
		sum = sum.Add(p)
	}
	return sum.Div(decimal.NewFromInt(int64(len(prices))))
}

func medianOf(sorted []decimal.Decimal) decimal.Decimal {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return sorted[n/2-1].Add(sorted[n/2]).Div(decimal.NewFromInt(2))
}

// percentile returns the p-th percentile (0..1) of sorted via linear
// interpolation between closest ranks.
func percentile(sorted []decimal.Decimal, p float64) decimal.Decimal {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}

	rank := p * float64(n-1)
	lowIdx := int(math.Floor(rank))
	highIdx := int(math.Ceil(rank))
	if lowIdx == highIdx {
		return sorted[lowIdx]
	}

	frac := decimal.NewFromFloat(rank - float64(lowIdx))
	low, high := sorted[lowIdx], sorted[highIdx]
	return low.Add(high.Sub(low).Mul(frac))
}
