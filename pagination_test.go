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

func TestListOptions_FiltersAppearInQueryString(t *testing.T) {
	// Locks in the contract for server-side filtering: the keys are
	// emitted in sorted order, URL-encoded, and empty-string values are
	// dropped so callers can pass a partial-filter map without emitting
	// no-op parameters.
	opts := ListOptions{
		Page:    2,
		PerPage: 25,
		Filters: map[string]string{
			"type":   "http",
			"status": "down",
			"tag":    "",                 // dropped
			"q":      "error code 500&6", // requires URL-encoding
		},
	}
	got := opts.query()
	// Expected ordering: pagination first, then filters in alphabetical key
	// order (q, status, type — tag is dropped).
	want := "page=2&per_page=25&q=error+code+500%266&status=down&type=http"
	if got != want {
		t.Errorf("query() = %q, want %q", got, want)
	}
}

func TestListOptions_Next_PreservesFilters(t *testing.T) {
	// Locks in the regression that auto-pagination must not silently drop
	// filters on page 2 and beyond — otherwise ListMonitors({Filters: {...}})
	// would return page 1 filtered and every subsequent page unfiltered.
	opts := ListOptions{
		Page:    1,
		PerPage: 100,
		Filters: map[string]string{"status": "down"},
	}
	next := opts.next(PageMeta{Page: 1, PerPage: 100, Total: 250})
	if next.Page != 2 || next.PerPage != 100 {
		t.Errorf("next() lost pagination: %+v", next)
	}
	if next.Filters["status"] != "down" {
		t.Errorf("next() dropped filters: %+v", next.Filters)
	}
}

func TestListMonitorsPage_ForwardsFiltersToServer(t *testing.T) {
	var gotQuery url.Values
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []Monitor{},
			"meta": map[string]int{"total": 0, "page": 1, "per_page": 100},
		})
	})

	_, _, err := c.ListMonitorsPage(t.Context(), ListOptions{
		Filters: map[string]string{"status": "down", "type": "http"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("status") != "down" || gotQuery.Get("type") != "http" {
		t.Errorf("server did not receive filter params: %v", gotQuery)
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
