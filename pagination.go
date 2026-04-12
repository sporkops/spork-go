package spork

import (
	"fmt"
	"net/url"
	"sort"
)

// defaultPerPage is the page size used when ListOptions.PerPage is zero.
const defaultPerPage = 100

// ListOptions controls pagination and server-side filtering for list
// operations.
//
// Callers who want every record should use the plain ListX(ctx) helpers,
// which transparently auto-paginate and prefer cursor-based pagination
// when the server provides a next_cursor. ListOptions is for callers
// who need fine-grained control (fetching a single page for a UI,
// capping the total records they traverse, or pushing a filter down
// to the server rather than filtering locally).
//
// Pagination mode: when Cursor is non-empty, it takes precedence over
// Page (the server honors the cursor and ignores the page number). The
// plain Page field is kept for callers with a legacy integration; new
// code should iterate on the cursor returned in PageMeta.NextCursor.
type ListOptions struct {
	// Page is the 1-indexed page number. Zero is treated as page 1.
	// Ignored when Cursor is set.
	Page int
	// PerPage is the number of records per page (1-100 at the API).
	// Zero means use the SDK default (100, the API maximum).
	PerPage int
	// Cursor is an opaque pagination token obtained from a previous
	// response's PageMeta.NextCursor. Non-empty Cursor takes precedence
	// over Page.
	Cursor string
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

// PageMeta is the pagination metadata returned alongside a list response.
type PageMeta struct {
	// Total is the total number of records across all pages. For
	// cursor-native endpoints this may be zero; iterate on HasMore
	// instead.
	Total int
	// Page is the 1-indexed page number that was returned.
	Page int
	// PerPage is the page size that was returned.
	PerPage int
	// NextCursor is the opaque token to pass on the next request to
	// retrieve the following page. Empty string when there are no more
	// pages. Callers auto-paginating with the plain ListX helpers do not
	// need to inspect this — the SDK uses it internally.
	NextCursor string `json:"next_cursor,omitempty"`
}

// HasMore reports whether more pages exist after the one just returned.
//
// Resolution order:
//
//  1. If NextCursor is non-empty, there is explicitly another page.
//  2. If Total/Page/PerPage are populated (legacy page endpoints), use
//     the offset arithmetic.
//  3. Otherwise fall back to "a short page is the last page."
func (m PageMeta) HasMore(returned int) bool {
	if m.NextCursor != "" {
		return true
	}
	if m.Total > 0 && m.Page > 0 && m.PerPage > 0 {
		return m.Page*m.PerPage < m.Total
	}
	return returned == m.PerPage && returned > 0
}

// query renders ListOptions as a URL query fragment (without the leading "?").
// It fills in the SDK defaults and URL-encodes filter values.
//
// Pagination mode:
//
//   - When Cursor is set, emit `cursor=...&limit=N` and omit `page`.
//   - Otherwise emit the legacy `page=N&per_page=N` pair.
//
// Filter keys are emitted in sorted order so the same logical request
// produces the same URL every call — which matters for caching, test
// assertions, and idempotency-key derivation.
func (o ListOptions) query() string {
	perPage := o.PerPage
	if perPage < 1 {
		perPage = defaultPerPage
	}

	var q string
	if o.Cursor != "" {
		// Cursor mode: server takes `cursor` + `limit`.
		q = fmt.Sprintf("cursor=%s&limit=%d", url.QueryEscape(o.Cursor), perPage)
	} else {
		page := o.Page
		if page < 1 {
			page = 1
		}
		q = fmt.Sprintf("page=%d&per_page=%d", page, perPage)
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

// next returns the options for the page after `meta`, preserving the
// caller's filters so auto-pagination does not silently drop them on
// page 2 and later.
//
// Prefers the server's opaque cursor when provided: this fixes the
// `page=1000` ceiling for auto-paginators on large organizations, and
// automatically opts in to whatever keyset scheme the server migrates
// to behind the cursor.
func (o ListOptions) next(meta PageMeta) ListOptions {
	if meta.NextCursor != "" {
		return ListOptions{Cursor: meta.NextCursor, PerPage: o.PerPage, Filters: o.Filters}
	}
	return ListOptions{Page: meta.Page + 1, PerPage: o.PerPage, Filters: o.Filters}
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
func collectAll[T any](fetchPage func(opts ListOptions) ([]T, PageMeta, error)) ([]T, error) {
	var all []T
	opts := ListOptions{}
	for i := 0; i < maxPages; i++ {
		page, meta, err := fetchPage(opts)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if !meta.HasMore(len(page)) {
			return all, nil
		}
		opts = opts.next(meta)
	}
	return all, fmt.Errorf("auto-pagination exceeded %d pages; stopping", maxPages)
}
