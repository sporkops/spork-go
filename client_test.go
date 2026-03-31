package spork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(
		WithAPIKey("sk_test_key"),
		WithBaseURL(srv.URL),
	)
	return c, srv
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(WithAPIKey("sk_test"))
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.token != "sk_test" {
		t.Errorf("token = %q, want %q", c.token, "sk_test")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient should not be nil")
	}
}

func TestNewClient_Options(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c := NewClient(
		WithAPIKey("key123"),
		WithBaseURL("https://custom.api.com/v1"),
		WithHTTPClient(hc),
		WithUserAgent("my-app/1.0"),
	)
	if c.baseURL != "https://custom.api.com/v1" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.token != "key123" {
		t.Errorf("token = %q", c.token)
	}
	if c.httpClient != hc {
		t.Error("httpClient should be custom client")
	}
	if c.userAgent != "my-app/1.0" {
		t.Errorf("userAgent = %q", c.userAgent)
	}
}

func TestCreateMonitor(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/monitors" {
			t.Errorf("path = %s, want /monitors", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk_test_key" {
			t.Errorf("auth = %q", auth)
		}

		var m Monitor
		json.NewDecoder(r.Body).Decode(&m)
		if m.Name != "Test Monitor" {
			t.Errorf("name = %q", m.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": Monitor{
				ID:     "mon_123",
				Name:   m.Name,
				Target: m.Target,
				Status: "active",
			},
		})
	})

	m, err := client.CreateMonitor(context.Background(), &Monitor{
		Name:   "Test Monitor",
		Target: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "mon_123" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Status != "active" {
		t.Errorf("Status = %q", m.Status)
	}
}

func TestListMonitors(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []Monitor{
				{ID: "mon_1", Name: "Mon 1"},
				{ID: "mon_2", Name: "Mon 2"},
			},
			"meta": map[string]int{"total": 2, "page": 1, "per_page": 100},
		})
	})

	monitors, err := client.ListMonitors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 2 {
		t.Fatalf("got %d monitors, want 2", len(monitors))
	}
	if monitors[0].ID != "mon_1" {
		t.Errorf("monitors[0].ID = %q", monitors[0].ID)
	}
}

func TestGetMonitor(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/monitors/mon_123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": Monitor{ID: "mon_123", Name: "My Monitor"},
		})
	})

	m, err := client.GetMonitor(context.Background(), "mon_123")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "My Monitor" {
		t.Errorf("Name = %q", m.Name)
	}
}

func TestDeleteMonitor(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteMonitor(context.Background(), "mon_123")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIError_NotFound(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "not_found", "message": "not found"},
		})
	})

	_, err := client.GetMonitor(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestAPIError_Unauthorized(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_abc123")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "unauthorized", "message": "authentication required"},
		})
	})

	_, err := client.GetAccount(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized, got: %v", err)
	}
	if apiErr, ok := asAPIError(err); ok {
		if apiErr.RequestID != "req_abc123" {
			t.Errorf("RequestID = %q, want req_abc123", apiErr.RequestID)
		}
	}
}

func TestAPIError_PaymentRequired(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "payment_required", "message": "subscription required"},
		})
	})

	_, err := client.ListMonitors(context.Background())
	if !IsPaymentRequired(err) {
		t.Errorf("expected IsPaymentRequired, got: %v", err)
	}
}

func TestRetryOnTransientError(t *testing.T) {
	var attempts int32
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": Monitor{ID: "mon_1"},
		})
	})

	m, err := client.GetMonitor(context.Background(), "mon_1")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "mon_1" {
		t.Errorf("ID = %q", m.ID)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
	}
}

func TestRetryExhausted(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.GetMonitor(context.Background(), "mon_1")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

func TestContextCancellation(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetMonitor(ctx, "mon_1")
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestCreateAlertChannel(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/alert-channels" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": AlertChannel{ID: "ac_1", Name: "Email", Type: "email"},
		})
	})

	ch, err := client.CreateAlertChannel(context.Background(), &AlertChannel{
		Name:   "Email",
		Type:   "email",
		Config: map[string]string{"to": "test@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "ac_1" {
		t.Errorf("ID = %q", ch.ID)
	}
}

func TestCreateStatusPage(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/status-pages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": StatusPage{ID: "sp_1", Name: "My Status", Slug: "my-status"},
		})
	})

	sp, err := client.CreateStatusPage(context.Background(), &StatusPage{
		Name: "My Status",
		Slug: "my-status",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sp.ID != "sp_1" {
		t.Errorf("ID = %q", sp.ID)
	}
}

func TestCreateIncident(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status-pages/sp_1/incidents" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": Incident{ID: "inc_1", Title: "Outage"},
		})
	})

	inc, err := client.CreateIncident(context.Background(), "sp_1", &Incident{
		Title: "Outage",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inc.ID != "inc_1" {
		t.Errorf("ID = %q", inc.ID)
	}
}

func TestCreateAPIKey(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api-keys" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": APIKey{ID: "ak_1", Name: "test", Key: "sk_live_xxx"},
		})
	})

	key, err := client.CreateAPIKey(context.Background(), &CreateAPIKeyInput{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if key.Key != "sk_live_xxx" {
		t.Errorf("Key = %q", key.Key)
	}
}

func TestUserAgent(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != "spork-go-sdk/"+Version {
			t.Errorf("User-Agent = %q", ua)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client.DeleteMonitor(context.Background(), "mon_1")
}

func TestCustomUserAgent(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua != "my-cli/2.0" {
			t.Errorf("User-Agent = %q", ua)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client.userAgent = "my-cli/2.0"

	client.DeleteMonitor(context.Background(), "mon_1")
}

func asAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
