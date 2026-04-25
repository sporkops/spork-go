package spork

import (
	"context"
	"encoding/json"
)

// GetOrganization returns the named organization, including subscriptions
// and the authenticated principal's role within it. Uses the active
// organization configured via WithOrganization (or auto-resolved).
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	path, err := c.orgPath(ctx, "")
	if err != nil {
		return nil, err
	}
	var result Organization
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOrganizationUsage returns usage metrics for each product subscription
// on the active organization.
func (c *Client) GetOrganizationUsage(ctx context.Context) (*OrganizationUsage, error) {
	path, err := c.orgPath(ctx, "/usage")
	if err != nil {
		return nil, err
	}
	var result OrganizationUsage
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListRegions returns the monitoring regions available for checks.
// User-scoped (the catalogue is the same for every org).
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var result []Region
	if _, err := c.doList(ctx, "GET", "/regions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ExportOrganizationData downloads a JSON export of all data for the
// active organization (GDPR Art. 20 data portability). Requires the
// owner role. Server-side collections are capped at 1000 items per
// type.
//
// The response is the raw export JSON, not the standard {data: ...}
// envelope, so it is returned as json.RawMessage for the caller to
// persist or unmarshal into a shape they define.
func (c *Client) ExportOrganizationData(ctx context.Context) (json.RawMessage, error) {
	path, err := c.orgPath(ctx, "/export")
	if err != nil {
		return nil, err
	}
	respBody, _, err := c.rawRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(respBody), nil
}

// GetMe returns the authenticated principal's account-level identity
// (UID, email, optional admin role). Org-specific data — name, role,
// plan — comes from ListMyOrgs / GetOrganization.
//
// User-scoped: the caller may belong to multiple organizations, but
// the user record itself is the same across all of them.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var result User
	if err := c.doSingle(ctx, "GET", "/users/me", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMyOrgs returns every organization the caller belongs to, along
// with the caller's role in each. When authenticated with an API key,
// the response is a single-element list — the org the key is bound to.
//
// User-scoped: this is the canonical way to discover which org a
// freshly-authenticated client belongs to. The first element is also
// what WithOrganization auto-resolution picks when no explicit org ID
// was supplied.
func (c *Client) ListMyOrgs(ctx context.Context) ([]OrgSummary, error) {
	var result []OrgSummary
	if _, err := c.doList(ctx, "GET", "/users/me/orgs", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateOrganizationInput is the request body for CreateOrganization.
type CreateOrganizationInput struct {
	// Name is an optional human-readable label. If empty, the server
	// picks a default (e.g. "Acme's Org").
	Name string `json:"name,omitempty"`
}

// CreateOrganization creates a new organization on the free plan; the
// caller becomes the owner. Each user is capped at a small number of
// free organizations they own (default 3); paid orgs are uncapped.
//
// Cannot be called with an API key — keys are bound to their home org
// and cannot create new tenants. The server returns 403
// `free_org_limit_reached` if the cap is hit.
//
// The returned OrgSummary is suitable to pass to SetOrganization to
// switch the client onto the freshly-created tenant for subsequent
// org-scoped calls.
func (c *Client) CreateOrganization(ctx context.Context, input *CreateOrganizationInput) (*OrgSummary, error) {
	if input == nil {
		input = &CreateOrganizationInput{}
	}
	var result OrgSummary
	if err := c.doSingle(ctx, "POST", "/orgs", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
