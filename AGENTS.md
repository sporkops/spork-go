# Agent guide for spork-go

Hand-written Go SDK for the [Spork](https://sporkops.com) uptime-monitoring API. Zero runtime dependencies. Used by the [Spork CLI](https://github.com/sporkops/cli) and the [Terraform provider](https://github.com/sporkops/terraform-provider-sporkops).

If your user wants Go code that talks to Spork, this file tells you how to write it idiomatically. If the user wants to manage monitors interactively or from a non-Go script, **prefer the [Spork CLI](https://github.com/sporkops/cli) with `--json`** over generating bespoke HTTP code.

## Install

```bash
go get github.com/sporkops/spork-go
```

Requires Go 1.24+.

## Construct a client

```go
import spork "github.com/sporkops/spork-go"

client := spork.NewClient(
    spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
)
```

The client is safe for concurrent use. Reuse one client for the lifetime of your program; do not construct a new one per call.

For twelve-factor configs, prefer `WithEnvDefaults` — one option that reads `SPORK_API_KEY`, `SPORK_ORGANIZATION_ID` (or `SPORK_ORG_ID`), and `SPORK_BASE_URL`:

```go
client := spork.NewClient(spork.WithEnvDefaults())
```

Options passed after `WithEnvDefaults` override the env-derived values, so you can pin one knob and inherit the rest.

Optional functional options: `WithOrganization`, `WithBaseURL`, `WithUserAgent`, `WithHTTPClient`, `WithEagerOrgResolve`, `WithRetryPolicy`, `WithLogger`.

## Authentication

`SPORK_API_KEY` is the convention. Generate a key at <https://sporkops.com/settings/api-keys> or via the CLI (`spork apikey create`). Keys are prefixed `sk_`.

Do **not** hard-code keys in source. Do **not** log them. The SDK redacts auth headers from any request/response logging it emits.

## Multi-org

API keys are bound to one organization, so for the common single-tenant case the SDK auto-resolves the org on first use. Firebase callers (humans with multiple memberships) **must** set it explicitly via `WithOrganization`:

```go
client := spork.NewClient(
    spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
    spork.WithOrganization("org_acme"),
)
```

To run calls against several orgs from one client, use `ForOrg` for a per-call clone — concurrent goroutines can target different orgs without racing:

```go
acmeMonitors,    _ := client.ForOrg("org_acme").ListMonitors(ctx)
widgetsMonitors, _ := client.ForOrg("org_widgets").ListMonitors(ctx)
```

`client.SetOrganization(id)` mutates the receiver in place and is **deprecated** (removed in v1.0). Use `ForOrg` for switching or `WithOrganization` at construction.

## Resources covered

| Resource       | Methods                                                                                  |
|----------------|------------------------------------------------------------------------------------------|
| Monitors       | `Create` / `List` / `Get` / `Update` / `Delete` / `GetMonitorResults`                    |
| Alert Channels | `Create` / `List` / `Get` / `Update` / `Delete` / `Test`                                 |
| Status Pages   | `Create` / `List` / `Get` / `Update` / `Delete` / `SetCustomDomain` / `RemoveCustomDomain` |
| Incidents      | `Create` / `List` / `ListRecent` / `Get` / `Update` / `Delete` / `CreateUpdate` / `ListUpdates` |
| API Keys       | `Create` / `List` / `Delete`                                                             |
| Account        | `GetAccount`                                                                             |

All methods take `context.Context` as the first argument. Cancel the context to abort an in-flight request.

## Errors — use the typed predicates

```go
m, err := client.GetMonitor(ctx, id)
switch {
case spork.IsNotFound(err):
    // 404
case spork.IsUnauthorized(err):
    // 401 — bad or expired API key
case spork.IsForbidden(err):
    // 403 — key lacks permission
case spork.IsRateLimited(err):
    // 429 — already retried with backoff and still failed
case err != nil:
    // anything else
}
```

For full structured detail, type-assert to `*spork.APIError`:

```go
var apiErr *spork.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.RequestID)
}
```

Always include `apiErr.RequestID` when reporting issues upstream.

## Idempotency for create operations

Agents re-run things. Set an idempotency key on creates so a retry doesn't produce a duplicate resource:

```go
ctx = spork.WithIdempotencyKey(ctx, "create-monitor-acme-prod")
m, err := client.CreateMonitor(ctx, &spork.Monitor{...})
```

Use a stable, semantically meaningful key per logical operation — not a random UUID per retry.

## Retries

The SDK already retries 429 / 503 / 504 with exponential backoff (up to 3 attempts) and respects `Retry-After`. Do **not** wrap your own retry loop on top of `spork-go` calls; you'll multiply the wait.

## Pagination

`List*` methods return up to a default page size. Iterate until the returned `NextPageToken` is empty. Don't fetch all pages eagerly when you only need the first match.

## Testing

The `sporkopstest` subpackage exposes a mock client suitable for unit tests:

```go
import "github.com/sporkops/spork-go/sporkopstest"

client := sporkopstest.NewMock()
```

Prefer this over hand-rolled HTTP mocks.

## When to use this SDK vs alternatives

- **Building a Go service that owns Spork resources** → use this SDK.
- **One-off script in Go or any other language** → use the [Spork CLI](https://github.com/sporkops/cli) with `--json`. Less code, no compilation, identical underlying API.
- **Declarative infra in HCL** → use the [Spork Terraform provider](https://github.com/sporkops/terraform-provider-sporkops).

## Reporting issues

File bugs at <https://github.com/sporkops/spork-go/issues>. Include the SDK version, the request ID from `*spork.APIError`, and a minimal repro.
