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
	var result []Member
	path := "/members?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// InviteMember invites a user to the organization by email.
func (c *Client) InviteMember(ctx context.Context, input *InviteMemberInput) (*Member, error) {
	var result Member
	if err := c.doSingle(ctx, "POST", "/members/invite", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RemoveMember removes a member from the organization.
func (c *Client) RemoveMember(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/members/"+url.PathEscape(id), nil)
}

// TransferOwnership transfers organization ownership to another member.
func (c *Client) TransferOwnership(ctx context.Context, input *TransferOwnershipInput) (*TransferOwnershipResult, error) {
	var result TransferOwnershipResult
	if err := c.doSingle(ctx, "POST", "/members/transfer-ownership", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListPendingInvites returns pending invitations matching the authenticated
// user's email. Any authenticated user can call this.
func (c *Client) ListPendingInvites(ctx context.Context) ([]Member, error) {
	var result []Member
	if _, err := c.doList(ctx, "GET", "/members/invites", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AcceptInvite accepts a pending organization invitation. The authenticated
// user's email must match the invite email. Users can only belong to one
// organization at a time.
func (c *Client) AcceptInvite(ctx context.Context, input *AcceptInviteInput) (*AcceptInviteResult, error) {
	var result AcceptInviteResult
	if err := c.doSingle(ctx, "POST", "/members/accept", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
