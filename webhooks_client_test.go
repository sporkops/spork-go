package spork

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestTriggerWebhook_WireShapeAndResponseDecode(t *testing.T) {
	var gotBody TriggerWebhookInput
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/webhooks/trigger" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": TriggerWebhookResult{
				Delivered:       true,
				StatusCode:      200,
				DurationMs:      123,
				Signature:       "t=1712937600,v1=abc",
				PayloadPreview:  `{"event":"monitor.down"}`,
				ResponsePreview: "OK",
			},
		})
	})

	result, err := c.TriggerWebhook(t.Context(), &TriggerWebhookInput{
		AlertChannelID: "ach_test",
		Event:          "monitor.down",
	})
	if err != nil {
		t.Fatalf("TriggerWebhook: %v", err)
	}

	if gotBody.AlertChannelID != "ach_test" || gotBody.Event != "monitor.down" {
		t.Errorf("request body mismatch: %+v", gotBody)
	}
	if !result.Delivered || result.StatusCode != 200 || result.DurationMs != 123 {
		t.Errorf("top-line fields wrong: %+v", result)
	}
	if !strings.HasPrefix(result.Signature, "t=") {
		t.Errorf("signature pass-through failed: %q", result.Signature)
	}
	if result.PayloadPreview == "" || result.ResponsePreview == "" {
		t.Errorf("preview fields dropped: %+v", result)
	}
}

func TestTriggerWebhook_SurfacesServerError(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_abc")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_event","message":"event must be one of: monitor.down, monitor.up"}}`))
	})

	_, err := c.TriggerWebhook(t.Context(), &TriggerWebhookInput{
		AlertChannelID: "ach_test",
		Event:          "not.a.real.event",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 400 || apiErr.Code != "invalid_event" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}
