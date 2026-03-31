package spork

import (
	"context"
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

// ListMonitors returns all monitors for the authenticated user.
func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	var result []Monitor
	if err := c.doList(ctx, "GET", "/monitors?per_page=100", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetMonitor returns a single monitor by ID.
func (c *Client) GetMonitor(ctx context.Context, id string) (*Monitor, error) {
	var result Monitor
	if err := c.doSingle(ctx, "GET", "/monitors/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMonitor partially updates a monitor by ID.
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
	if err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
