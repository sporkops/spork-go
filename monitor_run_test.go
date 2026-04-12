package spork

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestRunMonitor_WireShapeAndResponseDecode(t *testing.T) {
	var gotBody RunMonitorInput
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/monitors/run" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": RunMonitorResult{
				Target:         "https://example.com",
				Type:           "http",
				Region:         "us-central1",
				Status:         "up",
				HTTPCode:       200,
				ResponseTimeMs: 42,
				CheckedAt:      time.Unix(1_712_937_600, 0).UTC(),
			},
		})
	})

	result, err := c.RunMonitor(t.Context(), &RunMonitorInput{
		Target:         "https://example.com",
		Type:           "http",
		ExpectedStatus: 200,
	})
	if err != nil {
		t.Fatalf("RunMonitor: %v", err)
	}
	if gotBody.Target != "https://example.com" || gotBody.ExpectedStatus != 200 {
		t.Errorf("wire body mismatch: %+v", gotBody)
	}
	if result.Status != "up" || result.HTTPCode != 200 || result.ResponseTimeMs != 42 {
		t.Errorf("result fields wrong: %+v", result)
	}
	if result.CheckedAt.IsZero() {
		t.Errorf("CheckedAt should round-trip as a non-zero time, got %v", result.CheckedAt)
	}
}
