package load

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunAllSuccess(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Run(context.Background(), Options{
		URL:         srv.URL,
		Requests:    100,
		Concurrency: 10,
		Timeout:     2 * time.Second,
	})

	if got := atomic.LoadInt64(&hits); got != 100 {
		t.Fatalf("server saw %d requests, want 100", got)
	}
	if res.Success != 100 || res.Failed != 0 {
		t.Errorf("success=%d failed=%d, want 100/0", res.Success, res.Failed)
	}
	if res.StatusCounts[200] != 100 {
		t.Errorf("status 200 count = %d, want 100", res.StatusCounts[200])
	}
	if res.Latency.Count != 100 {
		t.Errorf("latency count = %d, want 100", res.Latency.Count)
	}
}

func TestRunCountsServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := Run(context.Background(), Options{
		URL:         srv.URL,
		Requests:    20,
		Concurrency: 5,
		Timeout:     2 * time.Second,
	})

	if res.Success != 0 || res.Failed != 20 {
		t.Errorf("success=%d failed=%d, want 0/20", res.Success, res.Failed)
	}
	if res.StatusCounts[500] != 20 {
		t.Errorf("status 500 count = %d, want 20", res.StatusCounts[500])
	}
}

func TestConcurrencyClampedToRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	res := Run(context.Background(), Options{
		URL:         srv.URL,
		Requests:    3,
		Concurrency: 50,
		Timeout:     time.Second,
	})

	if res.Concurrency != 3 {
		t.Errorf("concurrency = %d, want it clamped to 3", res.Concurrency)
	}
}
