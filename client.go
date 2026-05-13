// Package spork provides a Go client for the Spork API.
//
// This is the official Go SDK for Spork (https://sporkops.com), used by both
// the Spork CLI and Terraform provider. It provides typed CRUD operations for
// monitors, alert channels, status pages, incidents, and API keys.
//
// # Authentication
//
// All API calls require an API key (prefixed with "sk_"). Create one at
// https://sporkops.com/settings/api-keys or via the CLI: spork api-key create.
//
// # Quick start
//
//	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))
//
//	// Create a monitor
//	monitor, err := client.CreateMonitor(ctx, &spork.Monitor{
//	    Name:   "API Health",
//	    Target: "https://api.example.com/health",
//	})
//
//	// List all monitors
//	monitors, err := client.ListMonitors(ctx)
//
//	// Handle errors
//	if spork.IsNotFound(err) {
//	    // resource was deleted
//	}
//
// # Configuration
//
// The client supports functional options:
//
//	client := spork.NewClient(
//	    spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
//	    spork.WithBaseURL("https://api.sporkops.com/v1"),  // default
//	    spork.WithUserAgent("my-app/1.0"),
//	    spork.WithHTTPClient(customHTTPClient),
//	)
//
// # Error handling
//
// API errors are returned as *APIError with status code, error code, message,
// and request ID. Use the helper functions IsNotFound, IsUnauthorized,
// IsPaymentRequired, IsForbidden, and IsRateLimited for classification.
//
// # Retries
//
// The client automatically retries transient errors (429, 503, 504) with
// exponential backoff (up to 3 retries). It respects Retry-After headers.
package spork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the default Spork API base URL.
	DefaultBaseURL = "https://api.sporkops.com/v1"

	defaultTimeout  = 30 * time.Second
	maxRetryAfter   = 60
	maxResponseBody = 10 * 1024 * 1024 // 10 MB
)

// Version is the SDK version, used in the User-Agent header.
var Version = "0.8.0"

// DefaultRetryPolicy is the retry policy used when WithRetryPolicy is not set.
// It performs up to 3 attempts with exponential backoff starting at 500ms,
// retrying only on HTTP 429, 503, and 504 responses.
var DefaultRetryPolicy = RetryPolicy{
	MaxRetries: 3,
	BaseDelay:  500 * time.Millisecond,
	RetryOn:    []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
}

// RetryPolicy controls how the SDK retries transient HTTP errors.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts after the initial
	// request. Zero disables retries.
	MaxRetries int
	// BaseDelay is the initial backoff. Each subsequent retry doubles.
	BaseDelay time.Duration
	// RetryOn is the set of HTTP status codes considered retryable. If empty,
	// no retries are performed regardless of MaxRetries.
	RetryOn []int
}

func (p RetryPolicy) shouldRetry(status int) bool {
	for _, s := range p.RetryOn {
		if s == status {
			return true
		}
	}
	return false
}

// HTTPMiddleware wraps an http.RoundTripper. Middleware is composed in the
// order it was registered: the first middleware registered is the outermost,
// so its Do method sees the request before any other middleware and the
// response after all of them. This is the standard onion model used by
// net/http, AWS SDK v2, and most Go middleware libraries.
type HTTPMiddleware func(http.RoundTripper) http.RoundTripper

