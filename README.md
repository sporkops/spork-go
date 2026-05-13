# spork-go

[![Go Reference](https://pkg.go.dev/badge/github.com/sporkops/spork-go.svg)](https://pkg.go.dev/github.com/sporkops/spork-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Official Go SDK for the [Spork](https://sporkops.com) uptime monitoring API.

- Zero external dependencies
- Automatic retries with exponential backoff
- Typed CRUD for monitors, alert channels, status pages, and incidents
- Used by the [Spork CLI](https://github.com/sporkops/cli) and [Terraform provider](https://github.com/sporkops/terraform-provider-sporkops)

## Install

```bash
go get github.com/sporkops/spork-go
```

Requires Go 1.24+.

## Quick start

```go
import spork "github.com/sporkops/spork-go"

client := spork.NewClient(
    spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
)

// Create a monitor
monitor, err := client.CreateMonitor(ctx, &spork.Monitor{
    Name:           "API Health",
    Target:         "https://api.example.com/health",
    Interval:       60,
    ExpectedStatus: 200,
    Regions:        []string{"us-central1", "europe-west1"},
})

// List all monitors
monitors, err := client.ListMonitors(ctx)

// Create a status page with components
page, err := client.CreateStatusPage(ctx, &spork.StatusPage{
    Name:     "Acme Status",
    Slug:     "acme",
    IsPublic: true,
    Components: []spork.StatusComponent{
        {MonitorID: monitor.ID, DisplayName: "API", Order: 0},
    },
})
```

## Authentication

All API calls require an API key (prefixed `sk_`). Create one at
[sporkops.com/settings/api-keys](https://sporkops.com/settings/api-keys) or via the CLI:

```bash
spork api-key create
```

## Client options

```go
client := spork.NewClient(
    spork.WithAPIKey("sk_live_..."),                   // required (or via env)
    spork.WithOrganization("org_acme"),                // required for multi-org users
    spork.WithBaseURL("https://api.sporkops.com/v1"),  // default
    spork.WithUserAgent("my-app/1.0"),                 // optional prefix
    spork.WithHTTPClient(customHTTPClient),            // optional
)
```

### Env-var defaults

For twelve-factor configs, `WithEnvDefaults` reads the three common knobs
from the environment in one shot:

```go
client := spork.NewClient(spork.WithEnvDefaults())
```

| Variable                                | What it sets    |
|-----------------------------------------|-----------------|
| `SPORK_API_KEY`                         | API key         |
| `SPORK_ORGANIZATION_ID` (or `SPORK_ORG_ID`) | Organization ID |
| `SPORK_BASE_URL`                        | Base URL        |

Options passed after `WithEnvDefaults()` override the env-derived values,
so you can pin one knob and inherit the rest:

```go
client := spork.NewClient(
    spork.WithEnvDefaults(),
    spork.WithOrganization("org_pinned"),  // override env, keep env API key
)
```

## Multi-org

Pass `WithOrganization` to scope a client to a tenant:

```go
client := spork.NewClient(
    spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
    spork.WithOrganization("org_acme"),
)
```

API keys are bound to a single org, so for the common single-tenant case
you can omit `WithOrganization` and the SDK will resolve it lazily on the
first org-scoped call. Firebase callers (humans, not API keys) who belong
to more than one org **must** set it explicitly — the resolver refuses to
guess.

To run the same call against several orgs from one client, use `ForOrg`
to get a per-call clone instead of mutating the receiver:

```go
client := spork.NewClient(spork.WithAPIKey(os.Getenv("SPORK_API_KEY")))

acmeMonitors,  _ := client.ForOrg("org_acme").ListMonitors(ctx)
widgetsMonitors, _ := client.ForOrg("org_widgets").ListMonitors(ctx)
```

`ForOrg` shares the underlying transport, rate-limit snapshot, and retry
policy — there's no per-org connection cost. Prefer it over the older
`SetOrganization` method, which mutates the receiver in place and races
against any org-scoped call already in flight (`SetOrganization` is
deprecated and will be removed in v1.0).

## Resources

**Monitors** — `CreateMonitor` · `ListMonitors` · `GetMonitor` · `UpdateMonitor` · `DeleteMonitor` · `GetMonitorResults`

**Alert Channels** — `CreateAlertChannel` · `ListAlertChannels` · `GetAlertChannel` · `UpdateAlertChannel` · `DeleteAlertChannel` · `TestAlertChannel`

**Status Pages** — `CreateStatusPage` · `ListStatusPages` · `GetStatusPage` · `UpdateStatusPage` · `DeleteStatusPage` · `SetCustomDomain` · `RemoveCustomDomain`

**Incidents** — `CreateIncident` · `ListIncidents` · `ListRecentIncidents` · `GetIncident` · `UpdateIncident` · `DeleteIncident` · `CreateIncidentUpdate` · `ListIncidentUpdates`

**API Keys** — `CreateAPIKey` · `ListAPIKeys` · `DeleteAPIKey`

**Account** — `GetAccount`

## Error handling

```go
import "errors"

_, err := client.GetMonitor(ctx, "mon_nonexistent")

if spork.IsNotFound(err) {
    // 404
}
if spork.IsUnauthorized(err) {
    // 401 — invalid or expired API key
}
if spork.IsRateLimited(err) {
    // 429 — auto-retried, but all attempts exhausted
}

// Structured error details
var apiErr *spork.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.RequestID)
}
```

## Retries

Transient errors (429, 503, 504) are retried automatically with exponential
backoff (up to 3 attempts). The client respects `Retry-After` headers.

## License

[MIT](LICENSE)
