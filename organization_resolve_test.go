package spork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// --- Multi-org admin / per-call tenant switching ---

func TestForOrg_DoesNotMutateReceiver(t *testing.T) {
	c := NewClient(WithAPIKey("sk_test"), WithOrganization("org_origin"))
	scoped := c.ForOrg("org_other")
	if got := c.ConfiguredOrganizationID(); got != "org_origin" {
		t.Errorf("receiver mutated: ConfiguredOrganizationID = %q, want org_origin", got)
	}
	if got := scoped.ConfiguredOrganizationID(); got != "org_other" {
		t.Errorf("scoped client wrong org: ConfiguredOrganizationID = %q, want org_other", got)
	}
}

func TestForOrg_RoutesPerCallTraffic(t *testing.T) {
	var seenPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []Monitor{},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL), WithOrganization("org_alpha"))

	if _, err := c.ForOrg("org_beta").ListMonitors(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ForOrg("org_gamma").ListMonitors(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"/orgs/org_beta/monitors", "/orgs/org_gamma/monitors"}
	if len(seenPaths) != len(want) {
		t.Fatalf("paths = %v, want %v", seenPaths, want)
	}
	for i := range want {
		if seenPaths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, seenPaths[i], want[i])
		}
	}
}

// --- Loud-fail ambiguous auto-resolve ---

func TestOrganizationID_AmbiguousMultiMembershipErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{
				{ID: "org_a", Role: "owner"},
				{ID: "org_b", Role: "member"},
			},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()
	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL))

	_, err := c.OrganizationID(context.Background())
	if err == nil {
		t.Fatal("expected error for >1 memberships")
	}
	msg := err.Error()
	if !strings.Contains(msg, "org_a") || !strings.Contains(msg, "org_b") {
		t.Errorf("expected error to list candidate orgs, got: %v", err)
	}
	if !strings.Contains(msg, "ForOrg") && !strings.Contains(msg, "WithOrganization") {
		t.Errorf("expected error to point at ForOrg/WithOrganization, got: %v", err)
	}
}

// --- WithEagerOrgResolve ---

func TestWithEagerOrgResolve_ResolvesAtConstruction(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []OrgSummary{{ID: "org_eager", Role: "owner"}},
			"meta": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()

	c := NewClient(WithAPIKey("sk_test"), WithBaseURL(srv.URL), WithEagerOrgResolve(context.Background()))
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected eager resolve to hit /users/me/orgs once at NewClient time, got %d hits", got)
	}
	if got := c.ConfiguredOrganizationID(); got != "org_eager" {
		t.Errorf("ConfiguredOrganizationID = %q, want org_eager", got)
	}
}

// --- WithAPIKey trims hostile surroundings ---

func TestWithAPIKey_TrimsWhitespaceAndQuotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "sk_live_abc", "sk_live_abc"},
		{"surrounding whitespace", "  sk_live_abc \n", "sk_live_abc"},
		{"double-quoted", `"sk_live_abc"`, "sk_live_abc"},
		{"single-quoted", `'sk_live_abc'`, "sk_live_abc"},
		{"whitespace + quotes", `  "sk_live_abc"  `, "sk_live_abc"},
		{"mismatched quotes left alone", `"sk_live_abc'`, `"sk_live_abc'`},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient(WithAPIKey(tc.in))
			if c.Token() != tc.want {
				t.Errorf("Token() = %q, want %q", c.Token(), tc.want)
			}
		})
	}
}
