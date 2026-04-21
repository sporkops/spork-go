package spork

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// TestListMonitors_AutoPaginatesViaCursor asserts that the auto-paginator
// walks every page via the server's next_cursor without short-circuiting
// on the first page.
func TestListMonitors_AutoPaginatesViaCursor(t *testing.T) {
	const total = 253
	const perPage = 100
	var calls int
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit < 1 {
			limit = defaultLimit
		}

		// Decode the caller-supplied cursor as a simple integer offset
		// — the SDK treats it as opaque, so any scheme works server-side.
		start := 0
		if cur := q.Get("cursor"); cur != "" {
			start, _ = strconv.Atoi(cur)
		}
		end := start + limit
		if end > total {
			end = total
		}

		data := make([]Monitor, 0, end-start)
		for i := start; i < end; i++ {
			data = append(data, Monitor{ID: fmt.Sprintf("mon_%d", i), Name: fmt.Sprintf("monitor %d", i)})
		}

		nextCursor := ""
		if end < total {
			nextCursor = strconv.Itoa(end)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": data,
			"meta": map[string]any{
				"has_more":    nextCursor != "",
				"next_cursor": nextCursor,
			},
		})
	})

	monitors, err := c.ListMonitors(t.Context())
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(monitors) != total {
		t.Fatalf("got %d monitors, want %d", len(monitors), total)
	}
	if monitors[0].ID != "mon_0" || monitors[total-1].ID != fmt.Sprintf("mon_%d", total-1) {
		t.Fatalf("boundary mismatch: first=%s last=%s", monitors[0].ID, monitors[total-1].ID)
	}
	// 253/100 rounds up to 3 pages.
	if calls != 3 {
		t.Errorf("expected 3 HTTP calls, got %d", calls)
	}
}

func TestListOptions_FiltersAppearInQueryString(t *testing.T) {
	// Locks in the contract for server-side filtering: the keys are
	// emitted in sorted order, URL-encoded, and empty-string values are
	// dropped so callers can pass a partial-filter map without emitting
	// no-op parameters.
	opts := ListOptions{
		Cursor: "cur_abc",
		Limit:  25,
		Filters: map[string]string{
			"type":   "http",
			"status": "down",
			"tag":    "",                 // dropped
			"q":      "error code 500&6", // requires URL-encoding
		},
	}
	got := opts.query()
	want := "cursor=cur_abc&limit=25&q=error+code+500%266&status=down&type=http"
	if got != want {
		t.Errorf("query() = %q, want %q", got, want)
	}
}

func TestListOptions_EmitsLimitOnFirstPage(t *testing.T) {
	// No cursor, no filters → limit-only query string.
	opts := ListOptions{Limit: 25}
	got := opts.query()
	if got != "limit=25" {
		t.Errorf("query() = %q, want limit=25", got)
	}
}

func TestListOptions_Next_PreservesFiltersAndLimit(t *testing.T) {
	// Auto-pagination must not silently drop filters on page 2 and
	// beyond, and must carry the caller's Limit through.
	opts := ListOptions{
		Limit:   100,
		Filters: map[string]string{"status": "down"},
	}
	next := opts.next(PageInfo{HasMore: true, NextCursor: "cur_xyz"})
	if next.Cursor != "cur_xyz" {
		t.Errorf("next() should carry cursor, got %q", next.Cursor)
	}
	if next.Limit != 100 {
		t.Errorf("next() should preserve Limit, got %d", next.Limit)
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
			"meta": map[string]any{"has_more": false},
		})
	})

	_, _, err := c.ListMonitorsWithOptions(t.Context(), ListOptions{
		Filters: map[string]string{"status": "down", "type": "http"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("status") != "down" || gotQuery.Get("type") != "http" {
		t.Errorf("server did not receive filter params: %v", gotQuery)
	}
}

func TestListMonitors_FollowsNextCursorWhenProvided(t *testing.T) {
	// The auto-paginator passes the server's next_cursor back verbatim.
	calls := 0
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		switch calls {
		case 1:
			if q.Get("cursor") != "" {
				t.Errorf("first call should not carry a cursor, got %q", q.Get("cursor"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []Monitor{{ID: "mon_1"}},
				"meta": map[string]any{
					"has_more":    true,
					"next_cursor": "cur_page_2",
				},
			})
		case 2:
			if q.Get("cursor") != "cur_page_2" {
				t.Errorf("second call must carry the server's next_cursor, got %q", q.Get("cursor"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []Monitor{{ID: "mon_2"}},
				"meta": map[string]any{
					"has_more":    false,
					"next_cursor": "",
				},
			})
		default:
			t.Fatalf("unexpected extra call %d", calls)
		}
	})

	monitors, err := c.ListMonitors(t.Context())
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(monitors) != 2 || monitors[0].ID != "mon_1" || monitors[1].ID != "mon_2" {
		t.Fatalf("expected [mon_1, mon_2], got %+v", monitors)
	}
	if calls != 2 {
		t.Errorf("expected exactly 2 HTTP calls, got %d", calls)
	}
}

func TestPageInfo_HasMore(t *testing.T) {
	if !(PageInfo{HasMore: true, NextCursor: "cur_x"}).HasMore {
		t.Error("expected HasMore=true to survive round-trip")
	}
	if (PageInfo{HasMore: false}).HasMore {
		t.Error("expected HasMore=false to survive round-trip")
	}
}
