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
// which transparently auto-paginate. ListOptions is for callers who need
// fine-grained control (e.g. fetching a single page for a UI, capping
// the total number of records they traverse, or pushing a filter down to
// the server rather than filtering locally after fetching).
type ListOptions struct {
	// Page is the 1-indexed page number. Zero is treated as page 1.
	Page int
	// PerPage is the number of records per page (1-100 at the API).
	// Zero means use the SDK default (100, the API maximum).
	PerPage int
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
	// Total is the total number of records across all pages.
	Total int
	// Page is the 1-indexed page number that was returned.
	Page int
	// PerPage is the page size that was returned.
	PerPage int
}

// HasMore reports whether more pages exist after the one just returned.
// It uses Total when the server provided it, and otherwise falls back to
// assuming a short page is the last page.
func (m PageMeta) HasMore(returned int) bool {
	if m.Total > 0 && m.Page > 0 && m.PerPage > 0 {
		return m.Page*m.PerPage < m.Total
	}
	return returned == m.PerPage && returned > 0
}

// query renders ListOptions as a URL query fragment (without the leading "?").
// It fills in the SDK defaults and URL-encodes filter values.
//
// Filter keys are emitted in sorted order so the same logical request
// produces the same URL every call — which matters for caching, test
// assertions, and idempotency-key derivation.
func (o ListOptions) query() string {
	page := o.Page
	if page < 1 {
		page = 1
	}
	perPage := o.PerPage
	if perPage < 1 {
		perPage = defaultPerPage
	}
	q := fmt.Sprintf("page=%d&per_page=%d", page, perPage)
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
func (o ListOptions) next(meta PageMeta) ListOptions {
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