// Client is an HTTP client for the Spork API.
type Client struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	userAgent   string
	retryPolicy RetryPolicy
	logger      Logger
	// rateLimit is a pointer so a shallow Client copy (ForOrg) shares
	// the rate-limit snapshot with the parent — which is the right
	// semantics: rate limits apply to the API key, not the org. Vet
	// also flags copying a sync.RWMutex by value, which the value
	// form would force.
	rateLimit  *rateLimitStore
	middleware []HTTPMiddleware

	// orgMu guards organizationID, orgResolveOnce, and orgResolveErr
	// so concurrent callers (e.g. parallel Terraform resource ops) can
	// safely read the cached value while another goroutine resolves it
	// or SetOrganization swaps it out. Without the lock, the early
	// "is it already set?" read in OrganizationID would race with the
	// write inside Once.Do, and SetOrganization re-assigning
	// orgResolveOnce mid-flight would be undefined behaviour.
	orgMu          sync.Mutex
	organizationID string
	orgResolveOnce *sync.Once
	orgResolveErr  error
	// eagerResolve / eagerResolveCtx track WithEagerOrgResolve. The
	// actual resolution happens at the end of NewClient so options
	// can be applied in any order before the network call fires.
	eagerResolve    bool
	eagerResolveCtx context.Context
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key (Bearer token) for authentication.
//
// Spork API keys carry a fixed `sk_` prefix. WithAPIKey accepts any
// string for backwards compatibility (bare-token tests, custom proxies,
// etc.) but a leading whitespace or quote character almost always
// signals an env-var paste accident — those are stripped here so a
// `SPORK_API_KEY="\"sk_…\""` mistake doesn't generate confusing 401s
// downstream. Detailed validation is the server's job; the SDK only
// trims hostile-looking surroundings.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		// Drop surrounding whitespace and a single layer of matching
		// quotes — the two leading sources of misconfiguration in
		// the wild. Don't reject empty strings: the existing
		// behaviour returns a clear 401 from the server.
		key = strings.TrimSpace(key)
		if len(key) >= 2 {
			first, last := key[0], key[len(key)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				key = key[1 : len(key)-1]
			}
		}
		c.token = key
	}
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithOrganization sets the organization ID used to scope API calls
// to monitors, alert channels, members, billing, etc. Required (or
// auto-resolvable, see below) for every endpoint nested under
// /orgs/{orgID}/... in the REST surface.
//
// When the option is not set, the SDK lazily resolves the org on the
// first org-scoped call by issuing a single GET /users/me/orgs. The
// resolver expects exactly one membership in the response — API keys
// are bound to one org so this is the common case. Firebase callers
// who belong to multiple orgs MUST pick one explicitly via
// WithOrganization (preferred) or per-call via Client.ForOrg, or the
// resolver will return an error listing the candidate IDs rather
// than picking arbitrarily.
//
// Status-page and incident endpoints under /v1/status-pages/... and
// /v1/incidents/... are also nested under /orgs/{orgID}/ post the
// 2026-04 multi-org refactor, so they require this option too.
func WithOrganization(orgID string) Option {
	return func(c *Client) { c.organizationID = orgID }
}

