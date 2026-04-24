package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoRetriesOn503(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := Do(context.Background(), http.DefaultClient, req, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestDoNoRetryOn200(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := Do(context.Background(), http.DefaultClient, req, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1", attempts.Load())
	}
}

func TestDoNoRetryOn404(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := Do(context.Background(), http.DefaultClient, req, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if attempts.Load() != 1 {
		t.Errorf("attempts on 404 = %d, want 1 (permanent)", attempts.Load())
	}
}

func TestDoRespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any attempt; since attempt 0 uses ctx via req.Clone, it fails immediately.

	req, _ := http.NewRequest("GET", srv.URL, nil)
	_, err := Do(ctx, http.DefaultClient, req, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetryPolicyDefaults(t *testing.T) {
	p := RetryPolicy{}.defaults()
	if p.MaxAttempts < 2 {
		t.Errorf("default MaxAttempts = %d, want >=2", p.MaxAttempts)
	}
	if p.BaseDelay <= 0 {
		t.Errorf("default BaseDelay = %v, want >0", p.BaseDelay)
	}
	if p.MaxDelay < p.BaseDelay {
		t.Errorf("MaxDelay %v < BaseDelay %v", p.MaxDelay, p.BaseDelay)
	}
}
