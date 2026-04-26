package spork

import (
	"context"
	"net/url"
)

// CreateMaintenanceWindow creates a new maintenance window.
func (c *Client) CreateMaintenanceWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	path, err := c.orgPath(ctx, "/maintenance-windows")
	if err != nil {
		return nil, err
	}
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "POST", path, w, &result); err != nil {
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
	base, err := c.orgPath(ctx, "/maintenance-windows")
	if err != nil {
		return nil, PageMeta{}, err
	}
	var result []MaintenanceWindow
	meta, err := c.doList(ctx, "GET", base+"?"+opts.query(), nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetMaintenanceWindow returns a single maintenance window by ID.
func (c *Client) GetMaintenanceWindow(ctx context.Context, id string) (*MaintenanceWindow, error) {
	path, err := c.orgPath(ctx, "/maintenance-windows/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMaintenanceWindow partially updates a window by ID using HTTP PATCH.
// Only non-zero fields on w are applied. Matches UpdateMonitor semantics.
func (c *Client) UpdateMaintenanceWindow(ctx context.Context, id string, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	path, err := c.orgPath(ctx, "/maintenance-windows/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "PATCH", path, w, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteMaintenanceWindow deletes a maintenance window by ID.
func (c *Client) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	path, err := c.orgPath(ctx, "/maintenance-windows/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// CancelMaintenanceWindow cancels a scheduled or in-progress window. The
// window stays visible for audit/history but transitions to "cancelled"
// and is ignored by the alert-suppression and check-pause gates.
func (c *Client) CancelMaintenanceWindow(ctx context.Context, id string) (*MaintenanceWindow, error) {
	path, err := c.orgPath(ctx, "/maintenance-windows/"+url.PathEscape(id)+"/cancel")
	if err != nil {
		return nil, err
	}
	var result MaintenanceWindow
	if err := c.doSingle(ctx, "POST", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
