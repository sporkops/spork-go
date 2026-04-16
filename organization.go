package spork

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetOrganization returns the authenticated user's organization.
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	var result Organization
	if err := c.doSingle(ctx, "GET", "/organization", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOrganizationUsage returns usage metrics for each product subscription.
func (c *Client) GetOrganizationUsage(ctx context.Context) (*OrganizationUsage, error) {
	var result OrganizationUsage
	if err := c.doSingle(ctx, "GET", "/organization/usage", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListRegions returns the monitoring regions available for checks.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var result []Region
	if _, err := c.doList(ctx, "GET", "/regions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ExportOrganizationData downloads a JSON export of all organization data
// (GDPR Art. 20 data portability). Requires the owner role. Collections
// are capped at 1000 items per type.
func (c *Client) ExportOrganizationData(ctx context.Context) (json.RawMessage, error) {
	respBody, _, err := c.rawRequest(ctx, "GET", "/organization/export", nil)
	if err != nil {
		return nil, fmt.Errorf("exporting organization data: %w", err)
	}
	return json.RawMessage(respBody), nil
}

