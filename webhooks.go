package spork

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WebhookSignatureHeader is the HTTP header that carries the Sporkops webhook
// signature. Its value encodes a timestamp and one or more versioned
// signatures, e.g. "t=1712937600,v1=<hex-hmac>".
const WebhookSignatureHeader = "X-Sporkops-Signature"

// Default replay window for webhook verification. Requests whose timestamp is
// older than this relative to the verifier's clock are rejected even if the
// signature is otherwise valid.
const defaultWebhookReplayWindow = 5 * time.Minute

// ErrWebhookReplay is returned by VerifyWebhook when a payload's timestamp
// falls outside the configured replay window. Callers may wish to retry the
// request or investigate clock skew.
var ErrWebhookReplay = errors.New("webhook timestamp outside replay window")

// ErrWebhookSignature is returned by VerifyWebhook when the signature does
// not match any candidate the server sent. This typically means the wrong
// signing secret, a modified payload, or a malformed header.
var ErrWebhookSignature = errors.New("webhook signature mismatch")

// VerifyWebhookOption configures webhook verification.
type VerifyWebhookOption func(*verifyCfg)

type verifyCfg struct {
	replayWindow time.Duration
	now          func() time.Time
}

// WithReplayWindow overrides the default 5-minute replay window. Pass a
// larger value when the verifier's clock is known to be loose, or a smaller
// value when the integration has a strict freshness requirement.
func WithReplayWindow(d time.Duration) VerifyWebhookOption {
	return func(c *verifyCfg) { c.replayWindow = d }
}

// withNow is an internal helper used by tests to pin "now".
func withNow(now func() time.Time) VerifyWebhookOption {
	return func(c *verifyCfg) { c.now = now }
}

// VerifyWebhook checks whether a webhook payload's signature matches the
// shared secret. On success it returns nil; on failure it returns
// ErrWebhookSignature, ErrWebhookReplay, or another concrete error with a
// wrapped cause.
//
// The signature header format is "t=<unix-seconds>,v1=<hex-hmac-sha256>" —
// the exact scheme Stripe popularised. Multiple "v1=<sig>" segments may be
// present (e.g., during a secret rotation); the verification succeeds if
// any of them matches.
//
// Example use in an http.Handler:
//
//	sig := r.Header.Get(spork.WebhookSignatureHeader)
//	body, _ := io.ReadAll(r.Body)
//	if err := spork.VerifyWebhook(body, sig, webhookSecret); err != nil {
//	    http.Error(w, "invalid webhook", http.StatusBadRequest)
//	    return
//	}
func VerifyWebhook(payload []byte, signatureHeader, secret string, opts ...VerifyWebhookOption) error {
	cfg := verifyCfg{
		replayWindow: defaultWebhookReplayWindow,
		now:          time.Now,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if secret == "" {
		return errors.New("webhook secret is empty")
	}
	if signatureHeader == "" {
		return fmt.Errorf("%w: missing %s header", ErrWebhookSignature, WebhookSignatureHeader)
	}

	timestamp, v1sigs, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWebhookSignature, err)
	}

	// Enforce the replay window before doing the (costly) HMAC compare.
	age := cfg.now().Sub(time.Unix(timestamp, 0))
	if age < -cfg.replayWindow || age > cfg.replayWindow {
		return fmt.Errorf("%w: timestamp drift %s", ErrWebhookReplay, age)
	}

	expected := computeWebhookSignature(timestamp, payload, secret)
	for _, candidate := range v1sigs {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return nil
		}
	}
	return ErrWebhookSignature
}

// VerifyWebhookRequest is a convenience wrapper that reads the signature
// header from *http.Request.
func VerifyWebhookRequest(r *http.Request, payload []byte, secret string, opts ...VerifyWebhookOption) error {
	return VerifyWebhook(payload, r.Header.Get(WebhookSignatureHeader), secret, opts...)
}

// SignWebhookPayload is the counterpart to VerifyWebhook: it produces the
// header value a server (or a local CLI trigger) would emit for the given
// payload. Exposed publicly so integrations can build fixtures for tests.
func SignWebhookPayload(payload []byte, timestamp int64, secret string) string {
	return fmt.Sprintf("t=%d,v1=%s", timestamp, computeWebhookSignature(timestamp, payload, secret))
}

func computeWebhookSignature(timestamp int64, payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	// The signed value is "<timestamp>.<raw-body>" — same convention Stripe
	// uses; it prevents an attacker from replaying the body with a different
	// timestamp.
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func parseSignatureHeader(h string) (int64, []string, error) {
	var (
		timestamp int64
		tsSet     bool
		sigs      []string
	)
	for _, part := range strings.Split(h, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return 0, nil, fmt.Errorf("malformed segment %q", part)
		}
		switch kv[0] {
		case "t":
			n, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return 0, nil, fmt.Errorf("malformed timestamp %q", kv[1])
			}
			timestamp = n
			tsSet = true
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	if !tsSet {
		return 0, nil, errors.New("missing t= segment")
	}
	if len(sigs) == 0 {
		return 0, nil, errors.New("no v1= segments")
	}
	return timestamp, sigs, nil
}
