# formae-plugin-tailscale

A [formae](https://github.com/platform-engineering-labs/formae) resource
plugin for managing [Tailscale](https://tailscale.com/) objects as formae
resources.

Status: `v0.1.0`. Targets formae `>= 0.86.1` (the
`minFormaeVersion` declared in [formae-plugin.pkl](formae-plugin.pkl)).

## Who Needs This Plugin

Stock formae already has Tailscale available as a network option for
operations/security networking. Start there if you only need formae itself to
use Tailscale for connectivity:
[Tailscale (experimental)](https://docs.formae.io/en/latest/operations/security-networking/#tailscale-experimental).

Use this plugin when you want formae to manage **Tailscale account and tailnet
configuration as infrastructure resources**: auth keys, OAuth clients,
federated identities, ACLs, tailnet settings, contacts, DNS configuration,
device settings, services, posture integrations, log streaming, webhooks, and
read-only inventory. In other words, the stock formae networking option is for
formae connectivity; this plugin is for Tailscale resource lifecycle and
inventory management.

## What This Is

A single Go binary that implements the formae resource-plugin contract for
Tailscale under the `TAILSCALE` namespace. The formae agent discovers the
installed binary from `~/.pel/formae/plugins/` and dispatches create, read,
update, delete, list, inventory, sync, and discovery operations to it. The
plugin translates those operations into Tailscale API calls via
`tailscale.com/client/tailscale/v2`.

The schema package in [schema/pkl](schema/pkl/) defines the desired-state
surface users import in their forma files. The handlers in [pkg](pkg/) own the
runtime behavior for each `TAILSCALE::...` resource type.

## Supported Resources

22 resource types across IAM, tailnet policy/settings, DNS, devices, services,
posture, logging, and events.

### IAM & Identity

| Resource Type | Lifecycle | Description |
|---------------|-----------|-------------|
| `TAILSCALE::IAM::AuthKey` | create, read, delete, list | Creates and revokes Tailscale pre-authentication keys. Auth keys are immutable after creation. |
| `TAILSCALE::IAM::OAuthClient` | full CRUD, list | Creates, updates, and deletes Tailscale OAuth clients. |
| `TAILSCALE::IAM::FederatedIdentity` | full CRUD, list | Creates, updates, and deletes federated identity keys backed by an IdP trust. |
| `TAILSCALE::IAM::User` | read-only inventory | Lists and reads tailnet users provisioned by identity systems. |

### Tailnet Management

| Resource Type | Lifecycle | Description |
|---------------|-----------|-------------|
| `TAILSCALE::Policy::ACL` | singleton upsert | Manages the tailnet ACL policy as HuJSON/JSON; validates before applying and resets to the default policy on destroy. |
| `TAILSCALE::Tailnet::Settings` | singleton upsert | Manages tailnet-wide settings such as device approval, key duration, posture collection, regional routing, and HTTPS. |
| `TAILSCALE::Tailnet::Contacts` | singleton upsert | Manages account, support, and security contact emails. |

### DNS

| Resource Type | Lifecycle | Description |
|---------------|-----------|-------------|
| `TAILSCALE::DNS::Configuration` | singleton upsert | Complete tailnet DNS configuration using the alpha endpoint. Partial desired state is merged over live state before write. |
| `TAILSCALE::DNS::Nameservers` | singleton upsert | Global DNS nameserver list. |
| `TAILSCALE::DNS::Preferences` | singleton upsert | MagicDNS toggle. |
| `TAILSCALE::DNS::SearchPaths` | singleton upsert | DNS search paths. |
| `TAILSCALE::DNS::SplitNameservers` | singleton upsert | Split DNS domain-to-nameservers map. |

### Devices

| Resource Type | Lifecycle | Description |
|---------------|-----------|-------------|
| `TAILSCALE::Device::Authorization` | adopt/update | Approves or revokes an existing device's authorization. |
| `TAILSCALE::Device::Key` | adopt/update | Manages key-expiry behavior for an existing device. |
| `TAILSCALE::Device::SubnetRoutes` | adopt/update | Approves enabled subnet routes advertised by an existing device. |
| `TAILSCALE::Device::Tags` | adopt/update | Manages tags on an existing device. |
| `TAILSCALE::Device::Device` | read-only inventory | Lists and reads devices that joined the tailnet naturally. |

### Network, Posture, Logging & Events

| Resource Type | Lifecycle | Description |
|---------------|-----------|-------------|
| `TAILSCALE::Network::Service` | full CRUD, list | Manages Tailscale virtual IP services. |
| `TAILSCALE::Posture::Integration` | full CRUD, list | Manages device posture integrations such as Falcon, Fleet, Kolide, and Intune. |
| `TAILSCALE::Logging::LogstreamConfiguration` | singleton per log type | Manages configuration and network log streaming endpoints. |
| `TAILSCALE::Logging::AWSExternalID` | create/read oriented | Resolves the AWS external ID used by S3 RoleARN log streaming. |
| `TAILSCALE::Events::Webhook` | create, subscription update, delete, list | Creates webhooks, updates subscriptions, and deletes webhook endpoints. |

## Prerequisites

- Go `1.26.0` or newer (see [go.mod](go.mod)).
- The `pkl` CLI. The Makefile reads [formae-plugin.pkl](formae-plugin.pkl)
  with `pkl eval`; schema and examples also require Pkl project resolution.
- Tailscale API credentials for anything that hits the real API.
- Optional: `golangci-lint` for `make lint`.
- Optional: formae CLI for local E2E and conformance workflows.

## Local Setup

```bash
cp .env-sample .env
# edit .env with a dedicated test tailnet or test-scoped credentials
```

`.env` is gitignored. The live, conformance, cleanup, and local E2E targets
source it automatically. Do not use a production tailnet for test runs.

## Configuration & Credential Behavior

Targets use `tailscale.Config` from `@tailscale/tailscale.pkl`.

```pkl
new formae.Target {
  label = "tailscale"
  config = new tailscale.Config {
    apiKey = read?("env:TAILSCALE_API_KEY")
    tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
  }
}
```

OAuth client credentials are also supported:

```pkl
new formae.Target {
  label = "tailscale"
  config = new tailscale.Config {
    oauthClientID = read("env:TAILSCALE_OAUTH_CLIENT_ID")
    oauthClientSecret = read("env:TAILSCALE_OAUTH_CLIENT_SECRET")
    oauthScopes = new Listing { "auth_keys"; "webhooks" }
    tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
  }
}
```

Credential precedence in [pkg/plugin.go](pkg/plugin.go):

1. Target config JSON wins: `apiKey`, or `oauthClientID` plus
   `oauthClientSecret`.
2. Legacy snake_case target fields are accepted for compatibility.
3. Environment variables are used as fallback:
   `TAILSCALE_API_KEY`, `TAILSCALE_OAUTH_CLIENT_ID`,
   `TAILSCALE_OAUTH_CLIENT_SECRET`, `TAILSCALE_TAILNET`,
   `TAILSCALE_BASE_URL`, and `TAILSCALE_API_TIMEOUT_SECONDS`.

`tailnet` defaults to `"-"`, which lets Tailscale resolve the default tailnet
for the credential. `apiTimeoutSeconds` defaults to 15 seconds and controls
both the per-request context deadline and the underlying HTTP client timeout.

## Build & Install

```bash
make build
make install
```

`make install` writes the versioned formae plugin-discovery layout:

```text
~/.pel/formae/plugins/tailscale/v0.1.0/
  tailscale
  formae-plugin.pkl
  schema/
    Config.pkl
    pkl/
```

The binary name, install directory, schema version, and manifest version are
derived from [formae-plugin.pkl](formae-plugin.pkl). The install target removes
previous installed versions of this plugin name first, so local installs are a
clean slate.

## Testing

Full details are in [docs/testing.md](docs/testing.md).

```bash
make test
make verify-schema
make test-live-tailscale
make conformance-test-crud TEST=auth-key
make conformance-test-discovery TEST=service
make conformance-test
scripts/e2e-local.sh
```

| Tier | Hits real API? | Build tag | Scope |
|------|----------------|-----------|-------|
| Unit | No | none | Handler logic against fake clients for every resource family. |
| Schema | No | none | Pkl schema validation across all 22 resource types. |
| Live smoke | Yes | `integration` | Direct plugin method lifecycle checks for cheap live resources. |
| Conformance | Yes | `conformance` | Official formae plugin harness through the installed plugin binary. |
| Local E2E | Yes | n/a | Temporary forma project apply, inventory, destroy, and cleanup. |

The live and conformance suites use mutually exclusive build tags:
`integration && !conformance` and `conformance && !integration`.

## Schema

The Pkl schema lives under [schema/pkl](schema/pkl/) and defines:

- target configuration (`tailscale.Config`);
- resource modules by category;
- field hints (`required`, `createOnly`, `writeOnly`,
  `hasProviderDefault`);
- resource hints (`type`, `identifier`, `portable`, `extractable`).

See:

- [docs/schema.md](docs/schema.md) for schema layout, resource identifiers,
  field-hint conventions, and extractability.
- [docs/schema-data-types.md](docs/schema-data-types.md) for copy-pasteable
  Pkl examples for every data shape used by this plugin.

## Lifecycle Caveats

- **Singleton resources use a fixed native id.** ACL, tailnet settings,
  contacts, and DNS resources represent exactly one tailnet-wide object and
  use native id `"tailnet"`.
- **Deletes can mutate tailnet-wide settings.** ACL delete resets the ACL to
  the permissive Tailscale default. DNS deletes clear or disable the managed
  DNS section. Settings and contacts deletes are no-op successes because the
  backing objects are always present.
- **Devices are adopted, not created.** Device aspect resources require an
  existing `deviceId`. Destroy clears only managed routes/tags; authorization
  and key-expiry destroy are no-op successes because their baseline depends on
  tailnet-wide settings.
- **Secrets are one-time or write-only.** Auth key material, OAuth client
  secrets, federated identity key material, webhook secrets, posture client
  secrets, and logstream credentials are never returned by read/list handlers.
- **Tailscale deleted keys can linger in `Get`.** Key handlers treat a key with
  a non-zero revoked timestamp as `NotFound` so drift detection sees deleted
  keys as gone.
- **Discovery surfaces configured singleton resources.** Singleton `List`
  handlers report `"tailnet"` because the backing object always exists.

See [docs/parity-audit.md](docs/parity-audit.md) for the full lifecycle and
delete-semantics matrix.

## Examples

The basic example lives in [examples/basic/main.pkl](examples/basic/main.pkl).

```bash
cd examples/basic
pkl eval main.pkl
formae eval main.pkl
formae apply --mode reconcile --watch main.pkl
```

For a self-contained live workflow that copies the local schema, installs the
plugin, applies temporary resources, checks inventory, destroys them, and
sweeps leftovers:

```bash
scripts/e2e-local.sh
```

## Packaging

This repo includes an [Opkgfile](Opkgfile) for the centralized plugin build
pipeline. Local source installs are handled by `make install`; orbital package
publishing is documented in [docs/packaging.md](docs/packaging.md).

## Documentation

- [docs/development.md](docs/development.md) - local workflow,
  prerequisites, install layout, env vars, and adding resources.
- [docs/testing.md](docs/testing.md) - unit, schema, live smoke,
  conformance, cleanup, build tags, and local E2E.
- [docs/schema.md](docs/schema.md) - Pkl schema layout, resource categories,
  field/resource hints, singleton semantics, and extractability.
- [docs/schema-data-types.md](docs/schema-data-types.md) - copy-pasteable Pkl
  snippets for schema and fixture authors.
- [docs/parity-audit.md](docs/parity-audit.md) - Tailscale API behavior,
  Terraform-provider parity notes, lifecycle semantics, and test matrix.
- [docs/packaging.md](docs/packaging.md) - orbital package metadata and
  publishing notes.

## License

Apache-2.0. See [LICENSE](LICENSE).
