package spork

import (
	"context"
	"net/url"
)

// ListMembers returns all members of the organization.
func (c *Client) ListMembers(ctx context.Context) ([]Member, error) {
	var result []Member
	if err := c.doList(ctx, "GET", "/members?per_page=100", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
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
