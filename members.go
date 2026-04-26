package spork

import (
	"context"
	"net/url"
)

// ListMembers returns every member of the organization, transparently
// paginating through all pages.
func (c *Client) ListMembers(ctx context.Context) ([]Member, error) {
	return collectAll[Member](func(opts ListOptions) ([]Member, PageMeta, error) {
		return c.ListMembersWithOptions(ctx, opts)
	})
}

// ListMembersWithOptions returns a single page of members along with pagination
// metadata. Use ListMembers if you want every record.
func (c *Client) ListMembersWithOptions(ctx context.Context, opts ListOptions) ([]Member, PageMeta, error) {
	base, err := c.orgPath(ctx, "/members")
	if err != nil {
		return nil, PageMeta{}, err
	}
	var result []Member
	meta, err := c.doList(ctx, "GET", base+"?"+opts.query(), nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// InviteMember invites a user to the organization by email.
func (c *Client) InviteMember(ctx context.Context, input *InviteMemberInput) (*Member, error) {
	path, err := c.orgPath(ctx, "/members/invite")
	if err != nil {
		return nil, err
	}
	var result Member
	if err := c.doSingle(ctx, "POST", path, input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RemoveMember removes a member from the organization.
func (c *Client) RemoveMember(ctx context.Context, id string) error {
	path, err := c.orgPath(ctx, "/members/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// TransferOwnership transfers organization ownership to another member.
func (c *Client) TransferOwnership(ctx context.Context, input *TransferOwnershipInput) (*TransferOwnershipResult, error) {
	path, err := c.orgPath(ctx, "/members/transfer-ownership")
	if err != nil {
		return nil, err
	}
	var result TransferOwnershipResult
	if err := c.doSingle(ctx, "POST", path, input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListPendingInvites returns pending invitations matching the authenticated
// user's email. User-scoped (not nested under /orgs/{orgID}) — invites
// cross orgs by definition, so the endpoint enumerates them all.
func (c *Client) ListPendingInvites(ctx context.Context) ([]Member, error) {
	var result []Member
	if _, err := c.doList(ctx, "GET", "/members/invites", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AcceptInvite accepts a pending organization invitation. User-scoped:
// the invite token carries the orgID, so this is not nested under
// /orgs/{orgID}. Accepting an invite to a new org adds a membership;
// users can belong to multiple organizations.
func (c *Client) AcceptInvite(ctx context.Context, input *AcceptInviteInput) (*AcceptInviteResult, error) {
	var result AcceptInviteResult
	if err := c.doSingle(ctx, "POST", "/members/accept", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
