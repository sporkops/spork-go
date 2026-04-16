package spork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateMonitor creates a new uptime monitor.
func (c *Client) CreateMonitor(ctx context.Context, m *Monitor) (*Monitor, error) {
	var result Monitor
	if err := c.doSingle(ctx, "POST", "/monitors", m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMonitors returns every monitor for the authenticated organization,
// transparently paginating through all pages.
//
// Prior to v0.4.0 this method silently returned only the first 100 monitors.
// If you need explicit page control (e.g., for a UI), use ListMonitorsWithOptions.
func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	return collectAll[Monitor](func(opts ListOptions) ([]Monitor, PageMeta, error) {
		return c.ListMonitorsWithOptions(ctx, opts)
	})
}

// ListMonitorsWithOptions returns a single page of monitors along with pagination
// metadata. Use ListMonitors if you want every record.
func (c *Client) ListMonitorsWithOptions(ctx context.Context, opts ListOptions) ([]Monitor, PageMeta, error) {
	var result []Monitor
	path := "/monitors?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetMonitor returns a single monitor by ID.
func (c *Client) GetMonitor(ctx context.Context, id string) (*Monitor, error) {
	var result Monitor
	if err := c.doSingle(ctx, "GET", "/monitors/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMonitor partially updates a monitor by ID using HTTP PATCH.
//
// Only non-zero fields on m are applied; untouched fields retain their
// server-side values. This differs from UpdateAlertChannel and
// UpdateStatusPage, which use PUT (full replacement).
//
// The API also exposes PUT /monitors/{id} for full replacement, which
// requires the owner role; PATCH has no such role restriction. This SDK
// does not currently expose the PUT variant because most callers —
// Terraform and the CLI — supply every writable field anyway, and the
// role-restricted semantics are better modelled by the server rejecting
// the request than by the SDK gating it.
func (c *Client) UpdateMonitor(ctx context.Context, id string, m *Monitor) (*Monitor, error) {
	var result Monitor
	if err := c.doSingle(ctx, "PATCH", "/monitors/"+url.PathEscape(id), m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteMonitor deletes a monitor by ID.
func (c *Client) DeleteMonitor(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/monitors/"+url.PathEscape(id), nil)
}

// GetMonitorResults returns recent check results for a monitor.
func (c *Client) GetMonitorResults(ctx context.Context, id string, limit int) ([]MonitorResult, error) {
	path := fmt.Sprintf("/monitors/%s/results?per_page=%d", url.PathEscape(id), limit)
	var result []MonitorResult
	if _, err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetMonitorResult returns a single check result by monitor ID and result ID.
func (c *Client) GetMonitorResult(ctx context.Context, monitorID, resultID string) (*MonitorResult, error) {
	var result MonitorResult
	path := fmt.Sprintf("/monitors/%s/results/%s", url.PathEscape(monitorID), url.PathEscape(resultID))
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMonitorStats returns 24-hour aggregate statistics for a monitor.
// The server caches stats for 5 minutes.
func (c *Client) GetMonitorStats(ctx context.Context, id string) (*MonitorStats, error) {
	var result MonitorStats
	path := fmt.Sprintf("/monitors/%s/stats", url.PathEscape(id))
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMonitorAuditTrail returns a chronological log of changes made to a
// monitor. Pass limit <= 0 for the server-side default (50). The server
// caps limit at 100. Use the returned next_cursor value to paginate.
func (c *Client) ListMonitorAuditTrail(ctx context.Context, id string, limit int, cursor string) ([]AuditTrailEntry, string, error) {
	path := fmt.Sprintf("/monitors/%s/audit-trail", url.PathEscape(id))
	sep := "?"
	if limit > 0 {
		path += fmt.Sprintf("%slimit=%d", sep, limit)
		sep = "&"
	}
	if cursor != "" {
		path += fmt.Sprintf("%scursor=%s", sep, url.QueryEscape(cursor))
	}
	respBody, _, err := c.rawRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, "", err
	}
	var envelope struct {
		Data       []AuditTrailEntry `json:"data"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, "", fmt.Errorf("parsing audit trail response: %w", err)
	}
	return envelope.Data, envelope.NextCursor, nil
}
