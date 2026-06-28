// Package load implements a worker-pool based load tester.
package load

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/amitkhairwal/hitcheck/internal/stats"
)

// Options controls a single load test run.
type Options struct {
	URL         string
	Method      string
	Requests    int
	Concurrency int
	Timeout     time.Duration
	Headers     map[string]string
}

// Result aggregates everything we learned over the course of a run.
type Result struct {
	URL          string         `json:"url"`
	Requests     int            `json:"requests"`
	Concurrency  int            `json:"concurrency"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	Elapsed      time.Duration  `json:"elapsed_ns"`
	RPS          float64        `json:"requests_per_second"`
	StatusCounts map[int]int    `json:"status_counts"`
	Latency      stats.Summary  `json:"latency"`
	Errors       map[string]int `json:"errors,omitempty"`
}

// one request's outcome, passed back over the results channel.
type sample struct {
	latency time.Duration
	status  int
	err     string
}

// Run fires Options.Requests requests through a pool of Options.Concurrency
// workers and blocks until they all finish or the context is cancelled.
func Run(ctx context.Context, opts Options) Result {
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	if opts.Concurrency > opts.Requests {
		opts.Concurrency = opts.Requests
	}

	client := &http.Client{Timeout: opts.Timeout}

	jobs := make(chan struct{})
	samples := make(chan sample, opts.Concurrency)

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				samples <- do(ctx, client, opts)
			}
		}()
	}

	// Feed the workers, then close the channel so they drain and exit.
	go func() {
		defer close(jobs)
		for i := 0; i < opts.Requests; i++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- struct{}{}:
			}
		}
	}()

	// Close the results channel once every worker has returned.
	go func() {
		wg.Wait()
		close(samples)
	}()

	return collect(opts, samples)
}

func collect(opts Options, samples <-chan sample) Result {
	res := Result{
		URL:          opts.URL,
		Requests:     opts.Requests,
		Concurrency:  opts.Concurrency,
		StatusCounts: map[int]int{},
		Errors:       map[string]int{},
	}

	latencies := make([]time.Duration, 0, opts.Requests)
	start := time.Now()
	for s := range samples {
		latencies = append(latencies, s.latency)
		if s.err != "" {
			res.Failed++
			res.Errors[s.err]++
			continue
		}
		res.StatusCounts[s.status]++
		if s.status >= 200 && s.status < 400 {
			res.Success++
		} else {
			res.Failed++
		}
	}
	res.Elapsed = time.Since(start)

	res.Latency = stats.Summarize(latencies)
	if res.Elapsed > 0 {
		res.RPS = float64(len(latencies)) / res.Elapsed.Seconds()
	}
	return res
}

func do(ctx context.Context, client *http.Client, opts Options) sample {
	method := opts.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, opts.URL, nil)
	if err != nil {
		return sample{err: err.Error()}
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return sample{latency: latency, err: classify(err)}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return sample{latency: latency, status: resp.StatusCode}
}

// classify collapses errors into a few buckets so the report does not list a
// thousand near-identical connection errors.
func classify(err error) string {
	switch {
	case err == context.DeadlineExceeded:
		return "timeout"
	case err == context.Canceled:
		return "cancelled"
	default:
		msg := err.Error()
		if len(msg) > 60 {
			msg = msg[:60]
		}
		return msg
	}
}
