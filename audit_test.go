package spork

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Tests for the 19 API methods added to align spork-go with the OpenAPI spec.
// Each test asserts method/path, and verifies the response is parsed correctly
// (including the non-standard envelopes used by a few endpoints).

// --- Monitor supplementary endpoints ---

func TestGetMonitorResult(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/monitors/mon_1/results/res_7" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": MonitorResult{
				ID:             "res_7",
				MonitorID:      "mon_1",
				Status:         "up",
				StatusCode:     200,
				ResponseTimeMs: 142,
				Region:         "us-central1",
				CheckedAt:      "2026-04-16T05:00:00Z",
			},
		})
	})

	result, err := client.GetMonitorResult(context.Background(), "mon_1", "res_7")
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "res_7" {
		t.Errorf("ID = %q", result.ID)
	}
	if result.Region != "us-central1" {
		t.Errorf("Region = %q", result.Region)
	}
}

func TestGetMonitorStats(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/monitors/mon_1/stats" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": MonitorStats{
				UptimePercentage:  99.95,
				AvgResponseTimeMs: 142,
			},
		})
	})

	stats, err := client.GetMonitorStats(context.Background(), "mon_1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.UptimePercentage != 99.95 {
		t.Errorf("UptimePercentage = %v", stats.UptimePercentage)
	}
	if stats.AvgResponseTimeMs != 142 {
		t.Errorf("AvgResponseTimeMs = %v", stats.AvgResponseTimeMs)
	}
}

func TestListMonitorAuditTrail(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/monitors/mon_1/audit-trail" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Errorf("limit = %q, want 25", got)
		}
		if got := r.URL.Query().Get("cursor"); got != "c1" {
			t.Errorf("cursor = %q, want c1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		// Non-standard envelope: next_cursor at the top level, NOT under meta.
		json.NewEncoder(w).Encode(map[string]any{
			"data": []AuditTrailEntry{
				{ID: "au_1", Timestamp: "2026-04-16T05:00:00Z", ActorEmail: "u@x.com", Source: "cli", Action: "updated"},
			},
			"next_cursor": "c2",
		})
	})

	entries, next, err := client.ListMonitorAuditTrail(context.Background(), "mon_1", 25, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Source != "cli" {
		t.Errorf("Source = %q", entries[0].Source)
	}
	if next != "c2" {
		t.Errorf("next_cursor = %q, want c2", next)
	}
}

func TestListMonitorAuditTrail_OmitsParamsWhenUnset(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query string, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []AuditTrailEntry{}, "next_cursor": ""})
	})

	entries, next, err := client.ListMonitorAuditTrail(context.Background(), "mon_1", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
	if next != "" {
		t.Errorf("next_cursor = %q, want empty", next)
	}
}

// --- Alert channel supplementary endpoints ---

func TestResendAlertChannelVerification(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/alert-channels/ach_1/resend-verification" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.ResendAlertChannelVerification(context.Background(), "ach_1"); err != nil {
		t.Fatal(err)
	}
}

func TestListDeliveryLogs_WithChannelFilter(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/delivery-logs" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("channel_id"); got != "ach_1" {
			t.Errorf("channel_id = %q, want ach_1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []DeliveryLog{
				{ID: "dl_1", ChannelID: "ach_1", ChannelType: "email", Event: "monitor.down", Status: "success"},
			},
			"meta": map[string]int{"total": 1, "page": 1, "per_page": 100},
		})
	})

	logs, err := client.ListDeliveryLogs(context.Background(), "ach_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs = %d, want 1", len(logs))
	}
	if logs[0].ChannelID != "ach_1" {
		t.Errorf("ChannelID = %q", logs[0].ChannelID)
	}
}

func TestListDeliveryLogs_NoFilter(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("channel_id"); got != "" {
			t.Errorf("channel_id should be empty when not filtered, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []DeliveryLog{},
			"meta": map[string]int{"total": 0, "page": 1, "per_page": 100},
		})
	})

	if _, err := client.ListDeliveryLogs(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

// --- Status page supplementary endpoints ---

func TestGetCustomDomainStatus(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/status-pages/sp_1/custom-domain" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": CustomDomainStatus{
				Domain:      "status.example.com",
				Status:      "pending",
				SSLStatus:   "provisioning",
				DNSVerified: false,
				CNAMETarget: "status.sporkops.com",
			},
		})
	})

	status, err := client.GetCustomDomainStatus(context.Background(), "sp_1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Domain != "status.example.com" {
		t.Errorf("Domain = %q", status.Domain)
	}
	if status.DNSVerified {
		t.Errorf("DNSVerified = true, want false")
	}
}

