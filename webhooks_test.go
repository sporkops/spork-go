package spork

import (
	"errors"
	"testing"
	"time"
)

// TestSign_KnownVector is the **cross-repo contract test** for the v1
// webhook signing scheme.
//
// The exact HMAC-SHA256 output below is duplicated in the bemi backend
// at spork/ping/backend/internal/webhook/signer_test.go under
// TestSign_EmitsStripeStyleHeader_KnownVector. If either implementation
// drifts — a formatter change, a byte-order mistake, a different
// separator — this test and its bemi twin diverge. Both repos checking
// in the same hex value means either test failing catches the drift
// immediately, which is what makes VerifyWebhook's guarantee actually
// enforceable.
//
// Input pinned: secret="whsec_test", body={"event":"monitor.down","id":"mon_123"},
// timestamp=1712937600 (2024-04-12 15:20:00 UTC).
// Expected HMAC-SHA256 output over "1712937600.<body>".
func TestSign_KnownVector(t *testing.T) {
	const (
		secret        = "whsec_test"
		body          = `{"event":"monitor.down","id":"mon_123"}`
		timestamp     = int64(1_712_937_600)
		expectedHex   = "303603f6e23fa01d80e9a8f20f963ac60d6b9ed90a2fdfc67cbb27e52d7b23b1"
		expectedValue = "t=1712937600,v1=" + expectedHex
	)

	got := SignWebhookPayload([]byte(body), timestamp, secret)
	if got != expectedValue {
		t.Errorf("contract vector mismatch.\n  got:  %s\n  want: %s\n"+
			"\nIf this test fails, the webhook signing scheme changed — "+
			"update bemi's TestSign_EmitsStripeStyleHeader_KnownVector to match "+
			"and bump the v1 scheme identifier on both sides.", got, expectedValue)
	}

	// Round-trip: the SDK's VerifyWebhook must accept what our own
	// Sign produced. If the parse path drifts from the emit path
	// (different separators, case, etc.) this catches it locally even
	// before the bemi contract check runs.
	if err := VerifyWebhook([]byte(body), got, secret,
		withNow(func() time.Time { return time.Unix(timestamp, 0) })); err != nil {
		t.Errorf("SDK verifier rejected its own signature: %v", err)
	}
}

func TestVerifyWebhook_RoundTrip(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"event":"monitor.down","id":"mon_123"}`)
	ts := time.Now().Unix()

	header := SignWebhookPayload(payload, ts, secret)
	if err := VerifyWebhook(payload, header, secret); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifyWebhook_TamperedPayloadRejected(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"event":"monitor.down","id":"mon_123"}`)
	ts := time.Now().Unix()
	header := SignWebhookPayload(payload, ts, secret)

	tampered := append([]byte{}, payload...)
	tampered[10] = 'X'
	err := VerifyWebhook(tampered, header, secret)
	if !errors.Is(err, ErrWebhookSignature) {
		t.Fatalf("expected ErrWebhookSignature, got %v", err)
	}
}

func TestVerifyWebhook_WrongSecretRejected(t *testing.T) {
	payload := []byte(`{"event":"monitor.down"}`)
	ts := time.Now().Unix()
	header := SignWebhookPayload(payload, ts, "whsec_real")
	err := VerifyWebhook(payload, header, "whsec_wrong")
	if !errors.Is(err, ErrWebhookSignature) {
		t.Fatalf("expected ErrWebhookSignature, got %v", err)
	}
}

func TestVerifyWebhook_ReplayWindowEnforced(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{}`)
	old := time.Now().Add(-time.Hour).Unix()
	header := SignWebhookPayload(payload, old, secret)

	err := VerifyWebhook(payload, header, secret)
	if !errors.Is(err, ErrWebhookReplay) {
		t.Fatalf("expected ErrWebhookReplay, got %v", err)
	}
}

func TestVerifyWebhook_WidenedReplayWindow(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{}`)
	old := time.Now().Add(-2 * time.Hour).Unix()
	header := SignWebhookPayload(payload, old, secret)

	if err := VerifyWebhook(payload, header, secret, WithReplayWindow(3*time.Hour)); err != nil {
		t.Fatalf("expected valid with widened window, got %v", err)
	}
}

func TestVerifyWebhook_MissingHeader(t *testing.T) {
	err := VerifyWebhook([]byte(`{}`), "", "whsec")
	if !errors.Is(err, ErrWebhookSignature) {
		t.Fatalf("expected ErrWebhookSignature for empty header, got %v", err)
	}
}

func TestVerifyWebhook_MultipleCandidateSignaturesOneMatches(t *testing.T) {
	// Simulates a key rotation where the server emits two v1= segments.
	payload := []byte(`{}`)
	ts := time.Now().Unix()
	realSig := SignWebhookPayload(payload, ts, "whsec_new")
	// Append a decoy that doesn't match. Format: t=<ts>,v1=<decoy>,v1=<real>
	header := realSig + ",v1=00000000"

	if err := VerifyWebhook(payload, header, "whsec_new"); err != nil {
		t.Fatalf("expected valid with rotation, got %v", err)
	}
}
