package spork

import (
	"context"
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

// ListIncidents returns all incidents for a status page.
func (c *Client) ListIncidents(ctx context.Context, statusPageID string) ([]Incident, error) {
	var result []Incident
	path := "/status-pages/" + url.PathEscape(statusPageID) + "/incidents?per_page=100"
	if err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
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

// CreateIncidentUpdate adds a timeline update to an incident.
func (c *Client) CreateIncidentUpdate(ctx context.Context, incidentID string, upd *IncidentUpdate) (*IncidentUpdate, error) {
	var result IncidentUpdate
	path := "/incidents/" + url.PathEscape(incidentID) + "/updates"
	if err := c.doSingle(ctx, "POST", path, upd, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListIncidentUpdates returns all timeline updates for an incident.
func (c *Client) ListIncidentUpdates(ctx context.Context, incidentID string) ([]IncidentUpdate, error) {
	var result []IncidentUpdate
	path := "/incidents/" + url.PathEscape(incidentID) + "/updates"
	if err := c.doList(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
