package stats

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	// 1ms, 2ms, ... 100ms in a deliberately shuffled order.
	samples := make([]time.Duration, 0, 100)
	for i := 100; i >= 1; i-- {
		samples = append(samples, time.Duration(i)*time.Millisecond)
	}

	got := Summarize(samples)

	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"min", got.Min, 1 * time.Millisecond},
		{"max", got.Max, 100 * time.Millisecond},
		{"p50", got.P50, 50 * time.Millisecond},
		{"p95", got.P95, 95 * time.Millisecond},
		{"p99", got.P99, 99 * time.Millisecond},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	if got.Count != 100 {
		t.Errorf("count = %d, want 100", got.Count)
	}
	// mean of 1..100 is 50.5ms
	if want := 50500 * time.Microsecond; got.Mean != want {
		t.Errorf("mean = %v, want %v", got.Mean, want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	got := Summarize(nil)
	if got.Count != 0 || got.P99 != 0 {
		t.Errorf("empty input should give a zero summary, got %+v", got)
	}
}

func TestSummarizeDoesNotMutateInput(t *testing.T) {
	samples := []time.Duration{3, 1, 2}
	Summarize(samples)
	if samples[0] != 3 {
		t.Errorf("input slice was reordered: %v", samples)
	}
}
