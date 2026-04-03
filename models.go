package spork

import "time"

// Monitor represents an uptime monitor that periodically checks a target URL.
//
// When creating a monitor, set Name, Target, and optionally Type, Method,
// Interval, ExpectedStatus, Regions, Headers, Body, Keyword, KeywordType,
// SSLWarnDays, AlertChannelIDs, Tags, and Paused.
//
// Fields like ID, Status, LastCheckedAt, CreatedAt, and UpdatedAt are
// read-only and populated by the API.
type Monitor struct {
	ID              string            `json:"id,omitempty"`
	Name            string            `json:"name,omitempty"`
	Type            string            `json:"type,omitempty"`            // "http", "ssl", "dns", "keyword", "tcp", "ping"
	Target          string            `json:"target,omitempty"`          // URL or hostname to check
	Method          string            `json:"method,omitempty"`          // HTTP method (default: "GET")
	ExpectedStatus  int               `json:"expected_status,omitempty"` // expected HTTP status code (default: 200)
	Interval        int               `json:"interval,omitempty"`        // check interval in seconds
	Timeout         int               `json:"timeout,omitempty"`         // request timeout in seconds
	Regions         []string          `json:"regions,omitempty"`         // GCP regions to check from
	Headers         map[string]string `json:"headers,omitempty"`         // custom HTTP headers
	Body            string            `json:"body,omitempty"`            // request body for POST/PUT checks
	Keyword         string            `json:"keyword,omitempty"`         // keyword to search for in response
	KeywordType     string            `json:"keyword_type,omitempty"`    // "contains" or "not_contains"
	SSLWarnDays     int               `json:"ssl_warn_days,omitempty"`   // days before SSL expiry to alert
	AlertChannelIDs []string          `json:"alert_channel_ids,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Paused          *bool             `json:"paused,omitempty"`
	Status          string            `json:"status,omitempty"`           // read-only: "up", "down", "degraded", "paused"
	LastCheckedAt   string            `json:"last_checked_at,omitempty"`  // read-only
	CreatedAt       string            `json:"created_at,omitempty"`       // read-only
	UpdatedAt       string            `json:"updated_at,omitempty"`       // read-only
}

// Account represents the authenticated user's account info.
type Account struct {
	UID              string    `json:"uid"`
	Email            string    `json:"email"`
	Plan             string    `json:"plan"`               // "free", "starter", "pro", etc.
	MonitorLimit     int       `json:"monitor_limit"`       // max monitors allowed by plan
	CheckIntervalS   int       `json:"check_interval_s"`    // minimum check interval allowed by plan
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	HasPaymentMethod bool      `json:"has_payment_method"`
}

// APIKey represents an API key for programmatic access.
// The full Key value is only returned once, at creation time.
type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Key        string     `json:"key,omitempty"`        // full key (only in create response)
	Prefix     string     `json:"prefix"`               // visible prefix, e.g. "sk_live_abc..."
	ExpiresAt  *time.Time `json:"expires_at,omitempty"` // nil means no expiry
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// MonitorResult represents a single uptime check result.
type MonitorResult struct {
	ID             string `json:"id"`
	MonitorID      string `json:"monitor_id"`
	Status         string `json:"status"`           // "up" or "down"
	StatusCode     int    `json:"status_code"`      // HTTP response status code
	ResponseTimeMs int64  `json:"response_time_ms"` // response time in milliseconds
	Region         string `json:"region"`           // GCP region that performed the check
	ErrorMessage   string `json:"error_message,omitempty"`
	CheckedAt      string `json:"checked_at"`
}

// AlertChannel represents a notification channel for monitor alerts.
//
// Supported types: "email", "webhook". The Config map holds type-specific
// settings (e.g., {"to": "oncall@example.com"} for email channels).
type AlertChannel struct {
	ID                 string            `json:"id,omitempty"`
	Name               string            `json:"name"`
	Type               string            `json:"type"`   // "email" or "webhook"
	Config             map[string]string `json:"config"` // type-specific configuration
	Verified           bool              `json:"verified,omitempty"`            // read-only
	Secret             string            `json:"secret,omitempty"`              // read-only: webhook signing secret
	LastDeliveryStatus string            `json:"last_delivery_status,omitempty"` // read-only
	LastDeliveryAt     string            `json:"last_delivery_at,omitempty"`     // read-only
	CreatedAt          string            `json:"created_at,omitempty"`           // read-only
	UpdatedAt          string            `json:"updated_at,omitempty"`           // read-only
}

// StatusPage represents a public status page with customizable branding.
type StatusPage struct {
	ID                      string            `json:"id,omitempty"`
	Name                    string            `json:"name"`
	Slug                    string            `json:"slug"`                        // URL slug: {slug}.status.sporkops.com
	Components              []StatusComponent `json:"components,omitempty"`        // monitors displayed on the page
	ComponentGroups         []ComponentGroup  `json:"component_groups,omitempty"`  // optional grouping
	CustomDomain            string            `json:"custom_domain,omitempty"`     // read-only: use SetCustomDomain
	DomainStatus            string            `json:"domain_status,omitempty"`     // read-only: "pending", "active"
	Theme                   string            `json:"theme,omitempty"`             // "light" or "dark"
	AccentColor             string            `json:"accent_color,omitempty"`      // hex color, e.g. "#4F46E5"
	FontFamily              string            `json:"font_family,omitempty"`
	HeaderStyle             string            `json:"header_style,omitempty"`
	LogoURL                 string            `json:"logo_url,omitempty"`
	WebhookURL              string            `json:"webhook_url,omitempty"`
	EmailSubscribersEnabled bool              `json:"email_subscribers_enabled"`
	IsPublic                bool              `json:"is_public"`
	Password                string            `json:"password,omitempty"`          // set to password-protect the page
	CreatedAt               string            `json:"created_at,omitempty"`        // read-only
	UpdatedAt               string            `json:"updated_at,omitempty"`        // read-only
}

// ComponentGroup organizes components into named sections on the status page.
type ComponentGroup struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order"`
}

