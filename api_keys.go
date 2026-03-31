package spork

import (
	"context"
	"net/url"
)

// CreateAPIKeyInput is the request body for creating an API key.
type CreateAPIKeyInput struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days,omitempty"`
}

// CreateAPIKey creates a new API key. Set ExpiresInDays to nil for no expiry.
func (c *Client) CreateAPIKey(ctx context.Context, input *CreateAPIKeyInput) (*APIKey, error) {
	var result APIKey
	if err := c.doSingle(ctx, "POST", "/api-keys", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAPIKeys returns all API keys for the authenticated user.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	var result []APIKey
	if err := c.doList(ctx, "GET", "/api-keys?per_page=100", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteAPIKey deletes an API key by ID.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/api-keys/"+url.PathEscape(id), nil)
}
