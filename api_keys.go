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

// ListAPIKeys returns every API key for the authenticated organization,
// transparently paginating through all pages.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	return collectAll[APIKey](func(opts ListOptions) ([]APIKey, PageMeta, error) {
		return c.ListAPIKeysWithOptions(ctx, opts)
	})
}

// ListAPIKeysWithOptions returns a single page of API keys along with pagination
// metadata. Use ListAPIKeys if you want every record.
func (c *Client) ListAPIKeysWithOptions(ctx context.Context, opts ListOptions) ([]APIKey, PageMeta, error) {
	var result []APIKey
	path := "/api-keys?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// DeleteAPIKey deletes an API key by ID.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/api-keys/"+url.PathEscape(id), nil)
}
