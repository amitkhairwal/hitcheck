// Package report renders results to the terminal and to files.
package report

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/amitkhairwal/hitcheck/internal/check"
	"github.com/amitkhairwal/hitcheck/internal/load"
)

// CheckTable prints health check results as an aligned terminal table.
func CheckTable(w io.Writer, results []check.Result) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ENDPOINT\tSTATUS\tCODE\tLATENCY\tNOTE")

	var up int
	for _, r := range results {
		state := "DOWN"
		if r.Healthy {
			state = "UP"
			up++
		}

		code := "-"
		if r.Status > 0 {
			code = fmt.Sprintf("%d", r.Status)
		}

		note := r.Err
		if note == "" {
			note = "ok"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			truncate(r.Name, 30), state, code, ms(r.Latency), note)
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d/%d healthy\n", up, len(results))
}

// LoadSummary prints the outcome of a load test.
func LoadSummary(w io.Writer, r load.Result) {
	fmt.Fprintf(w, "Target       %s\n", r.URL)
	fmt.Fprintf(w, "Requests     %d (%d concurrent)\n", r.Requests, r.Concurrency)
	fmt.Fprintf(w, "Duration     %s\n", round(r.Elapsed))
	fmt.Fprintf(w, "Throughput   %.0f req/s\n", r.RPS)

	ok := 0.0
	if r.Requests > 0 {
		ok = float64(r.Success) / float64(r.Requests) * 100
	}
	fmt.Fprintf(w, "Success      %d (%.1f%%)\n", r.Success, ok)
	fmt.Fprintf(w, "Failed       %d\n", r.Failed)

	fmt.Fprintln(w, "\nLatency")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "  min\t%s\n", ms(r.Latency.Min))
	fmt.Fprintf(tw, "  mean\t%s\n", ms(r.Latency.Mean))
	fmt.Fprintf(tw, "  p50\t%s\n", ms(r.Latency.P50))
	fmt.Fprintf(tw, "  p90\t%s\n", ms(r.Latency.P90))
	fmt.Fprintf(tw, "  p95\t%s\n", ms(r.Latency.P95))
	fmt.Fprintf(tw, "  p99\t%s\n", ms(r.Latency.P99))
	fmt.Fprintf(tw, "  max\t%s\n", ms(r.Latency.Max))
	tw.Flush()

	if len(r.StatusCounts) > 0 {
		fmt.Fprintln(w, "\nStatus codes")
		for code, n := range r.StatusCounts {
			fmt.Fprintf(w, "  %-6d %d\n", code, n)
		}
	}
	if len(r.Errors) > 0 {
		fmt.Fprintln(w, "\nErrors")
		for msg, n := range r.Errors {
			fmt.Fprintf(w, "  %-40s %d\n", msg, n)
		}
	}
}

func ms(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
}

func round(d time.Duration) time.Duration {
	return d.Round(time.Millisecond)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
