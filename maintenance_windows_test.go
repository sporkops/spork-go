package spork

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateMaintenanceWindow(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/maintenance-windows" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body MaintenanceWindow
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "Weekly DB Maintenance" {
			t.Errorf("name = %q", body.Name)
		}
		if body.RecurrenceType != MaintenanceRecurrenceWeekly {
			t.Errorf("recurrence_type = %q", body.RecurrenceType)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": MaintenanceWindow{
				ID:             "mw_1",
				Name:           body.Name,
				Timezone:       body.Timezone,
				StartAt:        body.StartAt,
				EndAt:          body.EndAt,
				RecurrenceType: body.RecurrenceType,
				State:          MaintenanceStateScheduled,
			},
		})
	})

	allTrue := true
	mw, err := client.CreateMaintenanceWindow(context.Background(), &MaintenanceWindow{
		Name:           "Weekly DB Maintenance",
		Timezone:       "America/Los_Angeles",
		StartAt:        "2026-05-05T09:00:00Z",
		EndAt:          "2026-05-05T10:00:00Z",
		RecurrenceType: MaintenanceRecurrenceWeekly,
		RecurrenceDays: []int{2},
		AllMonitors:    &allTrue,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mw.ID != "mw_1" {
		t.Errorf("ID = %q", mw.ID)
	}
	if mw.State != MaintenanceStateScheduled {
		t.Errorf("State = %q", mw.State)
	}
}

func TestListMaintenanceWindows(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []MaintenanceWindow{
				{ID: "mw_1", Name: "One"},
				{ID: "mw_2", Name: "Two"},
			},
			"meta": map[string]int{"total": 2, "page": 1, "per_page": 100},
		})
	})

	items, err := client.ListMaintenanceWindows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d", len(items))
	}
}

func TestGetMaintenanceWindow(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/maintenance-windows/mw_1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": MaintenanceWindow{ID: "mw_1", Name: "One", State: MaintenanceStateInProgress},
		})
	})

	mw, err := client.GetMaintenanceWindow(context.Background(), "mw_1")
	if err != nil {
		t.Fatal(err)
	}
	if mw.State != MaintenanceStateInProgress {
		t.Errorf("State = %q", mw.State)
	}
}

func TestUpdateMaintenanceWindow(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": MaintenanceWindow{ID: "mw_1", Name: "Renamed"},
		})
	})

	mw, err := client.UpdateMaintenanceWindow(context.Background(), "mw_1", &MaintenanceWindow{Name: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if mw.Name != "Renamed" {
		t.Errorf("Name = %q", mw.Name)
	}
}

func TestCancelMaintenanceWindow(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/maintenance-windows/mw_1/cancel" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": MaintenanceWindow{ID: "mw_1", State: MaintenanceStateCancelled},
		})
	})

	mw, err := client.CancelMaintenanceWindow(context.Background(), "mw_1")
	if err != nil {
		t.Fatal(err)
	}
	if mw.State != MaintenanceStateCancelled {
		t.Errorf("State = %q", mw.State)
	}
}

func TestDeleteMaintenanceWindow(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteMaintenanceWindow(context.Background(), "mw_1"); err != nil {
		t.Fatal(err)
	}
}
