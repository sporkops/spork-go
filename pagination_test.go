package spork

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// TestListMonitors_AutoPaginatesPastFirstPage is the regression test for the
// single nastiest pre-v0.4.0 SDK bug: ListMonitors hard-coded per_page=100
// and only fetched page 1, silently truncating results for organizations
// with more than 100 monitors.
//
// The fake server below serves 253 synthetic monitors across three pages;
// we assert the SDK returns every one of them and makes exactly three
// HTTP calls to do it.
func TestListMonitors_AutoPaginatesPastFirstPage(t *testing.T) {
	const total = 253
	var calls int
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		if page < 1 {
			page = 1
		}
		if perPage < 1 {
			perPage = defaultPerPage
		}

		start := (page - 1) * perPage
		end := start + perPage
		if end > total {
			end = total
		}
		if start > total {
			start = total
		}

		data := make([]Monitor, 0, end-start)
		for i := start; i < end; i++ {
			data = append(data, Monitor{ID: fmt.Sprintf("mon_%d", i), Name: fmt.Sprintf("monitor %d", i)})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": data,
			"meta": map[string]int{"total": total, "page": page, "per_page": perPage},
		})
	})

	monitors, err := c.ListMonitors(t.Context())
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(monitors) != total {
		t.Fatalf("got %d monitors, want %d", len(monitors), total)
	}
	// Spot-check the first, middle, and last records to make sure we didn't
	// just double-count page 1.
	if monitors[0].ID != "mon_0" || monitors[total-1].ID != fmt.Sprintf("mon_%d", total-1) {
		t.Fatalf("boundary mismatch: first=%s last=%s", monitors[0].ID, monitors[total-1].ID)
	}
	if calls != 3 {
		t.Errorf("expected 3 HTTP calls (253 / 100, rounded up), got %d", calls)
	}
}

func TestListMonitorsPage_RespectsExplicitOptions(t *testing.T) {
	var gotQuery url.Values
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []Monitor{},
			"meta": map[string]int{"total": 0, "page": 2, "per_page": 25},
		})
	})

	_, _, err := c.ListMonitorsPage(t.Context(), ListOptions{Page: 2, PerPage: 25})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("page") != "2" || gotQuery.Get("per_page") != "25" {
		t.Errorf("expected page=2 per_page=25, got page=%q per_page=%q",
			gotQuery.Get("page"), gotQuery.Get("per_page"))
	}
}

func TestPageMeta_HasMore(t *testing.T) {
	tests := []struct {
		name     string
		meta     PageMeta
		returned int
		want     bool
	}{
		{"more pages remain", PageMeta{Total: 150, Page: 1, PerPage: 100}, 100, true},
		{"last full page", PageMeta{Total: 100, Page: 1, PerPage: 100}, 100, false},
		{"partial last page", PageMeta{Total: 150, Page: 2, PerPage: 100}, 50, false},
		{"zero total falls back to short-page heuristic", PageMeta{PerPage: 100}, 100, true},
		{"zero total with short page", PageMeta{PerPage: 100}, 42, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.meta.HasMore(tc.returned); got != tc.want {
				t.Errorf("HasMore() = %v, want %v", got, tc.want)
			}
		})
	}
}
