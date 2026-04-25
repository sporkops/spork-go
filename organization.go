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
	// Name is an optional free-form display name.
	Name string `json:"name,omitempty"`
	// Slug is an optional URL-friendly identifier (3-63 lowercase
	// alphanumeric/hyphens, starting and ending with an alphanumeric).
	// When set, the slug becomes the org's stored `name` so future
	// dashboard URLs are predictable. Server picks a random
	// `team-xxxxxxxx` slug when both Slug and Name are empty.
	// Mutually exclusive with Name — passing both returns 400.
	Slug string `json:"slug,omitempty"`
}

// UpdateOrganizationInput is the request body for UpdateOrganization.
type UpdateOrganizationInput struct {
	// Name is the new display name. Required, capped at 100 chars,
	// must not contain control characters. Empty string returns 400.
	Name string `json:"name"`
}

// DeleteOrganizationInput is the request body for DeleteOrganization.
// `Confirm` must equal the org ID being deleted; the server rejects
// any other value with 400 confirmation_required so a misrouted
// scripted DELETE in a multi-org pipeline cannot silently nuke the
// wrong tenant.
type DeleteOrganizationInput struct {
	Confirm string `json:"confirm"`
}

// CreateOrganization creates a new organization on the free plan; the
// caller becomes the owner.
//
// Gated by the per-user organization cap. Free / Starter / Pro plans
// each grant one slot; Agency grants five — i.e., a user can only own
// multiple orgs once at least one of them is on the Agency plan. The
// server returns 403 `org_limit_reached` when the cap is hit. Support
// can raise the cap per-user via the `org_limit` entitlement.
//
// Cannot be called with an API key — keys are bound to their home org
// and cannot create new tenants.
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

// UpdateOrganization renames the active organization. Owner-only on
// the server side. Returns the refreshed Organization so callers can
// update local state without a follow-up GetOrganization round-trip.
func (c *Client) UpdateOrganization(ctx context.Context, input *UpdateOrganizationInput) (*Organization, error) {
	if input == nil {
		input = &UpdateOrganizationInput{}
	}
	path, err := c.orgPath(ctx, "")
	if err != nil {
		return nil, err
	}
	var result Organization
	if err := c.doSingle(ctx, "PATCH", path, input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteOrganization permanently removes the active organization.
// Owner-only. The body's `confirm` field must equal the orgID — the
// helper fills it in for you using the configured / resolved org.
//
// Cannot be undone. Server rejects with 409 active_subscription if
// the org still has a paid subscription that hasn't been cancelled
// in the billing portal.
//
// After a successful DELETE the client's cached org ID is now stale.
// Call SetOrganization (or ForOrg) before issuing further org-scoped
// requests, otherwise every subsequent call hits a 404 / 403 for
// the deleted tenant.
func (c *Client) DeleteOrganization(ctx context.Context) error {
	orgID, err := c.OrganizationID(ctx)
	if err != nil {
		return err
	}
	path, err := c.orgPath(ctx, "")
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, &DeleteOrganizationInput{Confirm: orgID})
}

// GetMyMembership returns the caller's Member record in the active
// organization — the canonical "what's my role here?" lookup. Works
// for any authenticated member regardless of role; saves the
// /orgs/{orgID} round-trip when callers only need their own role.
func (c *Client) GetMyMembership(ctx context.Context) (*Member, error) {
	path, err := c.orgPath(ctx, "/members/me")
	if err != nil {
		return nil, err
	}
	var result Member
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
