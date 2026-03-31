package spork

import "context"

// GetAccount returns the authenticated user's account info.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	var result Account
	if err := c.doSingle(ctx, "GET", "/me", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
