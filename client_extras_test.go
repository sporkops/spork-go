package spork

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithIdempotencyKey_EndToEnd(t *testing.T) {
	var gotKey string
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": Monitor{ID: "mon_1"}})
	})

	ctx := WithIdempotencyKey(t.Context(), "key-abc-123")
	if _, err := c.CreateMonitor(ctx, &Monitor{Name: "x", Target: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if gotKey != "key-abc-123" {
		t.Errorf("expected Idempotency-Key=key-abc-123 on the wire, got %q", gotKey)
	}
}

func TestLastRateLimit_PopulatedFromHeaders(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "37")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10))
		_ = json.NewEncoder(w).Encode(map[string]any{"data": Monitor{ID: "mon_1"}})
	})

	if _, ok := c.LastRateLimit(); ok {
		t.Fatal("expected LastRateLimit to be unset before any request")
	}
	if _, err := c.GetMonitor(t.Context(), "mon_1"); err != nil {
		t.Fatal(err)
	}
	rl, ok := c.LastRateLimit()
	if !ok {
		t.Fatal("expected LastRateLimit to be populated")
	}
	if rl.Limit != 60 || rl.Remaining != 37 || rl.Reset.IsZero() {
		t.Errorf("unexpected snapshot: %+v", rl)
	}
}

func TestWithHTTPMiddleware_SeesRequestAndResponse(t *testing.T) {
	var seenPath, seenMethod, seenAuth string
	mw := func(next http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seenPath = r.URL.Path
			seenMethod = r.Method
			seenAuth = r.Header.Get("Authorization")
			return next.RoundTrip(r)
		})
	}

	var serverHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": Monitor{ID: "mon_1"}})
	}))
	t.Cleanup(srv.Close)
	c := NewClient(
		WithAPIKey("sk_test_key"),
		WithBaseURL(srv.URL),
		WithHTTPMiddleware(mw),
	)

	if _, err := c.GetMonitor(t.Context(), "mon_1"); err != nil {
		t.Fatal(err)
	}
	if serverHits.Load() != 1 {
		t.Errorf("expected server to see exactly one request, got %d", serverHits.Load())
	}
	if seenMethod != "GET" || seenPath != "/monitors/mon_1" {
		t.Errorf("middleware did not see expected request: method=%q path=%q", seenMethod, seenPath)
	}
	if seenAuth != "Bearer sk_test_key" {
		t.Errorf("middleware did not see Authorization header: %q", seenAuth)
	}
}

func TestWithRetryPolicy_RespectsDisabledRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(
		WithAPIKey("sk_test_key"),
		WithBaseURL(srv.URL),
		WithRetryPolicy(RetryPolicy{}),
	)

	_, err := c.GetMonitor(t.Context(), "mon_1")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("with retries disabled, expected exactly 1 attempt, got %d", got)
	}
}

// roundTripFunc is a helper to adapt a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
