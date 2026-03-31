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
	"time"
)

const (
	// DefaultBaseURL is the default Spork API base URL.
	DefaultBaseURL = "https://api.sporkops.com/v1"

	defaultTimeout  = 30 * time.Second
	maxRetries      = 3
	baseDelay       = 500 * time.Millisecond
	maxRetryAfter   = 60
	maxResponseBody = 10 * 1024 * 1024 // 10 MB
)

// Version is the SDK version, used in the User-Agent header.
var Version = "0.1.0"

// Client is an HTTP client for the Spork API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	userAgent  string
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

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithUserAgent sets the User-Agent header prefix.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// NewClient creates a new Spork API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:   DefaultBaseURL,
		userAgent: "spork-go-sdk/" + Version,
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
	return c
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

// doList performs a request and unwraps a list {data: [...], "meta": {...}} envelope.
func (c *Client) doList(ctx context.Context, method, path string, body any, result any) error {
	respBody, _, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if result != nil && len(respBody) > 0 {
		var envelope listEnvelope
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return fmt.Errorf("parsing response envelope: %w", err)
		}
		if err := json.Unmarshal(envelope.Data, result); err != nil {
			return fmt.Errorf("parsing response data: %w", err)
		}
	}
	return nil
}

// doNoContent performs a request expecting no response body (e.g., DELETE -> 204).
func (c *Client) doNoContent(ctx context.Context, method, path string, body any) error {
	_, _, err := c.rawRequest(ctx, method, path, body)
	return err
}

// rawRequest performs the HTTP request with retry logic for transient errors.
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

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
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

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		limitedBody := io.LimitReader(resp.Body, maxResponseBody)
		respBody, err := io.ReadAll(limitedBody)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		// Retry on transient errors
		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					if seconds > maxRetryAfter {
						seconds = maxRetryAfter
					}
					if seconds > 0 {
						select {
						case <-ctx.Done():
							return nil, nil, ctx.Err()
						case <-time.After(time.Duration(seconds) * time.Second):
						}
					}
				}
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

	return nil, nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

// dataEnvelope wraps the standard API response: {"data": ...}
type dataEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// listEnvelope wraps the standard API list response: {"data": [...], "meta": {...}}
type listEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}