// WithEagerOrgResolve forces the lazy /users/me/orgs lookup to run at
// NewClient time instead of on the first org-scoped call. Failure
// modes that would normally surface as an error from the first
// monitor / alert-channel / etc. call surface from NewClient instead,
// which is friendlier for tools that wrap each request in a tight
// deadline (Terraform providers especially).
//
// No-op when WithOrganization is supplied — there is nothing to
// resolve. Uses ctx for the resolve request; pass a context with a
// reasonable timeout so a hung backend doesn't stall startup.
func WithEagerOrgResolve(ctx context.Context) Option {
	return func(c *Client) {
		c.eagerResolveCtx = ctx
		c.eagerResolve = true
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithUserAgent sets the User-Agent header prefix.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithRetryPolicy overrides the default retry policy. Callers who want to
// disable retries entirely can pass RetryPolicy{}.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) { c.retryPolicy = p }
}

// WithLogger installs a Logger that will receive internal SDK events
// (retries, rate-limit sleeps, auto-pagination). Nil is treated as "no
// logging" (the default). See the Logger type for conventions.
func WithLogger(l Logger) Option {
	return func(c *Client) {
		if l == nil {
			l = nopLogger{}
		}
		c.logger = l
	}
}

// WithHTTPMiddleware installs a transport-level middleware. Middleware are
// applied in registration order (the first registered is outermost). Use
// this to add request/response logging, distributed tracing, custom auth,
// or fault injection without touching the rest of the SDK.
func WithHTTPMiddleware(mw HTTPMiddleware) Option {
	return func(c *Client) {
		if mw != nil {
			c.middleware = append(c.middleware, mw)
		}
	}
}

// WithEnvDefaults applies sensible defaults from environment variables for
// the three most-common configuration knobs: API key, organization ID,
// and base URL. Options passed AFTER this one override the env-derived
// values, so it composes cleanly:
//
//	// Pure env config (twelve-factor):
//	client := spork.NewClient(spork.WithEnvDefaults())
//
//	// Env defaults with an explicit override for one knob:
//	client := spork.NewClient(
//	    spork.WithEnvDefaults(),
//	    spork.WithOrganization("org_acme"),
//	)
//
// The env vars read (in priority order; first non-empty wins):
//
//   - API key:        SPORK_API_KEY
//   - Organization:   SPORK_ORGANIZATION_ID, then SPORK_ORG_ID
//   - Base URL:       SPORK_BASE_URL
//
// Empty / unset variables leave the corresponding field at its current
// value (the constructor default for fresh clients, or whatever a prior
// option set). The CLI standardizes on SPORK_ORG_ID and we keep that name
// supported for compatibility, but new code should prefer the longer
// SPORK_ORGANIZATION_ID since it matches the API path segment and the
// json tag on response payloads.
//
// We intentionally do NOT auto-read these from inside WithAPIKey /
// WithOrganization / WithBaseURL — silent env consumption is hard to
// reason about when a test forgets to clear state. Making the env-vars
// opt-in via a single, named option keeps the surface explicit.
func WithEnvDefaults() Option {
	return func(c *Client) {
		if v := strings.TrimSpace(os.Getenv("SPORK_API_KEY")); v != "" {
			// Route through WithAPIKey so the same trim / unquote
			// hardening applies to env-derived tokens.
			WithAPIKey(v)(c)
		}
		if v := firstNonEmptyEnv("SPORK_ORGANIZATION_ID", "SPORK_ORG_ID"); v != "" {
			c.organizationID = v
		}
		if v := strings.TrimSpace(os.Getenv("SPORK_BASE_URL")); v != "" {
			c.baseURL = v
		}
	}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

// NewClient creates a new Spork API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:        DefaultBaseURL,
		userAgent:      "spork-go-sdk/" + Version,
		retryPolicy:    DefaultRetryPolicy,
		logger:         nopLogger{},
		rateLimit:      &rateLimitStore{},
		orgResolveOnce: &sync.Once{},
	}
	for _, o := range opts {
		o(c)
	}
	if c.httpClient == nil {
		parsedBase, _ := url.Parse(c.baseURL)
		c.httpClient = &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				if parsedBase != nil && req.URL.Host != parsedBase.Host {
					req.Header.Del("Authorization")
				}
				return nil
			},
		}
	}
	// Wrap the http.Client's transport with any registered middleware. We
	// install the stack once at construction; Middleware added after this
	// is ignored (Option is one-shot by design).
	if len(c.middleware) > 0 {
		base := c.httpClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		for i := len(c.middleware) - 1; i >= 0; i-- {
			base = c.middleware[i](base)
		}
		c.httpClient = &http.Client{
			Timeout:       c.httpClient.Timeout,
			Transport:     base,
			CheckRedirect: c.httpClient.CheckRedirect,
		}
	}
	// WithEagerOrgResolve fires the lazy /users/me/orgs lookup right
	// now so any failure (zero memberships, ambiguous multi-org,
	// network) surfaces from the next OrganizationID-or-org-scoped
	// call rather than from the caller's first business operation.
	// We deliberately swallow the error here — the resolver caches
	// it, so the very next call sees it. NewClient stays
	// (*Client) for backwards compatibility.
	if c.eagerResolve && c.organizationID == "" {
		_, _ = c.OrganizationID(c.eagerResolveCtx)
	}
	return c
}

