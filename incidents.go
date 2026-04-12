package spork

import (
	"context"
	"fmt"
	"net/url"
)

// CreateIncident creates a new incident on a status page.
func (c *Client) CreateIncident(ctx context.Context, statusPageID string, inc *Incident) (*Incident, error) {
	var result Incident
	path := "/status-pages/" + url.PathEscape(statusPageID) + "/incidents"
	if err := c.doSingle(ctx, "POST", path, inc, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListIncidents returns every incident for a status page, transparently
// paginating through all pages.
func (c *Client) ListIncidents(ctx context.Context, statusPageID string) ([]Incident, error) {
	return collectAll[Incident](func(opts ListOptions) ([]Incident, PageMeta, error) {
		return c.ListIncidentsPage(ctx, statusPageID, opts)
	})
}

// ListIncidentsPage returns a single page of incidents for a status page
// along with pagination metadata. Use ListIncidents if you want every record.
func (c *Client) ListIncidentsPage(ctx context.Context, statusPageID string, opts ListOptions) ([]Incident, PageMeta, error) {
	var result []Incident
	path := "/status-pages/" + url.PathEscape(statusPageID) + "/incidents?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetIncident returns a single incident by ID.
func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	var result Incident
	if err := c.doSingle(ctx, "GET", "/incidents/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateIncident partially updates an incident by ID.
func (c *Client) UpdateIncident(ctx context.Context, id string, inc *Incident) (*Incident, error) {
	var result Incident
	if err := c.doSingle(ctx, "PATCH", "/incidents/"+url.PathEscape(id), inc, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteIncident deletes an incident by ID.
func (c *Client) DeleteIncident(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", "/incidents/"+url.PathEscape(id), nil)
}

// ListRecentIncidents returns recent incidents across every status page in the
// caller's organization, newest-first. Pass limit <= 0 for the server-side
// default (50). The server caps limit at 100.
//
// Unlike ListIncidents, this method does not auto-paginate — it returns a
// single bounded slice, which is the right shape for UI and dashboard use
// cases that only want "the N most recent".
func (c *Client) ListRecentIncidents(ctx context.Context, limit int) ([]Incident, error) {
	var result []Incident
	path := "/incidents"
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}
	if _, err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateIncidentUpdate adds a timeline update to an incident.
func (c *Client) CreateIncidentUpdate(ctx context.Context, incidentID string, upd *IncidentUpdate) (*IncidentUpdate, error) {
	var result IncidentUpdate
	path := "/incidents/" + url.PathEscape(incidentID) + "/updates"
	if err := c.doSingle(ctx, "POST", path, upd, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListIncidentUpdates returns every timeline update for an incident,
// transparently paginating through all pages.
func (c *Client) ListIncidentUpdates(ctx context.Context, incidentID string) ([]IncidentUpdate, error) {
	return collectAll[IncidentUpdate](func(opts ListOptions) ([]IncidentUpdate, PageMeta, error) {
		return c.ListIncidentUpdatesPage(ctx, incidentID, opts)
	})
}

// ListIncidentUpdatesPage returns a single page of incident updates along
// with pagination metadata. Use ListIncidentUpdates if you want every record.
func (c *Client) ListIncidentUpdatesPage(ctx context.Context, incidentID string, opts ListOptions) ([]IncidentUpdate, PageMeta, error) {
	var result []IncidentUpdate
	path := "/incidents/" + url.PathEscape(incidentID) + "/updates?" + opts.query()
	meta, err := c.doList(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}
