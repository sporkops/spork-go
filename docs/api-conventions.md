# Sporkops API conventions

This document describes the conventions every Sporkops HTTP API follows.
The goal is uniformity across products — a developer fluent in the
monitoring API can read the Worksuite API without surprise, and a single
language SDK can express every product without per-product special cases.

If a Sporkops API contradicts anything below, the API is wrong.

## 1. Base URL and versioning

Every product lives at `https://api.sporkops.com/v1`. The path version
(`/v1`) is permanent — we evolve additively rather than ever cutting a
`/v2`. Spec documents have their own SemVer in `info.version`, separate
from the path version.

## 2. Authentication

`Authorization: Bearer sk_<env>_<token>` — one header, every product.
API keys are prefixed `sk_live_` or `sk_test_`. A key is **bound to a
single organization** at creation time. Callers that need to operate
across multiple organizations mint one key per organization.

Keys carry **scopes** following the grammar `<product>:<resource>:<verb>`:

```
worksuite:mail:send       monitoring:monitors:write
worksuite:files:read      monitoring:alert_channels:admin
worksuite:calendar:admin  status_pages:incidents:write
```

Wildcards: `<product>:admin` grants every verb across the product;
`<product>:<resource>:admin` grants every verb on a resource.

Missing scope → `403 forbidden` with `code: "insufficient_scope"`.

## 3. Organization scoping

Every resource endpoint lives under `/v1/orgs/{org_id}/…`. The path
segment is present even though the bound key already implies the org —
this is deliberate:

1. URLs are portable across keys (and across user-auth sessions).
2. Audit logs and request traces show the org explicitly.
3. The same code path works for API-key callers and for Firebase-auth
   dashboard sessions where one user may belong to multiple orgs.

Mismatch between path `org_id` and the key's bound org → `403 forbidden`
with `code: "org_mismatch"`.

User-scoped exceptions (anything that necessarily transcends one org):

- `GET /v1/users/me`
- `GET /v1/users/me/orgs`
- `GET /v1/regions` (monitoring only)
- `POST /v1/orgs` (create a new org — not callable with an API key)
- `GET /v1/members/invites`
- `POST /v1/members/accept`

### 3a. Agencies and multi-client access

Sporkops does **not** sell through resellers — every organization owns
its own billing relationship and support contract directly with us. But
small agencies routinely manage Worksuite (or monitoring, or status
pages) for several client orgs. The supported pattern:

1. Each client signs up directly and creates their own org.
2. The agency operator is invited as a **member** of each client's org
   via `POST /v1/members/accept` after a `GET /v1/members/invites`.
3. `GET /v1/users/me/orgs` returns every org the agency operator
   belongs to — that endpoint is the canonical "agency dashboard"
   feed.
4. For automated work, the operator mints **one API key per client
   org** they need to script against. Each key is bound to one org
   (§2) so a leak is blast-radius-limited to one client.
5. Audit-trail entries log the agency operator as the `actor` against
   the client's org — every action is attributable to a human, even
   when an agency key was used.

What we deliberately don't ship: a "platform key" that authenticates
across orgs without explicit membership, white-label rebranding, or
billing pass-through. Those would turn the agency into a reseller and
break the direct-customer-relationship invariant.

## 4. Naming

- **JSON field names**: `snake_case` exclusively. Never `camelCase`,
  never `PascalCase`.
- **Query parameters**: `snake_case`.
- **Path parameters**: `snake_case`.
- **Headers**: `Title-Case` HTTP convention (`X-Request-Id`,
  `Idempotency-Key`, `X-Sporkops-Signature`).
- **operationId**: `camelCase`, mirroring the Go method name the
  generator emits (`listMailThreads`, `createCalendarEvent`).
- **Resource path segments**: plural, kebab-cased
  (`/alert-channels`, `/status-pages`, `/maintenance-windows`).

## 5. Identifiers

Every resource uses a string ID with a stable prefix:

