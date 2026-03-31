package spork

import (
	"context"
	"net/url"
)

// CreateAlertChannel creates a new alert channel.
func (c *Client) CreateAlertChannel(ctx context.Context, ch *AlertChannel) (*AlertChannel, error) {
	var result AlertChannel
	if err := c.doSingle(ctx, "POST", "/alert-channels", ch, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAlertChannels returns all alert channels for the authenticated user.
func (c *Client) ListAlertChannels(ctx context.Context) ([]AlertChannel, error) {
	var result []AlertChannel
	if err := c.doList(ctx, "GET", "/alert-channels?per_page=100", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAlertChannel returns a single alert channel by ID.
func (c *Client) GetAlertChannel(ctx context.Context, id string) (*AlertChannel, error) {
	var result AlertChannel
	if err := c.doSingle(ctx, "GET", "/alert-channels/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateAlertChannel updates an alert channel by ID.
func (c *Client) UpdateAlertChannel(ctx context.Context, id string, ch *AlertChannel) (*AlertChannel, error) {
	var result AlertChannel
	if err := c.doSingle(ctx, "PUT", "/alert-channels/"+url.PathEscape(id), ch, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteAlertChannel deletes an alert channel by ID.
func (c *Client) DeleteAlertChannel(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/alert-channels/"+url.PathEscape(id), nil)
}

// TestAlertChannel sends a test notification to an alert channel.
func (c *Client) TestAlertChannel(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "POST", "/alert-channels/"+url.PathEscape(id)+"/test", nil)
}
