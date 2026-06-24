# Packaging & Publishing

This repo includes an [Opkgfile](../Opkgfile) for building an orbital package
that can be published as a formae plugin. The repository itself currently
provides source-build and local-install Make targets; package building is
expected to happen in the centralized plugin build pipeline referenced by the
Opkgfile.

## Current Local Packaging Surface

Local development uses:

```bash
make build
make install
```

`make install` stages the plugin in the formae agent's versioned discovery
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

There are no local `make pkg`, `make dist`, or `make publish` targets in this
repository at the moment. Do not document or rely on those commands unless the
Makefile grows them.

## Opkgfile Metadata

[Opkgfile](../Opkgfile) amends `orbital:/opkg.pkl` and derives package
metadata from [formae-plugin.pkl](../formae-plugin.pkl):

| Field | Source | Notes |
|-------|--------|-------|
| `name` | `plugin.name` | `tailscale` |
| `version` | `plugin.version` | Parsed as semver. |
| `publisher` | `OPS_PUBLISHER` | Injected by the build/publish environment. |
| `originator` | `OPS_ORIGINATOR` | Injected by the build/publish environment. |
| `license` | `plugin.license` | `Apache-2.0`. |
| `summary` | `plugin.summary` | Falls back to plugin name. |
| `description` | `plugin.description` | Falls back to summary. |
| `requirements` | `plugin.minFormaeVersion` | Requires formae `>= minFormaeVersion`. |

The package metadata also declares:

```text
display:kind      = plugin
display:category  = network
plugin:type       = resource
plugin:namespace  = TAILSCALE
```

The namespace is the resource-type prefix (`TAILSCALE::...`). The plugin name
is the on-disk install directory (`tailscale`).

## Expected Package Contents

A publishable package must contain the same versioned discovery layout used by
`make install`. The exact staging directory is pipeline-specific, but the
payload should look like:

```text
plugins/
  tailscale/
    v0.1.0/
      tailscale
      formae-plugin.pkl
      schema/
        Config.pkl
        pkl/
          PklProject
          PklProject.deps.json
          VERSION
          tailscale.pkl
          dns/
          device/
          events/
          iam/
          logging/
          network/
          policy/
          posture/
          tailnet/
```

The binary must be named exactly `tailscale`, matching `name` in
`formae-plugin.pkl`. If the package installs a flat `TAILSCALE/` directory or
omits the `v<semver>` layer, the formae agent will not discover the plugin.

## Centralized Build Pipeline

The Opkgfile header states that the centralized build pipeline in
`formae-actions/.github/workflows/plugin-build.yml` consumes this file when
building and signing the package. That pipeline is expected to:

1. build the Go binary;
2. stage the versioned plugin layout;
3. set `OPS_PUBLISHER` and `OPS_ORIGINATOR`;
4. run orbital package build/sign/publish steps;
5. publish to the appropriate registry channel.

Keep [formae-plugin.pkl](../formae-plugin.pkl) authoritative for user-facing
metadata. The Opkgfile intentionally reads from it so package metadata and
plugin metadata cannot drift.

## Local Validation Before Packaging

Before handing the repo to the package pipeline:

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

Then perform at least one local install:

```bash
make install
```

The install target is the closest local approximation of the package payload.

## Publishing The Pkl Schema Package

`schema/pkl/tailscale.pkl` currently sets `extractable = false` through the
base `ResourceHint`. This keeps `formae extract` from attempting to resolve a
schema package that is not yet published to the formae registry.

Once the schema package is published:

1. update the schema package dependency/publication metadata as required by
   the formae registry process;
2. remove or override `extractable = false`;
3. add/update conformance coverage for extract phases;
4. update [docs/schema.md](schema.md) and [README.md](../README.md).

## Release Checklist

1. Update `version` in [formae-plugin.pkl](../formae-plugin.pkl).
2. Run `make build` so `schema/pkl/VERSION` matches the plugin version.
3. Run the local validation matrix above.
4. Run live/conformance tests against a dedicated test tailnet.
5. Confirm [README.md](../README.md), [docs/schema.md](schema.md), and
   [docs/parity-audit.md](parity-audit.md) match the shipped behavior.
6. Hand off to the centralized orbital package pipeline.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Package installs but plugin never registers | Versioned layout is missing or binary name does not match plugin name. | Mirror the `make install` layout exactly. |
| Package metadata has wrong namespace/category | `formae-plugin.pkl` changed without Opkgfile validation. | Check `plugin.namespace` and `plugin.category`; Opkgfile derives from them. |
| `OPS_PUBLISHER` or `OPS_ORIGINATOR` missing | Packaging was run outside the expected pipeline environment. | Set the variables or use the centralized build pipeline. |
| `formae extract` fails resolving schema | Schema package is not published. | Keep `extractable = false` until publishing is complete. |
