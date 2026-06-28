// Package check runs concurrent health probes against configured endpoints.
package check

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/amitkhairwal/hitcheck/internal/config"
)

// Result is the outcome of probing a single endpoint.
type Result struct {
	Name      string        `json:"name"`
	URL       string        `json:"url"`
	Status    int           `json:"status"`
	Latency   time.Duration `json:"latency_ns"`
	Healthy   bool          `json:"healthy"`
	Err       string        `json:"error,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
}

// Run probes every endpoint concurrently and returns the results in the same
// order as the config so the output table stays stable between runs.
func Run(ctx context.Context, cfg *config.Config) []Result {
	client := &http.Client{Timeout: cfg.Timeout.Duration}

	results := make([]Result, len(cfg.Endpoints))
	var wg sync.WaitGroup

	for i, ep := range cfg.Endpoints {
		wg.Add(1)
		go func(idx int, ep config.Endpoint) {
			defer wg.Done()
			results[idx] = probe(ctx, client, ep)
		}(i, ep)
	}

	wg.Wait()
	return results
}

func probe(ctx context.Context, client *http.Client, ep config.Endpoint) Result {
	res := Result{
		Name:      ep.Name,
		URL:       ep.URL,
		CheckedAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, ep.HTTPMethod(), ep.URL, nil)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	res.Latency = time.Since(start)
	if err != nil {
		res.Err = cleanErr(err)
		return res
	}
	defer resp.Body.Close()

	// Drain the body so the connection can be reused on the next run.
	io.Copy(io.Discard, resp.Body)

	res.Status = resp.StatusCode
	res.Healthy = resp.StatusCode == ep.WantStatus()
	if !res.Healthy {
		res.Err = fmt.Sprintf("expected %d", ep.WantStatus())
	}
	return res
}

// cleanErr trims the noisy "Get \"url\":" prefix the net/http package adds so
// the table column stays readable.
func cleanErr(err error) string {
	msg := err.Error()
	if i := lastColon(msg); i >= 0 && i+2 < len(msg) {
		return msg[i+2:]
	}
	return msg
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// Healthy reports whether every result in the slice passed.
func Healthy(results []Result) bool {
	for _, r := range results {
		if !r.Healthy {
			return false
		}
	}
	return true
}

// SortByName returns the results sorted alphabetically, used when the caller
// wants a deterministic export regardless of config order.
func SortByName(results []Result) []Result {
	out := make([]Result, len(results))
	copy(out, results)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