// Resolve eagerly populates the active organization ID by calling
// /users/me/orgs (no-op when WithOrganization was supplied). Returns
// the resolved error so callers wrapping NewClient can fail fast on
// boot without waiting for the first business call. Subsequent
// OrganizationID-or-org-scoped calls reuse the cached result.
func (c *Client) Resolve(ctx context.Context) error {
	_, err := c.OrganizationID(ctx)
	return err
}

// LastRateLimit returns the most recent rate-limit snapshot observed from
// the server, along with an ok flag that is false if no rate-limit headers
// have been seen yet. The snapshot is updated atomically after every HTTP
// response, including retry attempts and error responses.
func (c *Client) LastRateLimit() (RateLimit, bool) {
	return c.rateLimit.load()
}

// Token returns the configured API key/token. This is useful when the CLI
// needs to pass the token to auth-related endpoints.
func (c *Client) Token() string {
	return c.token
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// ConfiguredOrganizationID returns the organization ID without
// triggering the lazy /users/me/orgs lookup. Returns "" when neither
// WithOrganization nor a previous resolve has set it. Use this when
// you only want to read the current setting — for example, deciding
// whether to display "(auto)" vs the explicit ID in a CLI prompt.
func (c *Client) ConfiguredOrganizationID() string {
	c.orgMu.Lock()
	defer c.orgMu.Unlock()
	return c.organizationID
}

// OrganizationID returns the organization ID the client is scoped to,
// resolving it once via /users/me/orgs if WithOrganization was not
// supplied. The first call may issue a network request; subsequent
// calls return the cached value (or cached error).
//
// The fast-path read and the lazy resolution share orgMu so concurrent
// callers (e.g. parallel Terraform resource ops) cannot race on the
// cached value or trigger two simultaneous /users/me/orgs lookups. The
// lock is released across the network call by snapshotting the
// resolver Once locally — Once.Do still serialises competing
// resolvers without holding the mutex during I/O.
func (c *Client) OrganizationID(ctx context.Context) (string, error) {
	c.orgMu.Lock()
	if id := c.organizationID; id != "" {
		c.orgMu.Unlock()
		return id, nil
	}
	resolveOnce := c.orgResolveOnce
	c.orgMu.Unlock()

	resolveOnce.Do(func() {
		orgs, err := c.ListMyOrgs(ctx)
		c.orgMu.Lock()
		defer c.orgMu.Unlock()
		// SetOrganization may have replaced orgResolveOnce while we
		// were on the wire; in that case the result of this Do is
		// stale and must be discarded. Match the snapshot we took
		// before the call and only commit when it still owns the
		// resolver slot.
		if c.orgResolveOnce != resolveOnce {
			return
		}
		if err != nil {
			c.orgResolveErr = fmt.Errorf("auto-resolving organization: %w", err)
			return
		}
		if len(orgs) == 0 {
			c.orgResolveErr = fmt.Errorf("auto-resolving organization: caller has no organization memberships; create one with CreateOrganization")
			return
		}
		// Refuse to silently pick when the caller belongs to multiple
		// orgs — every non-explicit choice will be wrong half the
		// time and the symptom (data from the wrong org) is hard to
		// diagnose. Force the caller to be intentional via
		// WithOrganization, ForOrg, or SetOrganization.
		// API-key callers always see a single-element list (keys bind
		// to one org), so this branch is only ever hit by Firebase-
		// authenticated tools.
		if len(orgs) > 1 {
			ids := make([]string, 0, len(orgs))
			for _, o := range orgs {
				ids = append(ids, o.ID)
			}
			c.orgResolveErr = fmt.Errorf(
				"auto-resolving organization: caller belongs to %d orgs (%s); pick one explicitly with spork.WithOrganization, client.ForOrg, or client.SetOrganization",
				len(orgs), strings.Join(ids, ", "))
			return
		}
		c.organizationID = orgs[0].ID
	})

	c.orgMu.Lock()
	defer c.orgMu.Unlock()
	if c.orgResolveErr != nil {
		return "", c.orgResolveErr
	}
	return c.organizationID, nil
}

// SetOrganization overrides the active organization ID. Useful when a
// caller wants to switch tenants on a long-lived client (e.g. a CLI
// command that lists every org and runs the same query against each).
//
// Calling SetOrganization after auto-resolution clears the cached
// resolver Once / error so the next call observes the new value.
// Holds orgMu to keep the swap atomic against an in-flight
// OrganizationID resolution; an in-flight Do that lands after the
// swap is discarded by the matching-Once check above.
//
// Deprecated: SetOrganization mutates the receiver in place, which
// races against any org-scoped call already in flight on this client.
// Prefer one of:
//
//   - spork.NewClient(spork.WithOrganization(id), ...) at construction,
//     when the org is known up-front and won't change.
//   - client.ForOrg(id).Whatever(ctx) for per-call switching on a
//     shared client (e.g. listing every customer's monitors).
//
// SetOrganization will be removed in v1.0; the warnings on this
// method exist so callers can migrate before then. The mutation
// remains atomic against the resolver's sync.Once for callers that
// can guarantee no concurrent org-scoped requests are in flight, but
// that's a sharp edge most users don't actually need.
func (c *Client) SetOrganization(orgID string) {
	c.orgMu.Lock()
	defer c.orgMu.Unlock()
	c.organizationID = orgID
	c.orgResolveOnce = &sync.Once{}
	c.orgResolveErr = nil
}

// ForOrg returns a shallow-copied client scoped to the supplied
// organization ID. Use this for per-call tenant switching against a
// shared client — the receiver is unchanged, so concurrent goroutines
// targeting different orgs cannot race on the active org.
//
//	monsA, _ := client.ForOrg("org_acme").ListMonitors(ctx)
//	monsB, _ := client.ForOrg("org_beta").ListMonitors(ctx)
//
// The Stripe-Connect equivalent is `&stripe.RequestParams{StripeAccount}`
// per call. Spork's URL is path-scoped so we shallow-copy the client
// rather than threading the org through every method signature, but
// the use case is the same: a multi-tenant tool (CLI listing every
// customer's monitors, Terraform provider with provider aliases, MSP
// dashboard) can target many orgs without owning N long-lived
// clients.
//
// Internals: rate-limit snapshot, transport middleware, retry policy,
// and the underlying http.Client are all shared. Auto-resolution
// state is reset on the copy — `orgID` is treated as an explicit
// override, so the new client never hits /users/me/orgs.
func (c *Client) ForOrg(orgID string) *Client {
	// Field-by-field copy rather than `dup := *c` because Client
	// embeds sync.Mutex (orgMu) — copying a locked Mutex by value is
	// undefined behaviour and `go vet` rightly flags it. The
	// rateLimit, httpClient, logger, and middleware slice are all
	// pointer-equal so the dup observes the parent's snapshot
	// updates / shared transport stack.
	return &Client{
		baseURL:        c.baseURL,
		token:          c.token,
		httpClient:     c.httpClient,
		userAgent:      c.userAgent,
		retryPolicy:    c.retryPolicy,
		logger:         c.logger,
		rateLimit:      c.rateLimit,
		middleware:     c.middleware,
		organizationID: orgID,
		orgResolveOnce: &sync.Once{},
	}
}

// orgPath prepends /orgs/{orgID} to suffix, resolving the org ID if
// necessary. suffix must start with "/". Used by every org-scoped
// API call.
func (c *Client) orgPath(ctx context.Context, suffix string) (string, error) {
	orgID, err := c.OrganizationID(ctx)
	if err != nil {
		return "", err
	}
	return "/orgs/" + url.PathEscape(orgID) + suffix, nil
}

// doSingle performs a request and unwraps a single-item {data: ...} envelope.
func (c *Client) doSingle(ctx context.Context, method, path string, body, result any) error {
	respBody, _, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if result != nil && len(respBody) > 0 {
		var envelope dataEnvelope
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return fmt.Errorf("parsing response envelope: %w", err)
		}
		if err := json.Unmarshal(envelope.Data, result); err != nil {
			return fmt.Errorf("parsing response data: %w", err)
		}
	}
	return nil
}

