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
	KeywordType     string            `json:"keyword_type,omitempty"`    // "exists" or "not_exists"
	SSLWarnDays     int               `json:"ssl_warn_days,omitempty"`   // days before SSL expiry to alert
	AlertChannelIDs []string          `json:"alert_channel_ids,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Paused          *bool             `json:"paused,omitempty"`
	Status            string                    `json:"status,omitempty"`              // read-only: "up", "down", "degraded", "paused", "pending"
	UptimePercentage  float64                   `json:"uptime_percentage,omitempty"`   // read-only: 24-hour uptime percentage (0-100)
	AvgResponseTimeMs int64                     `json:"avg_response_time_ms,omitempty"` // read-only: 24-hour average response time in ms
	LastCheckedAt     string                    `json:"last_checked_at,omitempty"`     // read-only
	NextCheckAt       string                    `json:"next_check_at,omitempty"`       // read-only
	LiveStatus        map[string]RegionStatus   `json:"live_status,omitempty"`         // read-only: per-region live check status
	CreatedAt         string                    `json:"created_at,omitempty"`          // read-only
	UpdatedAt         string                    `json:"updated_at,omitempty"`          // read-only

	// ActiveMaintenanceWindowID is read-only: the ID of the maintenance
	// window currently suppressing this monitor (empty when the monitor
	// is not inside an active window). The server computes this field
	// opportunistically on GetMonitor so the UI can show a "Maintenance"
	// badge without a second request.
	ActiveMaintenanceWindowID string `json:"active_maintenance_window_id,omitempty"`
}

// RegionStatus represents the live check status for a specific monitoring region.
type RegionStatus struct {
	Status         string `json:"status"`           // "up" or "down"
	ResponseTimeMs int64  `json:"response_time_ms"`
	HTTPCode       int    `json:"http_code"`
	CheckedAt      string `json:"checked_at"`
	Error          string `json:"error,omitempty"`
}

// Organization represents the authenticated user's organization.
type Organization struct {
	ID            string         `json:"id"`
	Name          string         `json:"name,omitempty"`
	Subscriptions []Subscription `json:"subscriptions"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	User          *OrganizationUser `json:"user,omitempty"`
}

// Subscription represents a product subscription within an organization.
type Subscription struct {
	Product           string         `json:"product"`
	Plan              string         `json:"plan"`
	Entitlements      map[string]any `json:"entitlements"`
	HasPaymentMethod  bool           `json:"has_payment_method"`
	CancelAtPeriodEnd bool           `json:"cancel_at_period_end"`
	CancelAt             *time.Time  `json:"cancel_at,omitempty"`
	TrialEndsAt          *time.Time  `json:"trial_ends_at,omitempty"`
	TrialDaysRemaining   *int        `json:"trial_days_remaining,omitempty"`
}

// OrganizationUser is the authenticated user's info within an organization.
type OrganizationUser struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Subscription returns the subscription for the given product, or nil.
func (o *Organization) Subscription(product string) *Subscription {
	for i := range o.Subscriptions {
		if o.Subscriptions[i].Product == product {
			return &o.Subscriptions[i]
		}
	}
	return nil
}

