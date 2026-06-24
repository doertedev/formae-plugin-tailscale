# Schema

The plugin's desired-state surface is defined in
[Pkl](https://pkl-lang.org/) under `schema/pkl/`. The formae agent evaluates a
user's forma file against this schema, and the Go handlers in `pkg/` act on the
resolved resource properties. Field and resource hint annotations tell formae
how to compare desired and observed state.

## Layout

```text
schema/pkl/
|-- PklProject
|-- PklProject.deps.json
|-- VERSION
|-- tailscale.pkl                   module root: Config, FieldHint, ResourceHint
|-- dns/
|   |-- configuration.pkl           TAILSCALE::DNS::Configuration
|   |-- nameservers.pkl             TAILSCALE::DNS::Nameservers
|   |-- preferences.pkl             TAILSCALE::DNS::Preferences
|   |-- search_paths.pkl            TAILSCALE::DNS::SearchPaths
|   `-- split_nameservers.pkl       TAILSCALE::DNS::SplitNameservers
|-- device/
|   |-- authorization.pkl           TAILSCALE::Device::Authorization
|   |-- device.pkl                  TAILSCALE::Device::Device
|   |-- key.pkl                     TAILSCALE::Device::Key
|   |-- subnet_routes.pkl           TAILSCALE::Device::SubnetRoutes
|   `-- tags.pkl                    TAILSCALE::Device::Tags
|-- events/
|   `-- webhook.pkl                 TAILSCALE::Events::Webhook
|-- iam/
|   |-- auth_key.pkl                TAILSCALE::IAM::AuthKey
|   |-- federated_identity.pkl      TAILSCALE::IAM::FederatedIdentity
|   |-- oauth_client.pkl            TAILSCALE::IAM::OAuthClient
|   `-- user.pkl                    TAILSCALE::IAM::User
|-- logging/
|   |-- aws_external_id.pkl         TAILSCALE::Logging::AWSExternalID
|   `-- logstream_configuration.pkl TAILSCALE::Logging::LogstreamConfiguration
|-- network/
|   `-- service.pkl                 TAILSCALE::Network::Service
|-- policy/
|   `-- acl.pkl                     TAILSCALE::Policy::ACL
|-- posture/
|   `-- integration.pkl             TAILSCALE::Posture::Integration
`-- tailnet/
    |-- contacts.pkl                TAILSCALE::Tailnet::Contacts
    `-- settings.pkl                TAILSCALE::Tailnet::Settings
```

The plugin manifest at the repo root, `formae-plugin.pkl`, declares `name`,
`version`, `namespace` (`TAILSCALE`), `category`, `license`, and
`minFormaeVersion`. The Makefile reads `name`, `version`, and `namespace` from
that manifest via `pkl eval`.

## Module Root (`tailscale.pkl`)

```pkl
module tailscale

import "@formae/formae.pkl"

open class Config {
  fixed type: String = "TAILSCALE"

  @formae.ConfigFieldHint { createOnly = true }
  apiKey: String?

  @formae.ConfigFieldHint { createOnly = true }
  oauthClientID: String?

  @formae.ConfigFieldHint { createOnly = true }
  oauthClientSecret: String?

  @formae.ConfigFieldHint { createOnly = true }
  oauthScopes: Listing<String>?

  @formae.ConfigFieldHint { createOnly = true }
  tailnet: String = "-"

  @formae.ConfigFieldHint { createOnly = true }
  baseUrl: String?

  @formae.ConfigFieldHint { createOnly = true }
  apiTimeoutSeconds: Int?
}

class FieldHint extends formae.FieldHint {}

class ResourceHint extends formae.ResourceHint {
    extractable = false
}
```

- **`Config`** is the target config schema. The plugin also accepts legacy
  snake_case JSON fields and environment fallbacks, but Pkl users should use
  the camelCase fields above.
- **`FieldHint`** extends `formae.FieldHint` so resource schemas can use the
  shorter `@tailscale.FieldHint` annotation.
- **`ResourceHint`** extends `formae.ResourceHint` and pins
  `extractable = false` until the schema package is published.

## Resource Inventory

| Category | Types |
|----------|-------|
| IAM | `TAILSCALE::IAM::AuthKey`, `TAILSCALE::IAM::OAuthClient`, `TAILSCALE::IAM::FederatedIdentity`, `TAILSCALE::IAM::User` |
| Policy | `TAILSCALE::Policy::ACL` |
| Tailnet | `TAILSCALE::Tailnet::Settings`, `TAILSCALE::Tailnet::Contacts` |
| DNS | `TAILSCALE::DNS::Configuration`, `TAILSCALE::DNS::Nameservers`, `TAILSCALE::DNS::Preferences`, `TAILSCALE::DNS::SearchPaths`, `TAILSCALE::DNS::SplitNameservers` |
| Device | `TAILSCALE::Device::Authorization`, `TAILSCALE::Device::Key`, `TAILSCALE::Device::SubnetRoutes`, `TAILSCALE::Device::Tags`, `TAILSCALE::Device::Device` |
| Network | `TAILSCALE::Network::Service` |
| Posture | `TAILSCALE::Posture::Integration` |
| Logging | `TAILSCALE::Logging::LogstreamConfiguration`, `TAILSCALE::Logging::AWSExternalID` |
| Events | `TAILSCALE::Events::Webhook` |

Each resource schema opens a `formae.Resource` subclass annotated with
`@tailscale.ResourceHint` and declares its fields with
`@tailscale.FieldHint`s.

For copy-pasteable examples of each Pkl shape used in this repo, see
[schema-data-types.md](schema-data-types.md).

## Resource Hints

`@tailscale.ResourceHint { ... }` annotates each resource class.

| Attribute | Example | Effect |
|-----------|---------|--------|
| `type` | `"TAILSCALE::IAM::AuthKey"` | Fully-qualified resource type. Must match the handler registration. |
| `identifier` | `"id"`, `"tailnet"`, `"deviceId"` | Stable primary identifier used by formae. |
| `portable` | `true` | The resource can move across stacks/targets when identifiers remain valid. |
| `extractable` | `false` by base default | `formae extract` is disabled until the schema package is published. |
| `discoverable` | inherited default | Discovery is supported when `List` can enumerate useful native IDs. |

Identifier conventions:

| Resource family | Identifier | Notes |
|-----------------|------------|-------|
| Key-like IAM resources | `id` | Tailscale key ID returned by the API. |
| Tailnet singletons | `tailnet` | Fixed native ID `"tailnet"`. |
| DNS singletons | `tailnet` | Fixed native ID `"tailnet"`. |
| Device aspect resources | `deviceId` | Existing device node ID. |
| Device inventory | `nodeId` | Preferred over numeric ID when available. |
| Service | `name` | Service names must be `svc:`-prefixed. |
| Logstream configuration | `logType` | `configuration` or `network`. |
| AWS external ID | `externalId` | External ID returned by Tailscale. |
| Webhook | `endpointId` | Endpoint ID returned by Tailscale. |

## Field Hints

`@tailscale.FieldHint { ... }` annotates individual fields.

| Attribute | Effect |
|-----------|--------|
| `required = true` | The field must be present in desired state. |
| `createOnly = true` | The field is immutable after creation; changes should replace the resource or fail as not updatable. |
| `writeOnly = true` | The field is accepted on write but omitted from reads because the provider never returns it or it is secret material. |
| `hasProviderDefault = true` | The provider may compute or normalize this field; formae should not treat provider-supplied values as unexpected diffs. |

General conventions:

- Required fields are enforced by schema and usually also checked in handlers
  for structured operation errors.
- One-time secrets that the API emits only on create are marked
  `hasProviderDefault`; handlers emit them once in create progress and omit
  them from read/list state.
- Durable write-only credentials, such as posture client secrets and logstream
  tokens, are marked `writeOnly = true`.
- API-computed timestamps, IDs, user IDs, labels, and observed state are
  marked `hasProviderDefault = true`.
- Set-like arrays are sorted by handlers before read-back so repeated reads
  are stable.

## Singleton Semantics

Several Tailscale APIs expose tailnet-wide settings rather than independently
creatable objects. These are modeled as singleton resources with fixed native
ID `"tailnet"`:

- `TAILSCALE::Policy::ACL`
- `TAILSCALE::Tailnet::Settings`
- `TAILSCALE::Tailnet::Contacts`
- `TAILSCALE::DNS::Configuration`
- `TAILSCALE::DNS::Nameservers`
- `TAILSCALE::DNS::Preferences`
- `TAILSCALE::DNS::SearchPaths`
- `TAILSCALE::DNS::SplitNameservers`

Singleton `create` and `update` are upserts. `List` returns the fixed native
ID. Delete behavior varies by backing API:

- ACL delete resets to the permissive Tailscale default policy.
- Tailnet settings and contacts deletes are no-op successes.
- DNS deletes clear or disable the managed DNS section.

See [parity-audit.md](parity-audit.md#delete-semantics) for the full matrix.

## Device Resource Semantics

Devices join a tailnet outside this plugin. The device aspect resources adopt
existing devices by `deviceId`:

- `Device::Authorization`
- `Device::Key`
- `Device::SubnetRoutes`
- `Device::Tags`

`Device::Device` is read-only inventory. The aspect resources can update parts
of an existing device, but they cannot create a device. Delete behavior is
conservative: subnet routes and tags are cleared; authorization and key-expiry
state are left untouched because their baseline depends on tailnet settings.

## Secret and Write-Only Fields

| Resource | Field | Behavior |
|----------|-------|----------|
| `IAM::AuthKey` | `key` | Returned only on create, omitted on read/list. |
| `IAM::OAuthClient` | `key` | OAuth client secret returned only on create, omitted on read/list/update. |
| `IAM::FederatedIdentity` | `key` | Returned only on create, omitted on read/list/update. |
| `Events::Webhook` | `secret` | Signing secret returned only on create, omitted on read/list/update. |
| `Posture::Integration` | `clientSecret` | Write-only, omitted from reads. |
| `Logging::LogstreamConfiguration` | `token`, `s3SecretAccessKey`, `gcsCredentials` | Write-only, omitted from reads and write progress state. |

## Extractability Caveat

`schema/pkl/tailscale.pkl` pins `extractable = false` on the base
`ResourceHint`. The schema package is local to this repo and is not published
to the formae registry yet, so `formae extract` cannot resolve it remotely.

Once the schema package is published, remove the base override or set
`extractable = true` per resource and update the conformance expectations.

## Test Fixtures

`testdata/*.pkl` are conformance harness inputs. They declare cheap live
resources and update variants for the resources that fit the conformance
create/read/update/delete shape:

- `auth-key.pkl`
- `oauth-client.pkl` and `oauth-client-update.pkl`
- `webhook.pkl` and `webhook-update.pkl`
- `service.pkl` and `service-update.pkl`

Singletons, device-bound resources, read-only resources, and resources that
need extra external-provider credentials are covered by unit tests and targeted
live/manual runs rather than the generic conformance fixture set.

See [testdata/README.md](../testdata/README.md) for the fixture contract.

## Validating the Schema Locally

```bash
make verify-schema
cd schema/pkl && pkl eval tailscale.pkl
cd examples/basic && pkl eval main.pkl
```

`pkl eval schema/pkl/tailscale.pkl` from the repository root fails because the
Pkl project manifest is in `schema/pkl`. Evaluate from that project root.