| Prefix | Resource |
|---|---|
| `org_` | Organization |
| `mem_` | Member |
| `key_` | API key |
| `whk_` | Webhook subscription |
| `aud_` | Audit event |
| `mon_` | Monitor (monitoring) |
| `ach_` | Alert channel (monitoring) |
| `sp_` | Status page (status pages) |
| `inc_` | Incident (status pages) |
| `mw_` | Maintenance window (monitoring) |
| `msg_` | Mail message (Worksuite) |
| `thr_` | Mail thread (Worksuite) |
| `drf_` | Mail draft (Worksuite) |
| `lbl_` | Mail label (Worksuite) |
| `cal_` | Calendar (Worksuite) |
| `evt_` | Calendar event (Worksuite) |
| `fil_` | Storage file (Worksuite) |
| `fol_` | Storage folder (Worksuite) |
| `ver_` | Storage version (Worksuite) |
| `shr_` | Storage share (Worksuite) |
| `up_`  | Storage upload (Worksuite) |

Polymorphic IDs are explicit: `parent_id` accepts the literal `root` or
a `fol_…`; `calendar_id` accepts the literal `primary` or a `cal_…`.
This is documented in the field's description and enforced by the
endpoint's path-param pattern.

## 6. Timestamps

RFC 3339 UTC strings, e.g. `2026-05-13T08:24:11Z`. Never integer epoch
seconds in JSON bodies. (The webhook signature header is the one
deliberate exception — its `t=` segment is epoch seconds because the
HMAC signs `<timestamp>.<raw_body>`.)

## 7. Update semantics