// StatusComponent maps a monitor to a display name on a status page.
type StatusComponent struct {
	ID          string `json:"id,omitempty"`
	MonitorID   string `json:"monitor_id"`              // the monitor this component tracks
	DisplayName string `json:"display_name"`            // label shown on the status page
	Description string `json:"description,omitempty"`
	GroupID     string `json:"group_id,omitempty"`       // optional: ComponentGroup.ID
	GroupName   string `json:"group_name,omitempty"`     // optional: resolved to GroupID by server
	Order       int    `json:"order"`
}

// Incident represents a status page incident or scheduled maintenance.
//
// Type is "incident" or "maintenance". Status progresses through
// "investigating" -> "identified" -> "monitoring" -> "resolved" for incidents,
// or "scheduled" -> "in_progress" -> "completed" for maintenance.
type Incident struct {
	ID             string   `json:"id,omitempty"`
	StatusPageID   string   `json:"status_page_id,omitempty"` // read-only
	Title          string   `json:"title"`
	Message        string   `json:"message,omitempty"`
	Type           string   `json:"type,omitempty"`           // "incident" or "maintenance"
	Status         string   `json:"status,omitempty"`         // see type docs above
	Impact         string   `json:"impact,omitempty"`         // "none", "minor", "major", "critical"
	ComponentIDs   []string `json:"component_ids,omitempty"`  // affected StatusComponent IDs
	StartedAt      string   `json:"started_at,omitempty"`
	ResolvedAt     string   `json:"resolved_at,omitempty"`    // read-only: set when status becomes resolved
	ScheduledStart string   `json:"scheduled_start,omitempty"` // maintenance only
	ScheduledEnd   string   `json:"scheduled_end,omitempty"`   // maintenance only
	CreatedAt      string   `json:"created_at,omitempty"`      // read-only
	UpdatedAt      string   `json:"updated_at,omitempty"`      // read-only
}

// IncidentUpdate represents a timeline entry on an incident.
type IncidentUpdate struct {
	ID         string `json:"id,omitempty"`         // read-only
	IncidentID string `json:"incident_id,omitempty"` // read-only
	Status     string `json:"status,omitempty"`      // status at time of update
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`  // read-only
}