// EntitlementInt returns the integer value of an entitlement, or 0 if missing.
// Handles float64 from JSON unmarshaling.
func (s *Subscription) EntitlementInt(key string) int {
	if s == nil {
		return 0
	}
	v, ok := s.Entitlements[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// EntitlementBool returns the boolean value of an entitlement, or false if missing.
func (s *Subscription) EntitlementBool(key string) bool {
	if s == nil {
		return false
	}
	v, ok := s.Entitlements[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// Member represents a member of an organization.
type Member struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id,omitempty"`  // Firebase UID (empty while invite is pending)
	Email          string     `json:"email"`
	Role           string     `json:"role"`               // "owner" or "member"
	Status         string     `json:"status"`             // "pending" or "accepted"
	InvitedBy      string     `json:"invited_by,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"` // nil once accepted; invites expire after 7 days
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// InviteMemberInput is the request body for inviting a member.
type InviteMemberInput struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

// TransferOwnershipInput is the request body for transferring ownership.
type TransferOwnershipInput struct {
	MemberID string `json:"member_id"`
}

// TransferOwnershipResult is the response for a successful ownership transfer.
type TransferOwnershipResult struct {
	NewOwner      Member `json:"new_owner"`
	PreviousOwner Member `json:"previous_owner"`
}

// OrganizationUsage contains usage metrics for each product subscription.
type OrganizationUsage struct {
	Subscriptions []SubscriptionUsage `json:"subscriptions"`
}

// SubscriptionUsage contains usage metrics for a single product subscription.
type SubscriptionUsage struct {
	Product string         `json:"product"`
	Plan    string         `json:"plan"`
	Usage   map[string]int `json:"usage"`
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
// Supported types: "email", "webhook", "slack", "discord", "teams",
// "pagerduty", "telegram", "googlechat". The Config map holds type-specific
// settings (e.g., {"to": "oncall@example.com"} for email, {"url": "..."} for
// slack/discord/teams/googlechat/webhook, {"integration_key": "..."} for
// pagerduty, {"bot_token": "...", "chat_id": "..."} for telegram).
type AlertChannel struct {
	ID                 string            `json:"id,omitempty"`
	Name               string            `json:"name"`
	Type               string            `json:"type"`   // "email", "webhook", "slack", "discord", "teams", "pagerduty", "telegram", "googlechat"
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
	Theme                   string            `json:"theme,omitempty"`             // "light", "dark", "blue", "midnight"
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
//
// MaintenanceWindowID is set by the server when the incident was
// auto-synced from a MaintenanceWindow — clients should not write it.
type Incident struct {
	ID                  string   `json:"id,omitempty"`
	StatusPageID        string   `json:"status_page_id,omitempty"` // read-only
	Title               string   `json:"title"`
	Message             string   `json:"message,omitempty"`
	Type                string   `json:"type,omitempty"`           // "incident" or "maintenance"
	Status              string   `json:"status,omitempty"`         // see type docs above
	Impact              string   `json:"impact,omitempty"`         // "none", "minor", "major", "critical"
	ComponentIDs        []string `json:"component_ids,omitempty"`  // affected StatusComponent IDs
	StartedAt           string   `json:"started_at,omitempty"`
	ResolvedAt          string   `json:"resolved_at,omitempty"`    // read-only: set when status becomes resolved
	ScheduledStart      string   `json:"scheduled_start,omitempty"` // maintenance only
	ScheduledEnd        string   `json:"scheduled_end,omitempty"`   // maintenance only
	MaintenanceWindowID string   `json:"maintenance_window_id,omitempty"` // read-only: populated on auto-synced maintenance incidents
	CreatedAt           string   `json:"created_at,omitempty"`      // read-only
	UpdatedAt           string   `json:"updated_at,omitempty"`      // read-only
}

// IncidentUpdate represents a timeline entry on an incident.
type IncidentUpdate struct {
	ID         string `json:"id,omitempty"`         // read-only
	IncidentID string `json:"incident_id,omitempty"` // read-only
	Status     string `json:"status,omitempty"`      // status at time of update
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`  // read-only
}

// MaintenanceWindow represents a scheduled period during which alerts are
// suppressed (and optionally checks are paused) for a set of monitors.
//
// Targeting: set exactly one of MonitorIDs, TagSelectors, or AllMonitors.
// Tag selectors match monitors whose Tags intersect the given list
// (OR semantics, matching UptimeRobot).
//
// Scheduling: StartAt and EndAt are RFC3339 UTC timestamps. Timezone is
// an IANA name (e.g., "America/Los_Angeles") used for DST-aware recurrence
// expansion and for display.
//
// Recurrence: leave RecurrenceType empty for a one-time window. For
// "weekly", RecurrenceDays is a list of [0..6] (Sunday=0). For "monthly",
// RecurrenceDays is a list of [1..31].
//
// Behavior: SuppressAlerts (default true) gates alert delivery during
// the window. ExcludeFromUptime (default true) drops checks in the
// window from uptime-percentage calculations. PauseChecks (default
// false) skips dispatch entirely — most callers want checks to keep
// running so data is preserved.
//
// Read-only: State, CancelledAt, CreatedBy, CreatedAt, UpdatedAt.
type MaintenanceWindow struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// Targeting — set exactly one.
	MonitorIDs   []string `json:"monitor_ids,omitempty"`
	TagSelectors []string `json:"tag_selectors,omitempty"`
	AllMonitors  *bool    `json:"all_monitors,omitempty"`

	// Scheduling
	Timezone string `json:"timezone,omitempty"` // IANA name, required
	StartAt  string `json:"start_at,omitempty"` // RFC3339 UTC
	EndAt    string `json:"end_at,omitempty"`   // RFC3339 UTC

	// Recurrence — empty type means one-time.
	RecurrenceType  string `json:"recurrence_type,omitempty"`  // "", "daily", "weekly", "monthly"
	RecurrenceDays  []int  `json:"recurrence_days,omitempty"`  // weekly: 0-6 (Sun-Sat); monthly: 1-31
	RecurrenceUntil string `json:"recurrence_until,omitempty"` // RFC3339 UTC

	// Behavior
	SuppressAlerts    *bool `json:"suppress_alerts,omitempty"`     // default true
	ExcludeFromUptime *bool `json:"exclude_from_uptime,omitempty"` // default true
	PauseChecks       *bool `json:"pause_checks,omitempty"`        // default false

	// Read-only
	State       string `json:"state,omitempty"`
	CancelledAt string `json:"cancelled_at,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// MaintenanceWindow state values returned by the API.
const (
	MaintenanceStateScheduled  = "scheduled"
	MaintenanceStateInProgress = "in_progress"
	MaintenanceStateCompleted  = "completed"
	MaintenanceStateCancelled  = "cancelled"
)

// MaintenanceWindow recurrence type values.
const (
	MaintenanceRecurrenceDaily   = "daily"
	MaintenanceRecurrenceWeekly  = "weekly"
	MaintenanceRecurrenceMonthly = "monthly"
)

// Region represents an available monitoring region.
type Region struct {
	ID   string `json:"id"`   // GCP region identifier, e.g. "us-central1"
	Name string `json:"name"` // human-readable name, e.g. "US Central (Iowa)"
}

// MonitorStats contains 24-hour aggregate statistics for a monitor.
// Stats are cached server-side for 5 minutes.
//
// AvgResponseTimeMs is int64 to match Monitor.AvgResponseTimeMs.
type MonitorStats struct {
	UptimePercentage  float64 `json:"uptime_percentage"`    // 0-100
	AvgResponseTimeMs int64   `json:"avg_response_time_ms"` // milliseconds
}

// CustomDomainStatus describes the provisioning and verification state
// of a status page's custom domain.
type CustomDomainStatus struct {
	Domain      string `json:"domain"`
	Status      string `json:"status"`       // "pending", "active", or "failed"
	SSLStatus   string `json:"ssl_status"`
	DNSVerified bool   `json:"dns_verified"` // whether CNAME points to status.sporkops.com
	CNAMETarget string `json:"cname_target"` // e.g. "status.sporkops.com"
	DNSProvider string `json:"dns_provider"`
}

// AuditTrailEntry represents a single change in a monitor's audit trail.
type AuditTrailEntry struct {
	ID         string                       `json:"id"`
	Timestamp  string                       `json:"timestamp"`
	ActorEmail string                       `json:"actor_email"`
	Source     string                       `json:"source"` // "ui", "api", "cli", "terraform"
	Action     string                       `json:"action"` // "created", "updated", "deleted"
	Changes    map[string]AuditTrailChange  `json:"changes,omitempty"`
}

// AuditTrailChange represents the old and new values of a single field change.
type AuditTrailChange struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// DeliveryLog records a single alert delivery attempt.
type DeliveryLog struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	ChannelType string `json:"channel_type"`
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	Event       string `json:"event"`    // "monitor.down", "monitor.up", "test"
	Status      string `json:"status"`   // "success" or "failed"
	Attempts    int    `json:"attempts"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// EmailSubscriber represents a status page email subscriber.
type EmailSubscriber struct {
	ID           string `json:"id"`
	StatusPageID string `json:"status_page_id"`
	Email        string `json:"email"`
	Confirmed    bool   `json:"confirmed"`
	CreatedAt    string `json:"created_at"`
	ConfirmedAt  string `json:"confirmed_at,omitempty"`
}

// AcceptInviteInput is the request body for accepting an organization invite.
type AcceptInviteInput struct {
	Token string `json:"token"`
}

// AcceptInviteResult is the response after accepting an invite.
type AcceptInviteResult struct {
	Status string `json:"status"` // "accepted"
}

// SubscriberCount is the response for GetSubscriberCount.
type SubscriberCount struct {
	Count int `json:"count"`
}

// UpdateComponentInput is the request body for updating a single component
// via PUT /status-pages/{id}/components/{componentId}. Because the endpoint
// uses PUT (full replacement), omitted fields on the server are reset.
//
// The fields below intentionally do NOT use `omitempty` so callers can:
//   - set Order to 0 (valid first position)
//   - clear Description by passing ""
//   - ungroup a component by passing GroupID: "" (per API contract)
//
// DisplayName is required by the server; the other fields are preserved
// at their current value only if the caller round-trips them.
type UpdateComponentInput struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	GroupID     string `json:"group_id"`
	Order       int    `json:"order"`
}
