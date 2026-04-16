// Package sporkopstest provides test doubles for code that calls the Spork
// Go SDK. Rather than spinning up an httptest.Server for every test, use
// NewFakeServer to get a minimal in-memory backend that exposes the subset
// of the API surface most real tests exercise: create / read / list /
// update / delete on the resources we ship SDK methods for.
//
// The fake is intentionally NOT a faithful simulation of the production
// API — it does just enough to drive happy-path SDK tests. Callers who
// need adversarial scenarios (rate limiting, validation errors, flaky
// networks) should use httptest.Server directly or add a custom handler
// via FakeServer.Handle.
//
// Example:
//
//	fake := sporkopstest.NewFakeServer()
//	t.Cleanup(fake.Close)
//
//	client := spork.NewClient(
//	    spork.WithAPIKey("sk_test"),
//	    spork.WithBaseURL(fake.BaseURL()),
//	)
//
//	created, err := client.CreateMonitor(ctx, &spork.Monitor{
//	    Name: "test", Target: "https://example.com",
//	})
package sporkopstest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	spork "github.com/sporkops/spork-go"
)

// FakeServer is an in-memory fake of the Spork API for use in tests.
type FakeServer struct {
	server *httptest.Server

	mu       sync.Mutex
	monitors map[string]spork.Monitor
	alerts   map[string]spork.AlertChannel
	pages    map[string]spork.StatusPage
	counter  atomic.Uint64

	// customHandlers lets callers override specific paths for a single test.
	customHandlers map[string]http.HandlerFunc
}

// NewFakeServer starts a fake HTTP server backed by in-memory state and
// returns a handle that tests can use to configure it and tear it down.
// The caller MUST call Close (typically via t.Cleanup).
func NewFakeServer() *FakeServer {
	fs := &FakeServer{
		monitors:       make(map[string]spork.Monitor),
		alerts:         make(map[string]spork.AlertChannel),
		pages:          make(map[string]spork.StatusPage),
		customHandlers: make(map[string]http.HandlerFunc),
	}
	fs.server = httptest.NewServer(http.HandlerFunc(fs.route))
	return fs
}

// Close shuts down the underlying HTTP server. Safe to call more than once.
func (f *FakeServer) Close() {
	if f.server != nil {
		f.server.Close()
	}
}

// BaseURL returns the URL to pass to spork.WithBaseURL.
func (f *FakeServer) BaseURL() string {
	return f.server.URL
}

// Client returns a *spork.Client pre-configured to talk to the fake. The
// returned client uses a placeholder API key; tests that care about auth
// should construct their own client.
func (f *FakeServer) Client(opts ...spork.Option) *spork.Client {
	base := []spork.Option{
		spork.WithAPIKey("sk_test_fake"),
		spork.WithBaseURL(f.BaseURL()),
	}
	return spork.NewClient(append(base, opts...)...)
}

// Handle overrides the default behavior for a specific method+path pair.
// Use this to inject errors, assert request shape, or simulate latency in
// a narrow test. The handler is reset to the default on every call to
// Reset.
func (f *FakeServer) Handle(method, path string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.customHandlers[method+" "+path] = h
}

// Reset clears all state (monitors, alert channels, status pages) and
// removes any custom handlers. Useful between subtests.
func (f *FakeServer) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.monitors = make(map[string]spork.Monitor)
	f.alerts = make(map[string]spork.AlertChannel)
	f.pages = make(map[string]spork.StatusPage)
	f.customHandlers = make(map[string]http.HandlerFunc)
}

// Monitors returns a snapshot of the monitors currently stored in the
// fake, keyed by ID. The returned map is a copy and is safe to iterate.
func (f *FakeServer) Monitors() map[string]spork.Monitor {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]spork.Monitor, len(f.monitors))
	for k, v := range f.monitors {
		out[k] = v
	}
	return out
}

func (f *FakeServer) nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, f.counter.Add(1))
}

func (f *FakeServer) route(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	if h, ok := f.customHandlers[r.Method+" "+r.URL.Path]; ok {
		f.mu.Unlock()
		h(w, r)
		return
	}
	f.mu.Unlock()

	// Generic request-id header so SDK error paths can surface it.
	w.Header().Set("X-Request-Id", fmt.Sprintf("req_fake_%d", time.Now().UnixNano()))
	w.Header().Set("Content-Type", "application/json")

	switch {
	case strings.HasPrefix(r.URL.Path, "/monitors"):
		f.handleMonitors(w, r)
	case strings.HasPrefix(r.URL.Path, "/alert-channels"):
		f.handleAlertChannels(w, r)
	case strings.HasPrefix(r.URL.Path, "/status-pages"):
		f.handleStatusPages(w, r)
	case r.URL.Path == "/regions" && r.Method == http.MethodGet:
		f.handleRegions(w, r)
	case strings.HasPrefix(r.URL.Path, "/delivery-logs"):
		f.handleDeliveryLogs(w, r)
	default:
		http.Error(w, `{"error":{"code":"not_implemented","message":"sporkopstest does not stub this endpoint"}}`, http.StatusNotImplemented)
	}
}

