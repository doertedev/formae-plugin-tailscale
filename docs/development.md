# Development

This is the developer guide for working on `formae-plugin-tailscale` locally.
There is no assumption of CI for day-to-day work; the Makefile and scripts in
this repository are the canonical entry points.

## Guide Map

- [schema.md](schema.md) covers schema layout, resource hints, field hints,
  singleton semantics, and extractability.
- [schema-data-types.md](schema-data-types.md) is the copy-paste reference for
  Pkl data shapes used by this plugin: scalars, enums, mappings, listings,
  nested classes, provider-default fields, write-only fields, resolvables, and
  resource fragments.
- [testing.md](testing.md) covers unit tests, schema checks, live Tailscale
  smoke tests, conformance runs, cleanup, build tags, and local E2E.
- [parity-audit.md](parity-audit.md) records lifecycle decisions and API
  behaviors discovered during implementation.
- [packaging.md](packaging.md) covers the orbital package metadata included in
  this repo and how package publishing is expected to happen.

## Prerequisites

| Tool | Version | Why |
|------|---------|-----|
| Go | `1.26.0`+ (see `go.mod`) | Build the plugin binary and run tests. |
| `pkl` CLI | project-compatible; `.tool-versions` currently pins it | Read `formae-plugin.pkl`, evaluate schema/examples, and resolve Pkl dependencies. |
| formae CLI | optional for unit work; required for local E2E/conformance | Runs `formae apply`, inventory, destroy, and the agent-backed conformance harness. |
| `golangci-lint` | optional | Required only for `make lint`. |
| Tailscale credentials | optional for unit work; required for live/conformance/E2E | Use a dedicated test tailnet or test-scoped credentials. |

## Environment Variables

The plugin resolves credentials and target behavior in `pkg/plugin.go`:

1. Target config JSON wins.
2. Legacy snake_case target config fields are accepted.
3. Environment variables are used as fallback.

For local development:

```bash
cp .env-sample .env
# edit .env
```

| Var | Used by | Notes |
|-----|---------|-------|
| `TAILSCALE_API_KEY` | plugin runtime, tests, cleanup | API-key auth. Mutually sufficient with OAuth credentials. |
| `TAILSCALE_OAUTH_CLIENT_ID` | plugin runtime, tests, cleanup | OAuth auth path. Requires `TAILSCALE_OAUTH_CLIENT_SECRET`. |
| `TAILSCALE_OAUTH_CLIENT_SECRET` | plugin runtime, tests, cleanup | OAuth secret. Never commit it. |
| `TAILSCALE_TAILNET` | plugin runtime, tests, cleanup | Optional; defaults to `-`. |
| `TAILSCALE_BASE_URL` | plugin runtime | Optional override for tests or mock endpoints. |
| `TAILSCALE_API_TIMEOUT_SECONDS` | plugin runtime | Optional per-call timeout when target config omits `apiTimeoutSeconds`. |
| `TAILSCALE_PLUGIN_DEBUG` | plugin runtime, live/conformance targets | Enables sanitized operation/API-call trace logs. |
| `FORMAE_PLUGIN_DEBUG` | plugin runtime | Also enables plugin tracing. |
| `TAILSCALE_CLEANUP_PREFIXES` | cleanup script | Comma-separated prefixes swept by `make clean-environment`. |
| `TEST_PREFIX` | cleanup script/E2E | Convenience single-prefix override used by `scripts/e2e-local.sh`. |
| `TAILSCALE_INTEGRATION` | live smoke tests | Opt-in gate set by `make test-live-tailscale`. |

`.env` is gitignored. Do not commit credentials or generated local test
outputs.

## Build & Install Layout

```bash
make build
make install
```

`make build` compiles `./bin/tailscale` from `./pkg`, writes
`schema/pkl/VERSION`, and ensures `minFormaeVersion` in
`formae-plugin.pkl` is never below the plugin SDK minimum.

`make install` stages the plugin into the formae agent's versioned discovery
layout:

