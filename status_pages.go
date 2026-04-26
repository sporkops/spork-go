package spork

import (
	"context"
	"fmt"
	"net/url"
)

// CreateStatusPage creates a new public status page.
func (c *Client) CreateStatusPage(ctx context.Context, sp *StatusPage) (*StatusPage, error) {
	path, err := c.orgPath(ctx, "/status-pages")
	if err != nil {
		return nil, err
	}
	var result StatusPage
	if err := c.doSingle(ctx, "POST", path, sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListStatusPages returns every status page for the authenticated organization,
// transparently paginating through all pages.
func (c *Client) ListStatusPages(ctx context.Context) ([]StatusPage, error) {
	return collectAll[StatusPage](func(opts ListOptions) ([]StatusPage, PageMeta, error) {
		return c.ListStatusPagesWithOptions(ctx, opts)
	})
}

// ListStatusPagesWithOptions returns a single page of status pages along with
// pagination metadata. Use ListStatusPages if you want every record.
func (c *Client) ListStatusPagesWithOptions(ctx context.Context, opts ListOptions) ([]StatusPage, PageMeta, error) {
	base, err := c.orgPath(ctx, "/status-pages")
	if err != nil {
		return nil, PageMeta{}, err
	}
	var result []StatusPage
	meta, err := c.doList(ctx, "GET", base+"?"+opts.query(), nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetStatusPage returns a single status page by ID.
func (c *Client) GetStatusPage(ctx context.Context, id string) (*StatusPage, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result StatusPage
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateStatusPage updates a status page by ID using HTTP PUT
// (full replacement). Requires the owner role on the server.
//
// Unlike UpdateMonitor (PATCH), this replaces the entire resource —
// any field omitted from sp is reset to its zero value on the server,
// and omitted nested components/component_groups are deleted. Callers
// that want partial-update semantics should fetch the current status
// page first, apply their changes locally, then pass the merged
// struct here.
func (c *Client) UpdateStatusPage(ctx context.Context, id string, sp *StatusPage) (*StatusPage, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var result StatusPage
	if err := c.doSingle(ctx, "PUT", path, sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteStatusPage deletes a status page by ID.
func (c *Client) DeleteStatusPage(ctx context.Context, id string) error {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// SetCustomDomain sets a custom domain on a status page.
func (c *Client) SetCustomDomain(ctx context.Context, statusPageID, domain string) error {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/custom-domain")
	if err != nil {
		return err
	}
	body := map[string]string{"domain": domain}
	return c.doNoContent(ctx, "POST", path, body)
}

// RemoveCustomDomain removes the custom domain from a status page.
func (c *Client) RemoveCustomDomain(ctx context.Context, statusPageID string) error {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/custom-domain")
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// GetCustomDomainStatus returns the provisioning and SSL verification state
// of a status page's custom domain. Poll this after SetCustomDomain to check
// whether the domain is fully active.
func (c *Client) GetCustomDomainStatus(ctx context.Context, statusPageID string) (*CustomDomainStatus, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/custom-domain")
	if err != nil {
		return nil, err
	}
	var result CustomDomainStatus
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateComponent adds a single component to a status page.
func (c *Client) CreateComponent(ctx context.Context, statusPageID string, comp *StatusComponent) (*StatusComponent, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/components")
	if err != nil {
		return nil, err
	}
	var result StatusComponent
	if err := c.doSingle(ctx, "POST", path, comp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateComponent updates a single component on a status page.
func (c *Client) UpdateComponent(ctx context.Context, statusPageID, componentID string, input *UpdateComponentInput) (*StatusComponent, error) {
	path, err := c.orgPath(ctx, fmt.Sprintf("/status-pages/%s/components/%s", url.PathEscape(statusPageID), url.PathEscape(componentID)))
	if err != nil {
		return nil, err
	}
	var result StatusComponent
	if err := c.doSingle(ctx, "PUT", path, input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteComponent removes a component from a status page.
func (c *Client) DeleteComponent(ctx context.Context, statusPageID, componentID string) error {
	path, err := c.orgPath(ctx, fmt.Sprintf("/status-pages/%s/components/%s", url.PathEscape(statusPageID), url.PathEscape(componentID)))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// CreateComponentGroup creates a named group on a status page for organizing components.
func (c *Client) CreateComponentGroup(ctx context.Context, statusPageID string, group *ComponentGroup) (*ComponentGroup, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/component-groups")
	if err != nil {
		return nil, err
	}
	var result ComponentGroup
	if err := c.doSingle(ctx, "POST", path, group, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateComponentGroup updates a component group on a status page.
func (c *Client) UpdateComponentGroup(ctx context.Context, statusPageID, groupID string, group *ComponentGroup) (*ComponentGroup, error) {
	path, err := c.orgPath(ctx, fmt.Sprintf("/status-pages/%s/component-groups/%s", url.PathEscape(statusPageID), url.PathEscape(groupID)))
	if err != nil {
		return nil, err
	}
	var result ComponentGroup
	if err := c.doSingle(ctx, "PUT", path, group, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteComponentGroup removes a component group from a status page.
// Components in the group become ungrouped.
func (c *Client) DeleteComponentGroup(ctx context.Context, statusPageID, groupID string) error {
	path, err := c.orgPath(ctx, fmt.Sprintf("/status-pages/%s/component-groups/%s", url.PathEscape(statusPageID), url.PathEscape(groupID)))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}

// ListSubscribers returns email subscribers for a status page, transparently
// paginating through all pages. Requires the owner role.
func (c *Client) ListSubscribers(ctx context.Context, statusPageID string) ([]EmailSubscriber, error) {
	return collectAll[EmailSubscriber](func(opts ListOptions) ([]EmailSubscriber, PageMeta, error) {
		return c.ListSubscribersWithOptions(ctx, statusPageID, opts)
	})
}

// ListSubscribersWithOptions returns a single page of email subscribers along
// with pagination metadata.
func (c *Client) ListSubscribersWithOptions(ctx context.Context, statusPageID string, opts ListOptions) ([]EmailSubscriber, PageMeta, error) {
	base, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/subscribers")
	if err != nil {
		return nil, PageMeta{}, err
	}
	var result []EmailSubscriber
	meta, err := c.doList(ctx, "GET", base+"?"+opts.query(), nil, &result)
	if err != nil {
		return nil, PageMeta{}, err
	}
	return result, meta, nil
}

// GetSubscriberCount returns the count of confirmed email subscribers for a
// status page.
func (c *Client) GetSubscriberCount(ctx context.Context, statusPageID string) (int, error) {
	path, err := c.orgPath(ctx, "/status-pages/"+url.PathEscape(statusPageID)+"/subscribers/count")
	if err != nil {
		return 0, err
	}
	var result SubscriberCount
	if err := c.doSingle(ctx, "GET", path, nil, &result); err != nil {
		return 0, err
	}
	return result.Count, nil
}

// DeleteSubscriber removes an email subscriber from a status page.
// Requires the owner role.
func (c *Client) DeleteSubscriber(ctx context.Context, statusPageID, subscriberID string) error {
	path, err := c.orgPath(ctx, fmt.Sprintf("/status-pages/%s/subscribers/%s", url.PathEscape(statusPageID), url.PathEscape(subscriberID)))
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, "DELETE", path, nil)
}
