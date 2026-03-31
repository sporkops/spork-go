package spork

import (
	"context"
	"net/url"
)

// CreateStatusPage creates a new public status page.
func (c *Client) CreateStatusPage(ctx context.Context, sp *StatusPage) (*StatusPage, error) {
	var result StatusPage
	if err := c.doSingle(ctx, "POST", "/status-pages", sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListStatusPages returns all status pages for the authenticated user.
func (c *Client) ListStatusPages(ctx context.Context) ([]StatusPage, error) {
	var result []StatusPage
	if err := c.doList(ctx, "GET", "/status-pages?per_page=100", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetStatusPage returns a single status page by ID.
func (c *Client) GetStatusPage(ctx context.Context, id string) (*StatusPage, error) {
	var result StatusPage
	if err := c.doSingle(ctx, "GET", "/status-pages/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateStatusPage updates a status page by ID (full replace via PUT).
func (c *Client) UpdateStatusPage(ctx context.Context, id string, sp *StatusPage) (*StatusPage, error) {
	var result StatusPage
	if err := c.doSingle(ctx, "PUT", "/status-pages/"+url.PathEscape(id), sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteStatusPage deletes a status page by ID.
func (c *Client) DeleteStatusPage(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/status-pages/"+url.PathEscape(id), nil)
}

// SetCustomDomain sets a custom domain on a status page.
func (c *Client) SetCustomDomain(ctx context.Context, statusPageID, domain string) error {
	body := map[string]string{"domain": domain}
	return c.doNoContent(ctx, "POST", "/status-pages/"+url.PathEscape(statusPageID)+"/custom-domain", body)
}

// RemoveCustomDomain removes the custom domain from a status page.
func (c *Client) RemoveCustomDomain(ctx context.Context, statusPageID string) error {
	return c.doNoContent(ctx, "DELETE", "/status-pages/"+url.PathEscape(statusPageID)+"/custom-domain", nil)
}