func (f *FakeServer) handleMonitors(w http.ResponseWriter, r *http.Request) {
	// /monitors or /monitors/{id}
	rest := strings.TrimPrefix(r.URL.Path, "/monitors")
	id := strings.Trim(rest, "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && id == "":
		var m spork.Monitor
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		m.ID = f.nextID("mon")
		f.monitors[m.ID] = m
		writeData(w, http.StatusOK, m)
	case r.Method == http.MethodGet && id == "":
		writeList(w, collectMonitors(f.monitors), r)
	case r.Method == http.MethodGet:
		m, ok := f.monitors[id]
		if !ok {
			writeErr(w, http.StatusNotFound, "not_found", "monitor not found")
			return
		}
		writeData(w, http.StatusOK, m)
	case r.Method == http.MethodPatch:
		m, ok := f.monitors[id]
		if !ok {
			writeErr(w, http.StatusNotFound, "not_found", "monitor not found")
			return
		}
		var patch spork.Monitor
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		mergeMonitor(&m, patch)
		f.monitors[id] = m
		writeData(w, http.StatusOK, m)
	case r.Method == http.MethodDelete:
		if _, ok := f.monitors[id]; !ok {
			writeErr(w, http.StatusNotFound, "not_found", "monitor not found")
			return
		}
		delete(f.monitors, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method)
	}
}

func (f *FakeServer) handleAlertChannels(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/alert-channels")
	id := strings.Trim(rest, "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && id == "":
		var ch spork.AlertChannel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		ch.ID = f.nextID("ach")
		f.alerts[ch.ID] = ch
		writeData(w, http.StatusOK, ch)
	case r.Method == http.MethodGet && id == "":
		items := make([]spork.AlertChannel, 0, len(f.alerts))
		for _, v := range f.alerts {
			items = append(items, v)
		}
		writeList(w, items, r)
	case r.Method == http.MethodGet:
		ch, ok := f.alerts[id]
		if !ok {
			writeErr(w, http.StatusNotFound, "not_found", "alert channel not found")
			return
		}
		writeData(w, http.StatusOK, ch)
	case r.Method == http.MethodDelete:
		delete(f.alerts, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method)
	}
}

func (f *FakeServer) handleStatusPages(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/status-pages")
	id := strings.Trim(rest, "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && id == "":
		var sp spork.StatusPage
		if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		sp.ID = f.nextID("sp")
		f.pages[sp.ID] = sp
		writeData(w, http.StatusOK, sp)
	case r.Method == http.MethodGet && id == "":
		items := make([]spork.StatusPage, 0, len(f.pages))
		for _, v := range f.pages {
			items = append(items, v)
		}
		writeList(w, items, r)
	case r.Method == http.MethodGet:
		sp, ok := f.pages[id]
		if !ok {
			writeErr(w, http.StatusNotFound, "not_found", "status page not found")
			return
		}
		writeData(w, http.StatusOK, sp)
	case r.Method == http.MethodDelete:
		delete(f.pages, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method)
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeList(w http.ResponseWriter, items any, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 100
	}
	// The fake doesn't actually paginate large result sets — it returns
	// everything on page 1 and reports an accurate total so the SDK
	// iterator terminates cleanly.
	total := sliceLen(items)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": items,
		"meta": map[string]int{"total": total, "page": page, "per_page": perPage},
	})
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg, "request_id": "req_fake_err"},
	})
}

func collectMonitors(m map[string]spork.Monitor) []spork.Monitor {
	out := make([]spork.Monitor, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func (f *FakeServer) handleRegions(w http.ResponseWriter, r *http.Request) {
	regions := []spork.Region{
		{ID: "us-central1", Name: "US Central (Iowa)"},
		{ID: "europe-west1", Name: "Europe West (Belgium)"},
	}
	writeList(w, regions, r)
}

func (f *FakeServer) handleDeliveryLogs(w http.ResponseWriter, r *http.Request) {
	// The fake returns an empty list; callers needing specific logs
	// should use Handle to inject them.
	writeList(w, []spork.DeliveryLog{}, r)
}

func sliceLen(items any) int {
	switch v := items.(type) {
	case []spork.Monitor:
		return len(v)
	case []spork.AlertChannel:
		return len(v)
	case []spork.StatusPage:
		return len(v)
	case []spork.Region:
		return len(v)
	case []spork.DeliveryLog:
		return len(v)
	case []spork.EmailSubscriber:
		return len(v)
	default:
		return 0
	}
}

// mergeMonitor applies the non-zero fields from patch to dst. The fake only
// looks at the fields most tests care about; callers who need a broader
// merge should Handle their own PATCH endpoint.
func mergeMonitor(dst *spork.Monitor, patch spork.Monitor) {
	if patch.Name != "" {
		dst.Name = patch.Name
	}
	if patch.Target != "" {
		dst.Target = patch.Target
	}
	if patch.Interval != 0 {
		dst.Interval = patch.Interval
	}
	if patch.Paused != nil {
		dst.Paused = patch.Paused
	}
	if len(patch.AlertChannelIDs) > 0 {
		dst.AlertChannelIDs = patch.AlertChannelIDs
	}
	if len(patch.Tags) > 0 {
		dst.Tags = patch.Tags
	}
}
