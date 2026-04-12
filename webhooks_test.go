package spork

import (
	"errors"
	"testing"
	"time"
)

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
