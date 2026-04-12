package spork

import "context"

// TriggerWebhookInput is the request body for TriggerWebhook.
type TriggerWebhookInput struct {
	// AlertChannelID identifies the webhook alert channel to fire into.
	// The channel must be of type "webhook"; the server rejects other types.
	AlertChannelID string `json:"alert_channel_id"`
	// Event is the synthetic event to deliver. Must be "monitor.down" or
	// "monitor.up"; the server rejects unknown values rather than signing
	// a payload with a bogus event name.
	Event string `json:"event"`
}

// TriggerWebhookResult reports the outcome of a single synthetic delivery.
// Fields mirror the server's TriggerWebhookResult schema.
type TriggerWebhookResult struct {
	// Delivered is true iff the receiver returned a 2xx response.
	Delivered bool `json:"delivered"`
	// StatusCode is the downstream HTTP status code, or 0 if no response.
	StatusCode int `json:"status_code,omitempty"`
	// DurationMs is the round-trip latency in milliseconds.
	DurationMs int64 `json:"duration_ms"`
	// Error holds a transport-level error message when Delivered is false
	// (TCP reset, DNS failure, TLS handshake error, etc). Empty when the
	// receiver responded — even if non-2xx. Check StatusCode to
	// distinguish.
	Error string `json:"error,omitempty"`
	// Signature is the exact X-Sporkops-Signature header value the server
	// sent with the delivery. Useful for verifying an integration's
	// verifier picks up the same bytes.
	Signature string `json:"signature,omitempty"`
	// PayloadPreview is a truncated copy (<=4 KiB) of the synthetic
	// WebhookPayload that was signed and sent.
	PayloadPreview string `json:"payload_preview,omitempty"`
	// ResponsePreview is a truncated copy (<=2 KiB) of the receiver's
	// response body. Empty when DurationMs = transport-level failure.
	ResponsePreview string `json:"response_preview,omitempty"`
}

// TriggerWebhook fires a synthetic, signed webhook event at the specified
// webhook alert channel. The server returns delivery details — status
// code, latency, signature header, payload preview — for integration
// testing.
//
// Unlike a real alert delivery, this does NOT retry on failure. The 5xx
// status code (or transport error) is returned verbatim so the caller
// can see it immediately.
//
// Rate-limited by the server to 1 trigger per channel per 60 seconds;
// exceeding that returns IsRateLimited(err) == true.
//
// Example:
//
//	result, err := client.TriggerWebhook(ctx, &spork.TriggerWebhookInput{
//	    AlertChannelID: "ach_abc",
//	    Event:          "monitor.down",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("delivered=%t status=%d dur=%dms\n",
//	    result.Delivered, result.StatusCode, result.DurationMs)
func (c *Client) TriggerWebhook(ctx context.Context, input *TriggerWebhookInput) (*TriggerWebhookResult, error) {
	var result TriggerWebhookResult
	if err := c.doSingle(ctx, "POST", "/webhooks/trigger", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