- **`PATCH`** = **JSON Merge Patch ([RFC 7396](https://www.rfc-editor.org/rfc/rfc7396))**.
  Fields omitted are left unchanged; fields set to `null` are cleared.
  Request bodies use `Content-Type: application/merge-patch+json`
  (the standard `application/json` is also accepted for ergonomics).
  We do **not** support [JSON Patch (RFC 6902)](https://www.rfc-editor.org/rfc/rfc6902)
  operation arrays — pick one model and stick to it.
- **`POST`** = create or trigger an action.
- **`PUT`** = full replacement. **Avoid** for new APIs — the legacy
  monitoring/status-pages PUT/PATCH split was a papercut we are not
  propagating. Use `PATCH` unless full replacement is genuinely
  required, and if it is, prefer modelling the replacement as a `POST`
  to a dedicated sub-resource.

Every `PATCH` operation's description states "merge semantics" verbatim
— enforced by the `sporkops-patch-describes-merge` Spectral rule.

## 8. List responses

Every list endpoint returns:

```json
{
  "data": [/* items */],
  "meta": {
    "has_more": true,
    "next_cursor": "opaque-server-token"
  }
}
```

Query parameters:

- `cursor` — opaque token from a previous response's `meta.next_cursor`.
  Omit for the first page.
- `limit` — 1 to 100, default 100.

Auto-pagination clients pass `next_cursor` back as `cursor` until
`has_more` is false. Cursors are opaque — callers must not parse them.

Even fixed-size lists (e.g. a user's mail addresses) use this envelope
so callers can reuse a single auto-paginator helper across products.

## 9. Single-resource responses

```json
{ "data": { /* the resource */ } }
```

Always wrapped. Never a bare object at the top level.

## 10. Error envelope

```json
{
  "error": {
    "code": "validation_error",
    "message": "Human-readable description.",
    "details": [
      { "field": "name", "message": "must not be empty" }
    ],
    "request_id": "req_01H7…"
  }
}
```

Stable, machine-readable codes (every product reuses these):

| Code | HTTP | Meaning |
|---|---|---|
| `validation_error` | 400 | Input failed schema/business validation; `details` populated. |
| `authentication_required` | 401 | No credentials. |
| `invalid_api_key` | 401 | Credentials parsed but rejected. |
| `insufficient_scope` | 403 | Authed but the key lacks the required scope. |
| `org_mismatch` | 403 | Path `org_id` ≠ key's bound org. |
| `not_found` | 404 | Resource doesn't exist or caller can't see it. |
| `conflict` | 409 | Business-rule conflict (e.g. deleting an org with an active subscription). |
| `rate_limited` | 429 | Honour `Retry-After`. |
| `quota_exceeded` | 402 / 413 | Per-usage cap hit (storage, send rate, etc.). |
| `payment_required` | 402 | Plan doesn't include the feature. |
| `confirmation_required` | 400 | Destructive action needs an explicit `confirm` body field. |
| `invalid_resource_type` | 400 | Operation called on a resource of the wrong type. |
| `upstream_error` | 502 / 503 | Transient infra failure. |
| `internal_error` | 500 | Bug. Include `request_id` when filing a support ticket. |

## 11. Tracing

Every response carries `X-Request-Id`. Clients should surface it in
errors. Support tickets include it.

## 12. Idempotency

`POST`, `PATCH`, and `DELETE` operations accept an optional
`Idempotency-Key: <client-key>` header. The server replays the original
response for any repeat within a 24-hour window. Use stable,
semantically-meaningful keys (`create-monitor-acme-prod`,
`patch-event-acme-q3-offsite`), not random UUIDs per retry.

`PATCH` is included so a retried merge-patch doesn't double-apply when
the body contains computed deltas (counters, append-only arrays).
`GET` is excluded — it's already idempotent by definition.

## 13. Rate limits and retries

Server responses include `Retry-After` on `429`, `503`, and `504`.
Clients **must** retry these with exponential backoff (the spork-go SDK
does 3 attempts, 500ms base, doubling). Other 4xx codes are caller
errors and should not be retried.

## 14. Webhook delivery

Subscriptions are created per organization. Every delivery is signed:

```
X-Sporkops-Signature: t=1712937600,v1=<hex-hmac-sha256>
```

The signed value is `<timestamp>.<raw_body>` (same scheme Stripe
popularised). Multiple `v1=` segments may appear during a secret
rotation; verifiers succeed if any one matches.

Replay window: reject deliveries whose `t=` is more than 5 minutes off
the verifier's clock.

Payload envelope (common across products):

```json
{
  "event": "mail.received",
  "organization_id": "org_…",
  "timestamp": "2026-05-13T08:24:11Z",
  "test": false,
  /* product-specific fields */
}
```

## 15. Per-usage billing

`GET /v1/orgs/{org_id}/usage` is one endpoint, every product. Worksuite
meters sit next to monitoring and status-page meters in the same
envelope. This is what makes the "per usage, not per seat" promise
auditable from a single GET — never one per product.

## 16. Audit trail

`GET /v1/orgs/{org_id}/audit-trail` is one endpoint, every product. The
`product` filter narrows; absence means "everything across the org".

## 17. Spec format

OpenAPI 3.0.3 documents under `openapi/`. The choice of 3.0 over 3.1 is
strictly for Go generator compatibility today; we'll upgrade when
`oapi-codegen` and `ogen` support 3.1's null-union form cleanly.

Every spec ships with:

- a `redocly.yaml` (catches structural errors)
- a `.spectral.yaml` that extends `spectral:oas` and enforces this doc
  via project rules
- a README documenting the lint and codegen flow

## 18. SSO

SSO is on the roadmap. Endpoints that will be SSO-gated carry an
`x-sso-required: true` vendor extension today so when SSO ships, the
enforcement is a configuration toggle rather than a path migration.

## 19. What's not in this doc yet

This document records the conventions a Sporkops API must follow
**today**. Gaps and proposals — JMAP parity for Worksuite Mail,
conditional requests (`ETag` / `If-Match`), batch operations,
per-operation security scopes, data residency, customer-managed
encryption keys, DSAR/GDPR export — live in
[`./roadmap.md`](./roadmap.md). When a roadmap item lands, it moves
into a numbered section here and stops being a footnote.
