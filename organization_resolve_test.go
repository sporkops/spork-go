package spork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// Tests for the multi-org option surface added in v0.8.0:
// WithOrganization, OrganizationID lazy auto-resolve, SetOrganization,
// and the resolver's behaviour when the caller has zero or multiple
// memberships.

func TestOrganizationID_ExplicitWithOrganization(t *testing.T) {
	c := NewClient(WithAPIKey("sk_test"), WithOrganization("org_explicit"))
	id, err := c.OrganizationID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "org_explicit" {
		t.Errorf("OrganizationID = %q, want org_explicit", id)
	}
}

func TestOrganizationID_AutoResolvesSingleMembership(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/users/me/orgs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{{ID: "org_resolved", Role: "owner"}},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	id, err := c.OrganizationID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "org_resolved" {
		t.Errorf("OrganizationID = %q, want org_resolved", id)
	}

	// Second call must reuse the cache — no extra /users/me/orgs hit.
	if _, err := c.OrganizationID(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("expected exactly 1 /users/me/orgs hit, got %d", got)
	}
}

func TestOrganizationID_NoMembershipsReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	_, err := c.OrganizationID(context.Background())
	if err == nil {
		t.Fatal("expected an error for zero memberships")
	}
	// Subsequent calls must keep returning the same error without
	// re-hitting the network.
	_, err2 := c.OrganizationID(context.Background())
	if err2 == nil || err2.Error() != err.Error() {
		t.Errorf("expected cached error on second call, got %v", err2)
	}
}

func TestOrganizationID_PropagatesListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"boom"}}`))
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL),
		// Disable retries so the test sees the 500 immediately.
		WithRetryPolicy(RetryPolicy{}))

	_, err := c.OrganizationID(context.Background())
	if err == nil {
		t.Fatal("expected an error from /users/me/orgs failure")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected wrapped *APIError, got %T: %v", err, err)
	}
}

func TestSetOrganization_OverridesResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{{ID: "org_first", Role: "owner"}},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	// Resolve first → caches "org_first".
	if _, err := c.OrganizationID(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Switch tenants, ensure the new value wins on the next call.
	c.SetOrganization("org_second")
	id, err := c.OrganizationID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "org_second" {
		t.Errorf("after SetOrganization, OrganizationID = %q, want org_second", id)
	}
}

func TestSetOrganization_ClearsCachedError(t *testing.T) {
	// First call gets a hard error (zero memberships); SetOrganization
	// must reset the resolver so a fresh ID can be used afterwards
	// without restarting the process.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	if _, err := c.OrganizationID(context.Background()); err == nil {
		t.Fatal("expected zero-memberships error on first call")
	}

	c.SetOrganization("org_explicit")
	id, err := c.OrganizationID(context.Background())
	if err != nil {
		t.Fatalf("expected error to clear after SetOrganization, got %v", err)
	}
	if id != "org_explicit" {
		t.Errorf("OrganizationID = %q, want org_explicit", id)
	}
}

func TestOrganizationID_ConcurrentCallersResolveOnce(t *testing.T) {
	// Concurrent OrganizationID() calls must serialise on the
	// resolver — only one /users/me/orgs hit, all callers see the
	// same cached ID. Run with -race; without the orgMu fix this
	// trips the race detector.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{{ID: "org_shared", Role: "owner"}},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	const N = 16
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			id, err := c.OrganizationID(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if id != "org_shared" {
				errs <- fmt.Errorf("OrganizationID = %q, want org_shared", id)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("expected exactly 1 /users/me/orgs hit under contention, got %d", got)
	}
}
