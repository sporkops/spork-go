package sporkopstest_test

import (
	"errors"
	"net/http"
	"testing"

	spork "github.com/sporkops/spork-go"
	"github.com/sporkops/spork-go/sporkopstest"
)

func TestFakeServer_CreateGetListDeleteMonitor(t *testing.T) {
	fake := sporkopstest.NewFakeServer()
	t.Cleanup(fake.Close)
	client := fake.Client()

	created, err := client.CreateMonitor(t.Context(), &spork.Monitor{
		Name:   "example",
		Target: "https://example.com",
	})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated ID on created monitor")
	}

	got, err := client.GetMonitor(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if got.Name != "example" {
		t.Errorf("got Name=%q, want %q", got.Name, "example")
	}

	all, err := client.ListMonitors(t.Context())
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("got %d monitors, want 1", len(all))
	}

	if err := client.DeleteMonitor(t.Context(), created.ID); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}
	if _, err := client.GetMonitor(t.Context(), created.ID); err == nil {
		t.Error("expected GetMonitor to return not-found after delete")
	}
}

func TestFakeServer_CustomHandlerOverride(t *testing.T) {
	fake := sporkopstest.NewFakeServer()
	t.Cleanup(fake.Close)
	// Inject a deliberate server error so the caller can exercise their
	// error-handling path against the SDK's APIError type.
	fake.Handle("GET", "/monitors/mon_boom", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_custom")
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"code":"boom","message":"custom error"}}`))
	})

	_, err := fake.Client().GetMonitor(t.Context(), "mon_boom")
	if err == nil {
		t.Fatal("expected error from custom handler")
	}
	var apiErr *spork.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *spork.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 || apiErr.Code != "boom" {
		t.Errorf("unexpected APIError contents: %+v", apiErr)
	}
}
