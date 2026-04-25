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
	"strconv"
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
	rateLimit   rateLimitStore
	middleware  []HTTPMiddleware

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
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key (Bearer token) for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.token = key }
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
// first org-scoped call by issuing a single GET /users/me/orgs and
// using the first organization in the response. That works
// transparently for API-key callers — keys are bound to one org so
// the listing returns exactly one record. Firebase-authenticated
// callers who belong to several orgs should pick one explicitly with
// WithOrganization or via SetOrganization rather than relying on the
// auto-resolved choice.
//
// Status-page and incident endpoints are NOT org-scoped in the URL
// (they live at /v1/status-pages/... and /v1/incidents/...) and do
// not require this option to function.
func WithOrganization(orgID string) Option {
	return func(c *Client) { c.organizationID = orgID }
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

// NewClient creates a new Spork API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:        DefaultBaseURL,
		userAgent:      "spork-go-sdk/" + Version,
		retryPolicy:    DefaultRetryPolicy,
		logger:         nopLogger{},
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
	return c
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
func (c *Client) SetOrganization(orgID string) {
	c.orgMu.Lock()
	defer c.orgMu.Unlock()
	c.organizationID = orgID
	c.orgResolveOnce = &sync.Once{}
	c.orgResolveErr = nil
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
