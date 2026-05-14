# API roadmap — known gaps not yet shipped

This is the working list of design gaps the [conventions doc](./api-conventions.md)
doesn't yet cover and the [Worksuite spec](../openapi/worksuite.yaml) doesn't
yet expose. Every item is a deliberate decision to defer, not a forgotten
detail — they're captured here so the team doesn't re-derive them.

When a roadmap item ships, move its summary into a numbered section in
`api-conventions.md` and delete the entry here.

---

## A. JMAP parity for Worksuite Mail

Worksuite Mail is positioned as a forward-looking modern mailbox — we
deliberately do **not** ship IMAP/POP3/SMTP-AUTH for end clients. End
users connect via the web UI or a modern client that speaks JMAP
([RFC 8620](https://www.rfc-editor.org/rfc/rfc8620) /
[RFC 8621](https://www.rfc-editor.org/rfc/rfc8621)). The REST surface
in `openapi/worksuite.yaml` is for **app/server integrations**; the
JMAP surface is for **mail clients**.

That split means our REST API can be smaller than Gmail's REST API
(clients don't go through it), but it also means anything a modern
mail client needs has to exist on the JMAP side or it doesn't exist
at all. The gaps below are the JMAP capabilities a real Apple
Mail / Thunderbird-Cobalt / mobile-client install will exercise and
where our current model is silent.

| Capability | Status | Notes |
|---|---|---|
| `Email/get`, `Email/set`, `Email/query` | ✅ Modelled (REST equivalents) | `GET /mail/messages/{id}`, `PATCH …`, `GET /mail/threads?query=…`. JMAP endpoint not yet documented. |
| `Email/changes` (delta sync) | ❌ Missing | Mail clients **must** sync offline. Without a change feed they fall back to full re-sync, which is unusable on mobile. Spec proposal: `GET /mail/changes?since=<state>` returning created / updated / destroyed IDs + a `next_state` token. JMAP `state` is the canonical analogue. |
| `Thread/changes` | ❌ Missing | Same shape as Email/changes but for the thread index. |
| `EmailSubmission` (send as a first-class resource) | ⚠ Partial | We have `POST /mail/messages` (send) and `POST /mail/drafts/{id}/send`. JMAP separates the **submission** (delivery attempt, status, error) from the message. Without that, clients can't show "sending… / sent / bounced" reliably. Spec proposal: introduce a `MailSubmission` resource with status `pending`/`sent`/`failed` and reference it from webhooks. |
| `VacationResponse` (auto-reply / out-of-office) | ❌ Missing | Mailboxes need an auto-reply feature for OoO. Spec proposal: `GET / PATCH /mail/vacation-response` per user. |
| `MDN` — message disposition notifications (read receipts) | ❌ Missing | Both sending an MDN request and acknowledging one. Optional but expected. |
| `Identity` (sending identities + aliases) | ⚠ Partial | `GET /mail/addresses` exists but is read-only. Need create/update with display name, signature, default identity per send. |
| Push notifications (JMAP push, RFC 8620 §7) | ⚠ Indirect | We have webhooks for `mail.received`. JMAP clients expect a per-session event-source they subscribe to. Either bridge JMAP push → webhooks, or implement the JMAP push spec when we ship the JMAP endpoint. |
| Sieve filters / server-side rules | ❌ Missing | Auto-archive, auto-forward, auto-label. RFC 5228 is the canonical model. Spec proposal: `GET / POST /mail/filters` returning Sieve scripts or a structured rule DSL. |
| Quotas surfaced per-user | ⚠ Partial | Aggregate org usage is at `GET /usage`. JMAP `urn:ietf:params:jmap:quota` exposes per-mailbox quotas; we should mirror that for end-user UIs. |
| Mail search beyond `query=` | ⚠ Partial | Current `query` is Gmail-style operators. JMAP `Email/query` filter+sort+collapseThreads is richer; clients will want a structured filter object eventually. |
| PGP / S/MIME hooks | ❌ Missing | Required for any vertical that handles regulated comms. Probably v2 — defer. |
| MX / SPF / DKIM / DMARC / Autodiscover management | ❌ Missing | Customers expect us to vend MX records and DKIM keys for their custom domains. Track separately under product domain-onboarding, not in this OpenAPI. |

Decision principle: anything a modern JMAP client needs to be
first-class on Worksuite gets a JMAP endpoint **and** (where it makes
sense for server-to-server consumers) a REST mirror with the same data
model. We do not retro-fit IMAP.

---

## B. Cross-cutting capabilities deferred from v1

### B1. Per-operation security scopes

The `Authorization: Bearer` security scheme is declared globally
(`worksuite.yaml`, top-level `security`). Individual operations do not
declare which scope they require — the scope grammar
(`<product>:<resource>:<verb>`) is documented but not machine-readable.

**Proposal:** attach `security: [{ bearerAuth: [worksuite:mail:send] }]`
to every operation. ~58 edits, mechanical once a scope→operation map
is agreed.

**Why deferred:** the mapping is a product decision (e.g. is "list
drafts" `worksuite:mail:read` or `worksuite:mail:drafts:read`?). Do
this when the dashboard's scope-picker UI is being designed; the two
should agree.

### B2. Conditional requests — `ETag` / `If-Match`

For collaborative resources (`CalendarEvent`, `StorageFile`,
`MailDraft`, `MailThread`), concurrent edits silently last-write-wins
today. CalDAV requires `If-Match`; Google Drive, Microsoft Graph, and
Stripe all support it.

**Proposal:**

1. Add `etag: { type: string, readOnly: true }` to each mutable
   resource schema. Emit a weak ETag header on `GET` and `200` PATCH
   responses.
2. Accept `If-Match: <etag>` on `PATCH` / `DELETE`. Mismatch → `412
   Precondition Failed` (new response component required).
3. Conventions doc gets a §20 "Optimistic concurrency".

### B3. Batch / bulk operations

Listing pages at `limit: 100` makes 10k-item migrations a 100-request
ordeal. Mail clients especially feel this on initial sync.

**Proposal:** a `POST /v1/batch` envelope that takes an array of
sub-requests and returns an array of sub-responses, à la Microsoft
Graph `$batch` and JMAP method chaining. Within-batch ordering
guarantees and partial-failure semantics need careful spec. Defer until
we have a concrete user pulled this off and asked for it — premature
batching APIs are common API-design footguns.

### B4. Delta / change feed

Separate from B3 — a sync primitive (`GET /v1/orgs/{org_id}/changes`)
that returns "what changed since cursor X" across selected resource
types. Required for offline-capable Worksuite clients (Apple
Mail-class). Tied to JMAP `Email/changes` in section A.

### B5. Member management endpoints

**Status:** mostly shipped — invite + role-change + remove are in the
spec. The agency model is a `Member.is_agency` boolean rather than a
separate role; `MemberRole` enumerates `[owner, admin, member]`.

**Still outstanding:**

- **Owner transfer.** `POST /v1/orgs/{org_id}/owner-transfer` is
  reserved in the spec but returns `501 not_implemented`. Implement
  the atomic two-sided transfer (demote current owner + promote
  target in one transaction; require `confirm: true`) when the
  product flow is designed.
- **Self-removal.** `DELETE /v1/orgs/{org_id}/members/me` is
  intentionally not exposed yet. Add when we decide whether
  last-owner self-removal blocks or auto-transfers (it should block,
  but the UX flow needs to lead the user to the transfer endpoint
  once that ships).

### B6. Data residency

EU customers will ask "where is my Worksuite data stored?". Today
`GET /v1/regions` exists for monitoring only (conventions §3).

**Proposal:**

- `GET /v1/regions` returns `[{ id: 'us-east', display_name: …, products: ['worksuite', 'monitoring', …] }, …]`.
- `Organization.data_region: { type: string, enum: [...regions], readOnly: true }` — chosen at org creation, immutable thereafter.
- Storage and mail data physically pinned to the org's region; webhook delivery and the dashboard remain global.

### B7. DSAR / GDPR data export + purge

Required for EU + California compliance. Endpoints (proposed):

- `POST /v1/orgs/{org_id}/users/{user_id}/data-export` → returns a job
  ID; webhook fires with a signed download URL when ready.
- `POST /v1/orgs/{org_id}/users/{user_id}/data-purge` → schedules
  irreversible deletion of the user's mail, calendar, storage; audit
  trail retention is governed by separate policy.
- Both require `confirmation_required` flow (conventions §10).

### B8. Customer-Managed Encryption Keys (CMEK / BYOK)

Enterprise / regulated customers expect to bring their own KMS key
(AWS KMS, GCP KMS, Azure Key Vault). Worksuite uses it as a key-
encryption-key wrapping the per-object data keys for stored mail and
files. Revoking the key in the customer's KMS leaves Sporkops unable
to decrypt — the kill-switch property is the whole point.

**Proposal for the spec today (cheap reservation):**

- Mark every endpoint that *will* be CMEK-sensitive at GA with
  `x-sporkops-encryption: customer-managed-eligible: true`.
- Reserve `Organization.encryption: { provider: 'sporkops'|'aws_kms'|'gcp_kms'|'azure_kv', key_arn: string }`.

**Proposal for v2:**

- `POST /v1/orgs/{org_id}/encryption/configure` to bind a customer
  KMS key.
- `POST /v1/orgs/{org_id}/encryption/rotate` to re-wrap.
- Background re-encryption job exposed via the usage endpoint.

### B9. Storage capabilities

Inherited from the audit; defer to a dedicated Storage roadmap doc
when this list grows:

- Folder-level ACLs (today only file-level).
- Share-link expiry + password protection.
- Comments / activity feed per file.
- Per-org trash retention policy.
- Custom metadata fields (`additionalProperties` slot on
  `StorageFile`).

### B10. Calendar capabilities

- Per-calendar ACL / delegation (today only owner+attendees).
- Free/busy query (`POST /calendar/free-busy`).
- Working hours + default reminder per user.
- Calendar colors (server-stored, not just client-side).
- iCal (`.ics`) import/export.

### B11. Webhook event payload schemas

`WebhookEventName` enumerates the event types a subscription can
filter on (`mail.*`, `calendar.*`, `storage.*`, `member.*`,
`monitor.*`), but the spec defines **no delivery envelope or
per-event payload schema** — a subscriber has an event name to
subscribe to and nothing to code their handler against. This gap
predates the member-management work; the `member.*` additions just
made it more visible.

**Proposal:**

- A `WebhookDelivery` envelope: `{ id, event, created_at,
  organization_id, data }` plus the existing signature headers.
- A discriminated `data` payload per event family. For `member.*`,
  `data` is the relevant `Member` or `MemberInvite` resource; a
  `member.role_changed` payload additionally carries the previous
  role.
- Document delivery semantics: at-least-once, ordering not
  guaranteed, retry/backoff schedule, signature verification.

### B12. Retrofit the spork-go SDK to the formalized schemas

The hand-written SDK structs in `models.go` predate the OpenAPI spec
and now diverge from it. This is **intentional** — per the
`openapi/README.md` plan, the spec is the source of truth and each
product's SDK gets retrofitted against it — but the divergence must
be tracked so it isn't mistaken for a bug. Known gaps as of the
member-management work:

- `Member` still carries `status` + `expires_at` (the spec split
  pre-acceptance state into a separate `MemberInvite`), lacks
  `is_agency`, and its `role` is `owner|member` (the spec adds
  `admin`). `user_id` is `omitempty` but the spec now requires it.
- `InviteMemberInput` lacks `is_agency`; there is no SDK type for
  `MemberInvite`, `MemberInviteCreate`, or the invite list/envelope.
- `AcceptInviteInput{token}` / `AcceptInviteResult{status}` reflect
  the old `POST /members/accept` shape. The spec replaces that with
  `POST /users/me/invites/resolve` (token → `MemberInvite`) and
  `POST /users/me/invites/{invite_id}/accept` (returns the new
  `Member`).
- `TransferOwnershipInput` exists but the spec's owner-transfer
  endpoint is a `501` stub (§B5) — align both when that ships.

Do this retrofit as a dedicated SDK PR (it touches `members.go`,
`models.go`, and their tests); keep it out of spec-only changes.

---

## C. Spec-quality polish (do anytime)

These are < 1-hour fixes the team can pick up in spare cycles:

- Replace remaining unsigned `string` ID fields with patterned
  schemas (Spectral `sporkops-string-ids-have-pattern` is at `warn`
  precisely so these stay visible).
- Add `additionalProperties: false` to closed-shape request bodies
  (response bodies stay open for forward-compatibility).
- Expand the `Error.code` enum's `description` to enumerate every
  HTTP status each code can appear under.
- Replace inline parameter definitions with `$ref`s to
  `components.parameters` wherever the same parameter (e.g. `org_id`)
  appears more than twice.
- Generate the three derived specs (`mail.yaml`, `calendar.yaml`,
  `storage.yaml`) from the monolith via a Redocly bundle script — for
  single-product customers who want a focused doc.

---

## D. What we have explicitly decided **not** to do

So future-us doesn't re-litigate:

- **IMAP / SMTP-AUTH / POP3 endpoints.** End users use a modern JMAP
  client or the web UI. See §A.
- **Resellers / white-label.** Agencies use the member model (see
  [api-conventions §3a](./api-conventions.md#3a-agencies-and-multi-client-access)).
- **A platform key that authenticates across orgs without
  membership.** Same reason.
- **PUT for resource updates.** PATCH only (`sporkops-no-put` Spectral
  rule).
- **Splitting the OpenAPI document into per-product specs as the
  source of truth.** One spec, derived per-product subsets when
  needed.
