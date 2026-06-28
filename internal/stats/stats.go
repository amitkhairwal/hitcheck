// Package stats computes latency summaries from a set of request durations.
package stats

import (
	"sort"
	"time"
)

// Summary holds the aggregate latency numbers we report after a run.
type Summary struct {
	Count int           `json:"count"`
	Min   time.Duration `json:"min_ns"`
	Max   time.Duration `json:"max_ns"`
	Mean  time.Duration `json:"mean_ns"`
	P50   time.Duration `json:"p50_ns"`
	P90   time.Duration `json:"p90_ns"`
	P95   time.Duration `json:"p95_ns"`
	P99   time.Duration `json:"p99_ns"`
}

// Summarize sorts a copy of the samples and computes the summary. The input
// slice is left untouched so callers can reuse it.
func Summarize(samples []time.Duration) Summary {
	if len(samples) == 0 {
		return Summary{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, d := range sorted {
		total += d
	}

	return Summary{
		Count: len(sorted),
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
		Mean:  total / time.Duration(len(sorted)),
		P50:   percentile(sorted, 50),
		P90:   percentile(sorted, 90),
		P95:   percentile(sorted, 95),
		P99:   percentile(sorted, 99),
	}
}

// percentile returns the value at the given percentile using the
// nearest-rank method. sorted must already be in ascending order.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := int(p / 100 * float64(len(sorted)))
	if rank > 0 {
		rank--
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
