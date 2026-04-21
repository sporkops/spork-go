package spork

import (
	"fmt"
	"net/url"
	"sort"
)

// defaultLimit is the page size used when ListOptions.Limit is zero.
const defaultLimit = 100

// ListOptions controls pagination and server-side filtering for list
// operations.
//
// Callers who want every record should use the plain ListX(ctx) helpers,
// which transparently auto-paginate on the opaque cursor the server
// returns. ListOptions is for callers who need fine-grained control
// (fetching a single page for a UI, capping the total records they
// traverse, or pushing a filter down to the server rather than
// filtering locally).
type ListOptions struct {
	// Cursor is the opaque pagination token obtained from a previous
	// response's PageInfo.NextCursor. Empty string requests the first page.
	Cursor string
	// Limit is the number of records per page (1-100 at the API). Zero
	// means use the SDK default (100, the API maximum).
	Limit int
	// Filters is an optional map of query parameters to forward to the
	// server. Each endpoint documents which keys it accepts. Unknown
	// keys are forwarded as-is; the server decides whether to honor or
	// ignore them, which keeps the SDK from having to track every
	// endpoint's filter vocabulary by hand.
	//
	// Values are URL-encoded once; empty-string values are omitted so a
	// caller can pass map[string]string{"status": ""} without emitting a
	// no-op filter.
	Filters map[string]string
}

// PageInfo is the pagination metadata returned alongside a list response.
type PageInfo struct {
	// HasMore is true when another page exists after the one just
	// returned. Equivalent to NextCursor != "" — emitted explicitly
	// because the boolean is more ergonomic than a string-emptiness
	// check.
	HasMore bool `json:"has_more"`
	// NextCursor is the opaque token to pass on the next request to
	// retrieve the following page. Empty string when there are no more
	// pages. Callers auto-paginating with the plain ListX helpers do
	// not need to inspect this — the SDK uses it internally.
	NextCursor string `json:"next_cursor,omitempty"`
}

// PageMeta is retained as a type alias for one release so out-of-tree
// callers migrating from v0.6.x don't have to rename every variable.
// It will be removed in v0.8.x.
//
// Deprecated: use PageInfo.
type PageMeta = PageInfo

// query renders ListOptions as a URL query fragment (without the leading "?").
// It fills in the SDK defaults and URL-encodes filter values.
//
// Filter keys are emitted in sorted order so the same logical request
// produces the same URL every call — which matters for caching, test
// assertions, and idempotency-key derivation.
func (o ListOptions) query() string {
	limit := o.Limit
	if limit < 1 {
		limit = defaultLimit
	}

	var q string
	if o.Cursor != "" {
		q = fmt.Sprintf("cursor=%s&limit=%d", url.QueryEscape(o.Cursor), limit)
	} else {
		q = fmt.Sprintf("limit=%d", limit)
	}

	if len(o.Filters) == 0 {
		return q
	}
	keys := make([]string, 0, len(o.Filters))
	for k := range o.Filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := o.Filters[k]
		if v == "" {
			continue
		}
		q += "&" + url.QueryEscape(k) + "=" + url.QueryEscape(v)
	}
	return q
}

// next returns the options for the page after `info`, preserving the
// caller's filters so auto-pagination does not silently drop them on
// page 2 and later.
func (o ListOptions) next(info PageInfo) ListOptions {
	return ListOptions{Cursor: info.NextCursor, Limit: o.Limit, Filters: o.Filters}
}

// maxPages is a safety valve on collectAll to avoid an infinite loop if the
// server returns pathological pagination metadata. It caps a single auto-
// pagination traversal at one million records, which is several orders of
// magnitude beyond any realistic organization size.
const maxPages = 10_000

// collectAll is a generic auto-paginator. It repeatedly invokes fetchPage
// until every record has been retrieved, returning the concatenated slice.
//
// fetchPage must return the records for the requested page and the server's
// pagination metadata. It is invoked at most maxPages times; each invocation
// requests the next page after the one most recently returned.
func collectAll[T any](fetchPage func(opts ListOptions) ([]T, PageInfo, error)) ([]T, error) {
	var all []T
	opts := ListOptions{}
	for i := 0; i < maxPages; i++ {
		page, info, err := fetchPage(opts)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if !info.HasMore {
			return all, nil
		}
		opts = opts.next(info)
	}
	return all, fmt.Errorf("auto-pagination exceeded %d pages; stopping", maxPages)
}
