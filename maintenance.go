package spork

import (
	"context"
	"net/url"
)

// CreateMaintenanceWindow creates a new maintenance window.
func (c *Client) CreateMaintenanceWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "POST", "/maintenance-windows", w, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMaintenanceWindows returns every maintenance window for the authenticated
// organization, auto-paginating across all pages.
func (c *Client) ListMaintenanceWindows(ctx context.Context) ([]MaintenanceWindow, error) {
	return collectAll[MaintenanceWindow](func(opts ListOptions) ([]MaintenanceWindow, PageMeta, error) {
		return c.ListMaintenanceWindowsWithOptions(ctx, opts)
	})
}

// ListMaintenanceWindowsWithOptions returns a single page of windows along with
// pagination metadata. Use ListMaintenanceWindows for every record.
func (c *Client) ListMaintenanceWindowsWithOptions(ctx context.Context, opts ListOptions) ([]MaintenanceWindow, PageMeta, error) {
	var result []MaintenanceWindow
	path := "/maintenance-windows?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetMaintenanceWindow returns a single maintenance window by ID.
func (c *Client) GetMaintenanceWindow(ctx context.Context, id string) (*MaintenanceWindow, error) {
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "GET", "/maintenance-windows/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMaintenanceWindow partially updates a window by ID using HTTP PATCH.
// Only non-zero fields on w are applied. Matches UpdateMonitor semantics.
func (c *Client) UpdateMaintenanceWindow(ctx context.Context, id string, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "PATCH", "/maintenance-windows/"+url.PathEscape(id), w, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteMaintenanceWindow deletes a maintenance window by ID.
func (c *Client) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/maintenance-windows/"+url.PathEscape(id), nil)
}

// CancelMaintenanceWindow cancels a scheduled or in-progress window. The
// window stays visible for audit/history but transitions to "cancelled"
// and is ignored by the alert-suppression and check-pause gates.
func (c *Client) CancelMaintenanceWindow(ctx context.Context, id string) (*MaintenanceWindow, error) {
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "POST", "/maintenance-windows/"+url.PathEscape(id)+"/cancel", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
