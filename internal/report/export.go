package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/amitkhairwal/hitcheck/internal/check"
	"github.com/amitkhairwal/hitcheck/internal/load"
)

// WriteJSON marshals any value to a file with indentation.
func WriteJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// CheckCSV writes health check results as a CSV file.
func CheckCSV(path string, results []check.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"name", "url", "status", "latency_ms", "healthy", "error", "checked_at"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range results {
		row := []string{
			r.Name,
			r.URL,
			strconv.Itoa(r.Status),
			fmt.Sprintf("%.2f", float64(r.Latency.Microseconds())/1000),
			strconv.FormatBool(r.Healthy),
			r.Err,
			r.CheckedAt.Format(time.RFC3339),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

// LoadCSV writes a load test summary as a single-row CSV file.
func LoadCSV(path string, r load.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"url", "requests", "concurrency", "success", "failed",
		"rps", "min_ms", "mean_ms", "p50_ms", "p95_ms", "p99_ms", "max_ms"}
	if err := w.Write(header); err != nil {
		return err
	}

	row := []string{
		r.URL,
		strconv.Itoa(r.Requests),
		strconv.Itoa(r.Concurrency),
		strconv.Itoa(r.Success),
		strconv.Itoa(r.Failed),
		fmt.Sprintf("%.1f", r.RPS),
		msFloat(r.Latency.Min),
		msFloat(r.Latency.Mean),
		msFloat(r.Latency.P50),
		msFloat(r.Latency.P95),
		msFloat(r.Latency.P99),
		msFloat(r.Latency.Max),
	}
	if err := w.Write(row); err != nil {
		return err
	}
	return w.Error()
}

func msFloat(d time.Duration) string {
	return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000)
}
