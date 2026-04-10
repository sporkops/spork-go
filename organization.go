package spork

import "context"

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