func TestCreateComponent(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/status-pages/sp_1/components" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var comp StatusComponent
		json.NewDecoder(r.Body).Decode(&comp)
		if comp.MonitorID != "mon_1" {
			t.Errorf("MonitorID = %q", comp.MonitorID)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": StatusComponent{ID: "cmp_1", MonitorID: "mon_1", DisplayName: "API"},
		})
	})

	c, err := client.CreateComponent(context.Background(), "sp_1", &StatusComponent{
		MonitorID: "mon_1", DisplayName: "API",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "cmp_1" {
		t.Errorf("ID = %q", c.ID)
	}
}

// TestUpdateComponent verifies that UpdateComponentInput serialises zero values
// and empty strings (no omitempty), which is required for the PUT semantics —
// users need to be able to clear description, ungroup (group_id=""), and set
// order=0.
func TestUpdateComponent_PreservesZeroValues(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/status-pages/sp_1/components/cmp_1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)
		// Every field must be present in the JSON, even when zero.
		for _, key := range []string{`"display_name"`, `"description"`, `"group_id"`, `"order"`} {
			if !strings.Contains(body, key) {
				t.Errorf("request body missing %s field\nbody: %s", key, body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": StatusComponent{ID: "cmp_1", DisplayName: "New", Order: 0},
		})
	})

	// Pass zero/empty values to ensure they round-trip to the server.
	_, err := client.UpdateComponent(context.Background(), "sp_1", "cmp_1", &UpdateComponentInput{
		DisplayName: "New",
		Description: "",
		GroupID:     "",
		Order:       0,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteComponent(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/status-pages/sp_1/components/cmp_1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteComponent(context.Background(), "sp_1", "cmp_1"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateComponentGroup(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/status-pages/sp_1/component-groups" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": ComponentGroup{ID: "grp_1", Name: "Public APIs"},
		})
	})

	g, err := client.CreateComponentGroup(context.Background(), "sp_1", &ComponentGroup{Name: "Public APIs"})
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != "grp_1" {
		t.Errorf("ID = %q", g.ID)
	}
}

func TestUpdateComponentGroup(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/status-pages/sp_1/component-groups/grp_1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": ComponentGroup{ID: "grp_1", Name: "Renamed"},
		})
	})

	g, err := client.UpdateComponentGroup(context.Background(), "sp_1", "grp_1", &ComponentGroup{Name: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Renamed" {
		t.Errorf("Name = %q", g.Name)
	}
}

func TestDeleteComponentGroup(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/status-pages/sp_1/component-groups/grp_1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteComponentGroup(context.Background(), "sp_1", "grp_1"); err != nil {
		t.Fatal(err)
	}
}

func TestListSubscribers(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status-pages/sp_1/subscribers" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []EmailSubscriber{
				{ID: "sub_1", Email: "a@x.com", Confirmed: true},
			},
			"meta": map[string]int{"total": 1, "page": 1, "per_page": 100},
		})
	})

	subs, err := client.ListSubscribers(context.Background(), "sp_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subs = %d, want 1", len(subs))
	}
}

func TestGetSubscriberCount(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status-pages/sp_1/subscribers/count" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": SubscriberCount{Count: 42},
		})
	})

	count, err := client.GetSubscriberCount(context.Background(), "sp_1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}
}

func TestDeleteSubscriber(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/status-pages/sp_1/subscribers/sub_1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteSubscriber(context.Background(), "sp_1", "sub_1"); err != nil {
		t.Fatal(err)
	}
}

// --- Member supplementary endpoints ---

func TestListPendingInvites(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/members/invites" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []Member{{ID: "mbr_1", Email: "u@x.com", Status: "pending"}},
		})
	})

	invites, err := client.ListPendingInvites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(invites) != 1 {
		t.Fatalf("invites = %d, want 1", len(invites))
	}
	if invites[0].Status != "pending" {
		t.Errorf("Status = %q", invites[0].Status)
	}
}

func TestAcceptInvite(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/members/accept" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var input AcceptInviteInput
		json.NewDecoder(r.Body).Decode(&input)
		if input.Token != "tok_abc" {
			t.Errorf("Token = %q", input.Token)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": AcceptInviteResult{Status: "accepted"},
		})
	})

	result, err := client.AcceptInvite(context.Background(), &AcceptInviteInput{Token: "tok_abc"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "accepted" {
		t.Errorf("Status = %q", result.Status)
	}
}

// --- Organization supplementary endpoints ---

func TestListRegions(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/regions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []Region{
				{ID: "us-central1", Name: "US Central (Iowa)"},
				{ID: "europe-west1", Name: "Europe West (Belgium)"},
			},
		})
	})

	regions, err := client.ListRegions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 2 {
		t.Fatalf("regions = %d, want 2", len(regions))
	}
	if regions[0].ID != "us-central1" {
		t.Errorf("ID[0] = %q", regions[0].ID)
	}
}

func TestExportOrganizationData(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organization/export" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// Note: this endpoint returns the raw export, NOT the standard {data:...} envelope.
		w.Write([]byte(`{"monitors":[{"id":"mon_1"}],"alert_channels":[]}`))
	})

	data, err := client.ExportOrganizationData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Verify it's valid JSON and has the expected shape.
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("export not valid JSON: %v", err)
	}
	if _, ok := parsed["monitors"]; !ok {
		t.Errorf("export missing monitors key: %s", data)
	}
}

func TestExportOrganizationData_ForwardsAPIError(t *testing.T) {
	// Verify the error is surfaced as *APIError (not wrapped) so callers can
	// do errors.As on it. A wrap would break that expectation.
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "insufficient_role", "message": "owner required"},
		})
	})

	_, err := client.ExportOrganizationData(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsForbidden(err) {
		t.Errorf("expected IsForbidden, got: %v", err)
	}
}