```text
~/.pel/formae/plugins/tailscale/v0.1.0/
  tailscale
  formae-plugin.pkl
  schema/
    Config.pkl
    pkl/
      PklProject
      PklProject.deps.json
      VERSION
      tailscale.pkl
      ...
```

The formae agent discovers plugins by walking
`<plugins-dir>/<name>/v<semver>/<name>`. A flat or namespace-only layout is
not enough. `make install` removes any existing installed versions for this
plugin name before copying the new binary, manifest, and schema.

## Common Make Targets

Run `make help` for the authoritative list.

| Target | What it does |
|--------|--------------|
| `make build` | Compile `./bin/tailscale` and update schema version metadata. |
| `make install` | Build and install into `~/.pel/formae/plugins/tailscale/v<version>/`. |
| `make test` | Run mock-only `go test -v ./...`. Safe; no real API. |
| `make test-unit` | Runs `go test -v -tags=unit ./...`; currently equivalent in scope to unit tests. |
| `make test-live-tailscale` | Direct live Tailscale smoke tests. Real API. |
| `make test-integration` | Alias for `make test-live-tailscale`. |
| `make verify-schema` | Runs the formae schema verifier over `schema/pkl`. |
| `make lint` | Runs `golangci-lint run`. |
| `make clean` | Removes `bin/` and `dist/`. |
| `make clean-environment` | Sweeps Tailscale test resources by prefix. |
| `make conformance-test-crud` | Runs CRUD conformance through the installed plugin binary. |
| `make conformance-test-discovery` | Runs discovery conformance through the installed plugin binary. |
| `make conformance-test` | Runs CRUD and discovery conformance. |

## Typical Local Loop

Fast, no real API:

```bash
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go vet ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp make verify-schema
```

After handler or schema changes:

```bash
make build
make install
cd examples/basic
pkl eval main.pkl
formae eval main.pkl
```

Against a dedicated test tailnet:

```bash
make test-live-tailscale
make conformance-test-crud TEST=auth-key
scripts/e2e-local.sh
```

## Adding a Resource Type

A new resource type follows the same shape as the existing handlers.

1. **Resource type constant** - add a `TAILSCALE::Category::Type` constant to
   `pkg/resource_types.go` unless the handler owns the constant locally.
2. **Schema** - add `schema/pkl/<category>/<type>.pkl` declaring a
   `formae.Resource` subclass annotated with `@tailscale.ResourceHint` and
   per-field `@tailscale.FieldHint`s.
3. **Handler** - add `pkg/<type>.go` implementing the local
   `resourceHandler` interface and self-registering with
   `register(ResourceType, handler{})` in `init()`.
4. **Client seam** - add the smallest API interface needed to `pkg/plugin.go`
   and implement it on `productionClient`. This keeps unit tests fakeable.
5. **Unit tests** - add `pkg/<type>_test.go` using the fake API in
   `pkg/plugin_test.go`.
6. **Live/conformance coverage** - add direct live tests only when the
   resource is cheap and testable with generic credentials. Add conformance
   fixtures only when the resource fits create/read/update/delete semantics.
7. **Docs** - update [README.md](../README.md), [schema.md](schema.md),
   [parity-audit.md](parity-audit.md), and [testing.md](testing.md).

## Handler Conventions

- Return structured `ProgressResult` failures instead of Go errors for
  provider/user failures. Reserve Go errors for unexpected plugin failures.
- Use `apiContext(ctx)` around every Tailscale API call.
- Map provider errors through `mapTailscaleError`.
- Omit one-time and write-only secrets from reads and update results.
- Normalize set-like arrays with `sortedStrings` before read-back.
- Treat revoked Tailscale keys as `NotFound`.
- Keep singleton native IDs stable as `"tailnet"`.
- For full-replace Tailscale endpoints, read current state first and merge
  omitted sections so partial resources do not erase unrelated tailnet state.

## Local Artifacts

Common generated artifacts are ignored by `.gitignore`:

- `bin/`
- `dist/`
- `.tmp/`
- `.pkl-cache/`
- `.env`

Remove build outputs with:

```bash
make clean
rm -rf .tmp
```
