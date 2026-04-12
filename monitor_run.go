package spork

import (
	"context"
	"time"
)

// RunMonitorInput is the request body for RunMonitor. Fields mirror
// Monitor; unset fields take the same defaults `CreateMonitor` uses.
type RunMonitorInput struct {
	Target         string            `json:"target"`
	Type           string            `json:"type,omitempty"`
	Method         string            `json:"method,omitempty"`
	ExpectedStatus int               `json:"expected_status,omitempty"`
	Timeout        int               `json:"timeout,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           string            `json:"body,omitempty"`
	Keyword        string            `json:"keyword,omitempty"`
	KeywordType    string            `json:"keyword_type,omitempty"`
	SSLWarnDays    int               `json:"ssl_warn_days,omitempty"`
}

// RunMonitorResult describes the outcome of a single ephemeral probe.
type RunMonitorResult struct {
	Target         string    `json:"target"`
	Type           string    `json:"type"`
	Region         string    `json:"region"`
	Status         string    `json:"status"`         // "up" or "down"
	HTTPCode       int       `json:"http_code,omitempty"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	Error          string    `json:"error,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

// RunMonitor executes a single ephemeral probe against the supplied
// configuration and returns the result. The server does not persist
// the probe — no monitor is created, no history row is written.
//
// Powers the `spork monitor run` CLI command: iterate on check config
// without polluting a monitor's history.
//
// The probe runs from the API server's own region (single-region only);
// for multi-region parity against a scheduled monitor, create a real
// monitor and wait for a scheduled check.
//
// Example:
//
//	result, err := client.RunMonitor(ctx, &spork.RunMonitorInput{
//	    Target: "https://api.example.com/health",
//	    Type:   "http",
//	})
//	if err != nil { log.Fatal(err) }
//	fmt.Printf("status=%s code=%d dur=%dms\n",
//	    result.Status, result.HTTPCode, result.ResponseTimeMs)
func (c *Client) RunMonitor(ctx context.Context, input *RunMonitorInput) (*RunMonitorResult, error) {
	var result RunMonitorResult
	if err := c.doSingle(ctx, "POST", "/monitors/run", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
