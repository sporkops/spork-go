# OpenAPI specifications

This directory holds the formal API contracts for the Sporkops product
surface. Today it contains the Worksuite spec; monitoring and status-page
specs will move in here as those products are retrofitted against the
[cross-product conventions](../docs/api-conventions.md).

## Files

| File | Purpose |
|---|---|
| [`worksuite.yaml`](./worksuite.yaml) | Worksuite (Mail, Calendar, Storage) HTTP API. OpenAPI 3.0.3. |
| [`.spectral.yaml`](./.spectral.yaml) | Project-specific Spectral ruleset that enforces the conventions doc. |
| [`redocly.yaml`](./redocly.yaml) | Redocly config (turns off rules we enforce more precisely via Spectral). |

## Validate the spec

```bash
# Schema validity (fast, strict)
npx @redocly/cli@latest lint openapi/worksuite.yaml --config openapi/redocly.yaml

# Project conventions (snake_case, org-scoping, merge-semantics PATCH, etc.)
npx @stoplight/spectral-cli@latest lint \
  --ruleset openapi/.spectral.yaml \
  openapi/worksuite.yaml
```

CI should fail on any **error** from either tool. **Warnings** from
Spectral are advisory — they currently fire on intentionally polymorphic
ID fields (`parent_id` accepts `root` or `fol_…`, `actor_id` is user
UID or `key_…`). They are flagged for reviewer attention rather than
auto-rejected.

## Render the docs

```bash
npx @redocly/cli@latest build-docs openapi/worksuite.yaml -o ./worksuite.html
```

## Generate a Go client

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen --package worksuite --generate types,client openapi/worksuite.yaml > worksuite.gen.go
```

The spec is intentionally OpenAPI 3.0.3 rather than 3.1, strictly for Go
generator compatibility — `oapi-codegen` and `ogen` do not yet handle
3.1's `type: [..., "null"]` nullable form. When that lands upstream,
migrate the spec with a mechanical `nullable: true` → `type: ["…","null"]`
pass.

## Spec versioning

`info.version` is the spec's own SemVer; bump it on each release:

- **Patch** — clarifications, schema example/description tweaks, new
  optional fields.
- **Minor** — new endpoints, new optional response fields, new error
  codes.
- **Major** — breaking changes (renamed fields, removed endpoints,
  required-field additions). Major bumps should be exceptionally rare;
  prefer additive evolution.

The HTTP path version (`/v1`) is **separate** from `info.version` and
only bumps on a wire-incompatible redesign. The Sporkops contract is
that `v1` is permanent — we evolve additively.

## Cross-product alignment

Every field, header, and envelope in `worksuite.yaml` is chosen to match
existing monitoring and status-page conventions implemented in the
spork-go SDK (see `client.go`, `pagination.go`, `errors.go`,
`webhooks.go`). The conventions are documented once in
[`../docs/api-conventions.md`](../docs/api-conventions.md); deviations
in this spec are a bug.
