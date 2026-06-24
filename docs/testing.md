# Testing

The plugin has five test tiers. They are deliberately separate: each has a
different cost, a different scope, and different safety gates. Only unit and
schema checks are safe to run without Tailscale credentials.

| Tier | Target | Hits Tailscale API? | Build tag | Scope |
|------|--------|---------------------|-----------|-------|
| Unit | `make test` | No | none | Handler logic against fake clients for every resource family. |
| Schema | `make verify-schema` | No | none | Pkl schema validation across all 22 resource types. |
| Live smoke | `make test-live-tailscale` | Yes | `integration` | Direct plugin-method lifecycle checks for cheap live resources. |
| Conformance | `make conformance-test` | Yes | `conformance` | Official formae conformance harness through the installed plugin binary. |
| Local E2E | `scripts/e2e-local.sh` | Yes | n/a | Temporary forma project apply, inventory, destroy, and cleanup. |

The live and conformance suites are mutually exclusive by build tag. Passing
one does not compile or execute the other.

> Never use production Tailscale credentials or a production tailnet for any
> tier that hits the real API. Use a dedicated test tailnet or test-scoped
> credentials.

## Credentials

Create `.env` from `.env-sample` or export variables directly:

```bash
cp .env-sample .env
```

Required for live, conformance, cleanup, and E2E:

- `TAILSCALE_API_KEY`, or
- `TAILSCALE_OAUTH_CLIENT_ID` plus `TAILSCALE_OAUTH_CLIENT_SECRET`.

Optional:

- `TAILSCALE_TAILNET`, defaults to `-`;
- `TAILSCALE_BASE_URL`, for API endpoint overrides;
- `TAILSCALE_API_TIMEOUT_SECONDS`, for per-call timeout fallback;
- `TAILSCALE_PLUGIN_DEBUG=1`, for sanitized handler/API call trace logs.

The Make targets source `.env` automatically before checking credentials.

## 1. Unit Tests

```bash
make test
```

This runs `go test -v ./...`. Unit tests live in `pkg/*_test.go`, use the
fake API implementations in `pkg/plugin_test.go`, and never touch the network.

The unit suite covers:

- property parsing and required-field validation;
- API request mapping;
- read-back normalization;
- error-code mapping;
- create-only and read-only behavior;
- singleton semantics;
- secret omission from reads;
- per-target API timeout behavior.

Useful direct commands:

```bash
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test -race ./pkg
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go vet ./...
```

## 2. Schema Checks

```bash
make verify-schema
```

This runs the formae plugin schema verifier:

```bash
go run github.com/platform-engineering-labs/formae/pkg/plugin/testutil/cmd/verify-schema --namespace TAILSCALE ./schema/pkl
```

It validates duplicate files, duplicate resource types, namespace consistency,
and general schema shape across all 22 resource modules.

Pkl evaluation must be run from a directory containing the relevant
`PklProject`:

```bash
cd schema/pkl && pkl eval tailscale.pkl
cd examples/basic && pkl eval main.pkl
```

Running `pkl eval schema/pkl/tailscale.pkl` from the repository root fails
because there is no Pkl project at the root.

## 3. Live Tailscale Smoke Tests

```bash
make test-live-tailscale
make test-integration        # alias
```

These are direct live smoke tests. They call plugin methods in Go with real
Tailscale credentials, but they do not go through the formae agent or the
installed plugin binary.

The Make target runs:

```bash
TAILSCALE_INTEGRATION=1 TAILSCALE_PLUGIN_DEBUG="${TAILSCALE_PLUGIN_DEBUG:-1}" \
  go test -v -tags=integration -run '^TestIntegration_' -count=1 -timeout "${TIMEOUT:-10m}" ./...
```

### Safety Gates

1. `TestMain` skips the suite unless `TAILSCALE_INTEGRATION=1`.
2. The Make target refuses to run unless API-key auth or OAuth credentials are
   configured.
3. `TestMain` sweeps test resources before and after the suite.
4. Every test resource name/description/URL uses the `formae-test-` prefix.
5. Test cleanup is best-effort but the suite still fails loudly on lifecycle
   errors.

### Current Live Scope

The live smoke suite covers resources that are cheap and generic enough to
create with ordinary Tailscale test credentials:

- auth key lifecycle;
- OAuth client lifecycle;
- webhook lifecycle;
- service lifecycle;
- cleanup sweeps for auth keys, OAuth clients, federated identities,
  webhooks, services, and posture integrations matching the test prefix.

Singleton resources, device-bound resources, posture integrations, federated
identity, logstream configuration, and AWS external IDs are covered by unit
tests and targeted manual/live runs when the right external prerequisites are
available.

## 4. Conformance Tests

```bash
make conformance-test
make conformance-test-crud
make conformance-test-discovery
make conformance-test-crud TEST=auth-key
make conformance-test-discovery TEST=service
```

