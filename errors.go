package spork

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrNotFound is returned when the requested resource does not exist.
var ErrNotFound = errors.New("resource not found")

// APIError represents a structured error response from the Spork API.
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int `json:"status_code"`
	// Code is a machine-readable error code (e.g., "validation_error").
	Code string `json:"code"`
	// Message is a human-readable error description.
	Message string `json:"message"`
	// RequestID is the X-Request-Id from the response, useful for support.
	RequestID string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("API error (%d): %s (request_id: %s)", e.StatusCode, e.Message, e.RequestID)
	}
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// IsNotFound reports whether err is a 404 Not Found error.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsUnauthorized reports whether err is a 401 Unauthorized error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsPaymentRequired reports whether err is a 402 Payment Required error.
func IsPaymentRequired(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusPaymentRequired
	}
	return false
}

// IsForbidden reports whether err is a 403 Forbidden error.
func IsForbidden(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusForbidden
	}
	return false
}

// IsRateLimited reports whether err is a 429 Too Many Requests error.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// apiErrorEnvelope matches the REST API error format.
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseAPIError extracts a structured error from an API response.
func parseAPIError(statusCode int, body []byte, headers http.Header) error {
	requestID := headers.Get("X-Request-Id")

	if statusCode == http.StatusNotFound {
		return ErrNotFound
	}

	var errResp apiErrorEnvelope
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return &APIError{
			StatusCode: statusCode,
			Code:       errResp.Error.Code,
			Message:    errResp.Error.Message,
			RequestID:  requestID,
		}
	}

	msg := string(body)
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return &APIError{
		StatusCode: statusCode,
		Code:       "unknown",
		Message:    msg,
		RequestID:  requestID,
	}
}
