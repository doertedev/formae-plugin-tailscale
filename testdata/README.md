# testdata/

This directory holds conformance fixtures consumed by
[conformance_test.go](../conformance_test.go) when built with the
`conformance` tag.

Each base fixture declares one cheapest-possible Tailscale resource that fits
the generic formae conformance lifecycle. Variant fixtures are discovered by
filename convention and let the harness exercise in-place updates.

## What Lives Here

```text
testdata/
  PklProject
  PklProject.deps.json
  config/
    vars.pkl
  auth-key.pkl
  oauth-client.pkl
  oauth-client-update.pkl
  webhook.pkl
  webhook-update.pkl
  service.pkl
  service-update.pkl
```

The shared config in `config/vars.pkl` defines:

- a test stack;
- the Tailscale target;
- the run ID used in labels/names/descriptions;
- credential reads from environment variables.

## What Does Not Live Here

- These are not inputs to the direct live smoke tests. The live tests in
  `pkg/integration_test.go` build their request properties inline in Go.
- These are not golden/canned responses for unit tests. Unit tests use fake
  clients and inline JSON properties.
- These are not exhaustive examples for every resource type. Singleton,
  device-bound, read-only, and external-credential resources do not fit the
  generic conformance lifecycle and are tested elsewhere.

## Fixture Contract

Base fixture:

```text
<name>.pkl
```

Update variant:

```text
<name>-update.pkl
```

The update variant must:

- keep the same resource label;
- keep create-only fields unchanged;
- change only mutable fields;
- keep the same target and stack;
- use the same `v.testRunID` interpolation pattern.

The harness should observe the same native ID after an update variant.

Replacement variants:

```text
<name>-replace.pkl
```

There are currently no replacement variants in this repo. Add one only when a
resource has a cheap create-only field change that can safely force
replacement in a live Tailscale test tailnet.

## Current Fixtures

| Fixture | Resource type | Variants | Notes |
|---------|---------------|----------|-------|
| `auth-key.pkl` | `TAILSCALE::IAM::AuthKey` | none | Auth keys are immutable; update returns `NotUpdatable`, so there is no update variant. |
| `oauth-client.pkl` | `TAILSCALE::IAM::OAuthClient` | `oauth-client-update.pkl` | Updates description/scopes/tags via `Keys.SetOAuthClient`. |
| `webhook.pkl` | `TAILSCALE::Events::Webhook` | `webhook-update.pkl` | Updates subscriptions only; endpoint URL/provider type are create-only. |
| `service.pkl` | `TAILSCALE::Network::Service` | `service-update.pkl` | Updates comment/ports while preserving API-allocated addresses. |

## Why Most Resource Types Are Excluded

The generic conformance harness is optimized for resources with independent
create/read/update/delete behavior. Many Tailscale resources deliberately do
not have that shape:

- ACL, tailnet settings, contacts, and DNS are tailnet-wide singletons.
- Device aspect resources require an existing joined device.
- Device and user resources are read-only inventory.
- Posture integrations usually require real third-party provider credentials.
- Federated identities require IdP trust configuration.
- Logstream configuration needs a real destination such as Splunk, S3, or GCS.
- AWS external IDs expose a create-or-get API rather than arbitrary read/delete.

Those resources are covered by unit tests and targeted live/manual checks. See
[docs/testing.md](../docs/testing.md) and
[docs/parity-audit.md](../docs/parity-audit.md).

## Adding A Fixture

1. Add `<name>.pkl` with one resource declaration.
2. Use `import "./config/vars.pkl" as v`.
3. Include `v.stack` and `v.target` inside `forma`.
4. Use labels and provider names/descriptions prefixed with
   `formae-test-\(v.testRunID)` where possible.
5. Add `<name>-update.pkl` only if the resource supports a cheap in-place
   update.
6. Update [docs/testing.md](../docs/testing.md) and
   [docs/schema.md](../docs/schema.md).

Example skeleton:

```pkl
// SPDX-License-Identifier: Apache-2.0

amends "@formae/forma.pkl"

import "@tailscale/category/example.pkl"
import "./config/vars.pkl" as v

forma {
  v.stack
  v.target

  new example.Example {
    label = "example-\(v.testRunID)"
    name = "formae-test-\(v.testRunID)-example"
  }
}
```

## Running Fixture Conformance

```bash
make conformance-test-crud TEST=auth-key
make conformance-test-crud TEST=oauth-client
make conformance-test-crud TEST=webhook
make conformance-test-crud TEST=service
make conformance-test-discovery TEST=service
```

Conformance targets install the plugin before running and call cleanup before
and after the test command.