Conformance drives the real plugin binary end-to-end through the official
formae plugin conformance harness. The targets depend on `make install`, so
they exercise the installed binary and schema in the formae discovery layout.

| Function | `-run` filter | What it exercises |
|----------|---------------|-------------------|
| `TestPluginConformance` | `^TestPluginConformance$` | CRUD lifecycle for conformance fixtures. |
| `TestPluginDiscovery` | `^TestPluginDiscovery$` | Discovery/inventory behavior for conformance fixtures. |

`TEST=` is forwarded as `FORMAE_TEST_FILTER` and can select one fixture or a
subset supported by the harness.

### Conformance Scope

The generic harness expects resources that fit a create/read/update/delete
shape. Fixtures exist for:

- `auth-key`;
- `oauth-client` plus update variant;
- `webhook` plus update variant;
- `service` plus update variant.

Resources intentionally outside generic conformance:

- **Singletons**: ACL, tailnet settings, contacts, and DNS resources are
  always-present tailnet-wide objects.
- **Device resources**: require an existing joined device and cannot create
  one.
- **Read-only inventory**: device and user resources do not support writes.
- **External-credential resources**: posture integration, federated identity,
  logstream configuration, and AWS external ID require additional external
  provider/IdP context or have non-standard read behavior.

### Cleanup

Conformance targets run cleanup before and after each CRUD/discovery run:

```bash
make clean-environment
make conformance-cleanup
```

The cleanup script:

- sources `.env`;
- skips cleanly when credentials are absent;
- sets `TAILSCALE_CLEANUP_PREFIXES` from itself, `TEST_PREFIX`, or
  `formae-test-`;
- runs `go run ./scripts/ci/tailscale-cleanup`.

The cleanup implementation deletes only resources whose identifying fields match the
configured prefix:

- auth keys, OAuth clients, and federated identities by description;
- webhooks by endpoint URL;
- services by name or comment;
- posture integrations by cloud ID or client ID.

Tailnet-wide singletons are intentionally not swept because deleting them would
mutate the whole tailnet.

## 5. Local E2E

```bash
scripts/e2e-local.sh
```

The local E2E script creates a temporary forma project under `/private/tmp`,
copies the local schema, installs the plugin, evaluates the forma file,
simulates apply, applies live resources, checks inventory, destroys resources,
and sweeps leftovers.

It creates only the cheap resource set:

- auth key;
- OAuth client;
- webhook;
- service.

Useful overrides:

| Var | Default | Meaning |
|-----|---------|---------|
| `WORK_DIR` | temporary directory | Reuse or inspect a specific workspace. |
| `RUN_ID` | current Unix timestamp | Stable test suffix. |
| `FORMAE_VERSION` | manifest `minFormaeVersion` | Forma schema dependency version. |
| `TAILSCALE_PLUGIN_DEBUG` | `1` during apply | Plugin trace logging. |

The script installs a cleanup trap before applying resources. If the script is
interrupted, it attempts `formae destroy` and then runs the cleanup sweep.

## Build Tags

| File(s) | Build tag | Meaning |
|---------|-----------|---------|
| `pkg/*_test.go` | none | Mock-only unit tests; always compiled. |
| `pkg/integration_test.go` | `//go:build integration && !conformance` | Direct live Tailscale smoke tests. |
| `conformance_test.go` | `//go:build conformance && !integration` | formae conformance harness. |

The `&& !<other>` clauses prevent the live and conformance suites from being
compiled together if both tags are passed.

## Useful Verification Matrix

Before sending a review or release candidate:

```bash
mkdir -p .tmp/go-build .tmp/gotmp
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test -tags=integration ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test -race ./pkg
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go vet ./...
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp make build
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp make verify-schema
GOCACHE=$PWD/.tmp/go-build GOTMPDIR=$PWD/.tmp/gotmp go test -tags=conformance -run '^$' ./...
```

The `integration` command above compiles and then skips unless
`TAILSCALE_INTEGRATION=1`, so it is useful as a no-credential build-tag check.

Remove local caches afterwards if desired:

```bash
rm -rf .tmp
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `tailscale credentials not configured` | No target credentials and no env fallback. | Set `apiKey` or OAuth credentials in the target config or `.env`. |
| `timeout waiting for plugin TAILSCALE to register` | Plugin is not installed in the versioned discovery layout. | Run `make install` and restart the formae agent. |
| `Cannot import dependency because there is no project found` | `pkl eval` was run outside a Pkl project root. | Run from `schema/pkl`, `testdata`, or `examples/basic`. |
| Live test exits 0 with "skipping" | `TAILSCALE_INTEGRATION` is not set. | Use `make test-live-tailscale`. |
| Conformance hits stale resources | Prior run was interrupted or cleanup prefix was different. | Run `TAILSCALE_CLEANUP_PREFIXES=formae-test- make clean-environment`. |
| Large tailnet list calls time out | Default API timeout is too low. | Set `apiTimeoutSeconds` in the target config or `TAILSCALE_API_TIMEOUT_SECONDS`. |
