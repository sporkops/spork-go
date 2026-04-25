package spork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateMonitor creates a new uptime monitor.
func (c *Client) CreateMonitor(ctx context.Context, m *Monitor) (*Monitor, error) {
	path, err := c.orgPath(ctx, "/monitors")
	if err != nil {
		return nil, err
	}
	var result Monitor
	if err := c.doSingle(ctx, "POST", path, m, &result); err != nil {
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
	base, err := c.orgPath(ctx, "/monitors")
	if err != nil {
		return nil, PageMeta{}, err
	}
	var result []Monitor
	meta, err := c.doList(ctx, "GET", base+"?"+opts.query(), nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetMonitor returns a single monitor by ID.
func (c *Client) GetMonitor(ctx context.Context, id string) (*Monitor, error) {
	path, err := c.orgPath(ctx, "/monitors/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result Monitor
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
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
	path, err := c.orgPath(ctx, "/monitors/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result Monitor
	if err := c.doSingle(ctx, "PATCH", path, m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteMonitor deletes a monitor by ID.
func (c *Client) DeleteMonitor(ctx context.Context, id string) error {
	path, err := c.orgPath(ctx, "/monitors/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// GetMonitorResults returns recent check results for a monitor.
func (c *Client) GetMonitorResults(ctx context.Context, id string, limit int) ([]MonitorResult, error) {
	path, err := c.orgPath(ctx, fmt.Sprintf("/monitors/%s/results?limit=%d", url.PathEscape(id), limit))
	if err != nil {
		return nil, err
	}
	var result []MonitorResult
	if _, err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetMonitorStats returns 24-hour aggregate statistics for a monitor.
// The server caches stats for 5 minutes.
func (c *Client) GetMonitorStats(ctx context.Context, id string) (*MonitorStats, error) {
	path, err := c.orgPath(ctx, fmt.Sprintf("/monitors/%s/stats", url.PathEscape(id)))
	if err != nil {
		return nil, err
	}
	var result MonitorStats
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMonitorAuditTrail returns a page of audit trail entries for a monitor.
//
// Pagination is cursor-only: pass "" for the first page, and the returned
// nextCursor for each subsequent page. An empty returned cursor signals
// the end. Pass limit <= 0 for the server-side default (50); the server
// caps limit at 100.
//
// The signature deviates from the rest of the SDK (which uses
// ListXWithOptions + PageInfo) because this endpoint's wire envelope
// is non-standard — next_cursor is at the top level rather than nested
// under meta — and this method mirrors that shape directly rather than
// hiding it behind a shim. Callers that want to iterate the full trail
// can loop until nextCursor == "".
func (c *Client) ListMonitorAuditTrail(ctx context.Context, id string, limit int, cursor string) ([]AuditTrailEntry, string, error) {
	path, err := c.orgPath(ctx, fmt.Sprintf("/monitors/%s/audit-trail", url.PathEscape(id)))
	if err != nil {
		return nil, "", err
	}
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
