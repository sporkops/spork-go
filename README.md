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
    spork.WithAPIKey("sk_live_..."),                   // required
    spork.WithBaseURL("https://api.sporkops.com/v1"),  // default
    spork.WithUserAgent("my-app/1.0"),                 // optional prefix
    spork.WithHTTPClient(customHTTPClient),             // optional
)
```

## Resources

**Monitors** — `CreateMonitor` · `ListMonitors` · `GetMonitor` · `UpdateMonitor` · `DeleteMonitor` · `GetMonitorResults`

**Alert Channels** — `CreateAlertChannel` · `ListAlertChannels` · `GetAlertChannel` · `UpdateAlertChannel` · `DeleteAlertChannel` · `TestAlertChannel`

**Status Pages** — `CreateStatusPage` · `ListStatusPages` · `GetStatusPage` · `UpdateStatusPage` · `DeleteStatusPage` · `SetCustomDomain` · `RemoveCustomDomain`

**Incidents** — `CreateIncident` · `ListIncidents` · `GetIncident` · `UpdateIncident` · `DeleteIncident` · `CreateIncidentUpdate` · `ListIncidentUpdates`

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
