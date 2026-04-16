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

// ListAlertChannels returns every alert channel for the authenticated
// organization, transparently paginating through all pages.
func (c *Client) ListAlertChannels(ctx context.Context) ([]AlertChannel, error) {
	return collectAll[AlertChannel](func(opts ListOptions) ([]AlertChannel, PageMeta, error) {
		return c.ListAlertChannelsWithOptions(ctx, opts)
	})
}

// ListAlertChannelsWithOptions returns a single page of alert channels along with
// pagination metadata. Use ListAlertChannels if you want every record.
func (c *Client) ListAlertChannelsWithOptions(ctx context.Context, opts ListOptions) ([]AlertChannel, PageMeta, error) {
	var result []AlertChannel
	path := "/alert-channels?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
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

// ResendAlertChannelVerification resends the verification email for an
// unverified email alert channel. Rate limited to 5 per user per 10 minutes.
func (c *Client) ResendAlertChannelVerification(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "POST", "/alert-channels/"+url.PathEscape(id)+"/resend-verification", nil)
}

// ListDeliveryLogs returns alert delivery log entries, transparently
// paginating through all pages. Pass an empty channelID to list logs
// across all channels, or a specific channel ID to filter.
func (c *Client) ListDeliveryLogs(ctx context.Context, channelID string) ([]DeliveryLog, error) {
	return collectAll[DeliveryLog](func(opts ListOptions) ([]DeliveryLog, PageMeta, error) {
		return c.ListDeliveryLogsWithOptions(ctx, channelID, opts)
	})
}

// ListDeliveryLogsWithOptions returns a single page of delivery logs along
// with pagination metadata.
func (c *Client) ListDeliveryLogsWithOptions(ctx context.Context, channelID string, opts ListOptions) ([]DeliveryLog, PageMeta, error) {
	var result []DeliveryLog
	path := "/delivery-logs?" + opts.query()
	if channelID != "" {
		path += "&channel_id=" + url.QueryEscape(channelID)
	}
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}