// doList performs a request and unwraps a list {data: [...], "meta": {...}} envelope,
// returning the pagination metadata alongside the deserialised data so callers
// (typically auto-paginators) can decide whether to fetch another page.
func (c *Client) doList(ctx context.Context, method, path string, body any, result any) (PageInfo, error) {
	respBody, _, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return PageInfo{}, err
	}
	var envelope listEnvelope
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return PageInfo{}, fmt.Errorf("parsing response envelope: %w", err)
		}
		if result != nil && len(envelope.Data) > 0 {
			if err := json.Unmarshal(envelope.Data, result); err != nil {
				return PageInfo{}, fmt.Errorf("parsing response data: %w", err)
			}
		}
	}
	return PageInfo{
		HasMore:    envelope.Meta.HasMore,
		NextCursor: envelope.Meta.NextCursor,
	}, nil
}

// doNoContent performs a request expecting no response body (e.g., DELETE -> 204).
func (c *Client) doNoContent(ctx context.Context, method, path string, body any) error {
	_, _, err := c.rawRequest(ctx, method, path, body)
	return err
}

// rawRequest performs the HTTP request with retry logic for transient errors.
// It obeys the configured RetryPolicy, surfaces Idempotency-Key values read
// from ctx via WithIdempotencyKey, records rate-limit state after every
// response, and delegates logging to the configured Logger.
func (c *Client) rawRequest(ctx context.Context, method, path string, body any) ([]byte, http.Header, error) {
	var jsonBytes []byte
	if body != nil {
		var err error
		jsonBytes, err = json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling request: %w", err)
		}
	}

	reqURL := c.baseURL + path
	idemKey := IdempotencyKeyFromContext(ctx)
	policy := c.retryPolicy

	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * policy.BaseDelay
			c.logger.Debug("retry %d after %s", attempt, delay)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var reqBody io.Reader
		if jsonBytes != nil {
			reqBody = bytes.NewReader(jsonBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", c.userAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			c.logger.Debug("request error on attempt %d: %v", attempt, err)
			continue
		}

		limitedBody := io.LimitReader(resp.Body, maxResponseBody)
		respBody, err := io.ReadAll(limitedBody)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		// Record rate-limit state before deciding what to do with the
		// response, so that LastRateLimit() reflects the most recent
		// server-side counter even on error responses.
		c.rateLimit.store(parseRateLimit(resp.Header))

		if policy.shouldRetry(resp.StatusCode) && attempt < policy.MaxRetries {
			// Honor server-specified Retry-After before sleeping on the next
			// loop iteration's exponential backoff.
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					if seconds > maxRetryAfter {
						seconds = maxRetryAfter
					}
					if seconds > 0 {
						c.logger.Info("server returned %d; honoring Retry-After=%ds", resp.StatusCode, seconds)
						select {
						case <-ctx.Done():
							return nil, nil, ctx.Err()
						case <-time.After(time.Duration(seconds) * time.Second):
						}
					}
				}
			} else {
				c.logger.Info("retrying after %d response", resp.StatusCode)
			}
			lastErr = &APIError{
				StatusCode: resp.StatusCode,
				Code:       "transient_error",
				Message:    "transient error, retrying",
				RequestID:  resp.Header.Get("X-Request-Id"),
			}
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, nil, parseAPIError(resp.StatusCode, respBody, resp.Header)
		}

		if resp.StatusCode == http.StatusNoContent {
			return nil, resp.Header, nil
		}

		return respBody, resp.Header, nil
	}

	return nil, nil, fmt.Errorf("request failed after %d retries: %w", policy.MaxRetries, lastErr)
}

// dataEnvelope wraps the standard API response: {"data": ...}
type dataEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// listEnvelope wraps the standard API list response: {"data": [...], "meta": {...}}.
type listEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}
