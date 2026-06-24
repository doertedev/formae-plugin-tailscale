# Tailscale resource parity audit

This document records the parity review of the formae Tailscale plugin against
the upstream Terraform `tailscale` provider, performed as new resources were
implemented. It is intended as a living reference; update it when fields are
added or behavior changes.

## `TAILSCALE::IAM::AuthKey` vs Terraform `tailscale_tailnet_key`

Status: **parity confirmed**.

| Terraform field | formae field | Notes |
|-----------------|--------------|-------|
| `reusable` | `reusable` | create-only bool |
| `ephemeral` | `ephemeral` | create-only bool |
| `preauthorized` | `preauthorized` | create-only bool; requires ≥1 tag (enforced) |
| `tags` | `tags` | create-only list |
| `expiry` (duration) | `expirySeconds` (int) | duration expressed in seconds |
| `description` | `description` | create-only string |
| `key` | `key` | returned only at create time (API does not re-emit) |
| `id` | `id` | identifier |

Read-only state surfaced additionally by formae: `createdAt`, `expiresAt`,
`invalid`, `userId`. These match fields returned by the Tailscale API.

Edge cases handled:
- Auth keys are immutable after creation; `update` returns `NotUpdatable`
  directing users to change a create-only field to trigger replacement.
- `preauthorized` without tags is rejected at parse time.
- List filters to `keyType == "auth"` so OAuth clients are excluded.

The SDK `Key` struct also exposes `revoked`, `updated`, `audience`, `issuer`,
`subject`, `customClaimRules`, and `scopes`. These are not part of the auth-key
surface (`revoked`/`updated` are read-only metadata; the remainder belong to
OAuth client and federated identity keys, which have dedicated resources).

## Tailscale API behaviors discovered via live/conformance runs

These behaviors are not obvious from the SDK types and were confirmed against the
real API while running live/conformance/E2E tests:

- **Service names must be `svc:`-prefixed.** The API rejects names that do not
  start with `svc:` (`400 ... is not a valid service name: must start with
  'svc:'`). Fixtures, examples, and the cleanup sweep account for this.
- **A service must carry exactly two addresses.** Create auto-allocates them;
  update rejects an empty `addrs`. The handler preserves the allocated addresses
  on update when the caller does not specify them, and reads back canonical state
  after every write.
- **Deleted keys linger in `Get` but vanish from `List`.** After a key (auth key,
  OAuth client, or federated identity) is deleted, `Keys().List` drops it
  immediately but `Keys().Get` keeps returning it for a while with
  `invalid=true` and a non-zero `Revoked` timestamp. Because formae detects
  out-of-band removal via `Read`/`Get`, the key read handlers treat a revoked key
  as `NotFound` (distinguished from a merely expired key, which is `Invalid` with
  a zero `Revoked` time).

## Singleton resource semantics

ACL policy, tailnet settings, contacts, and the DNS resources are tailnet-scoped
singletons: there is exactly one of each per tailnet. They use a fixed native id
(`"tailnet"`) and implement upsert-style create/update:

- **ACL**: delete resets the policy to the permissive Tailscale default.
- **Tailnet settings / contacts**: delete is a no-op success (the underlying
  object always exists; destroy relinquishes management).
- **DNS resources**: delete clears the managed setting (e.g. empties nameservers,
  disables MagicDNS) so the tailnet returns to an unmanaged baseline.

### Contacts update semantics

Contacts can only be **set or overwritten, never cleared**: an empty email in
the properties is treated as "leave unchanged". This mirrors the upstream API,
which interprets a missing email as no-op rather than delete (and rejects
clearing the account contact outright). To change a contact, supply the new
email; to relinquish management, destroy the resource (delete is a no-op
success).

## Delete semantics

Every resource documents what `delete` does so reconcile/cleanup behavior is
predictable. There are three categories:

| Resource | Delete behavior |
|----------|-----------------|
| `Policy::ACL` | Reset to the permissive Tailscale default ACL. |
| `Tailnet::Settings`, `Tailnet::Contacts` | No-op success (always-present singleton; destroy relinquishes management). |
| `DNS::Configuration`, `DNS::Nameservers`, `DNS::SearchPaths`, `DNS::SplitNameservers` | Reset to empty (unmanaged baseline). |
| `DNS::Preferences` | Reset MagicDNS to disabled. |
| `Device::Authorization`, `Device::Key` | No-op success (baseline depends on tailnet-wide settings). |
| `Device::SubnetRoutes`, `Device::Tags` | Clear the managed routes/tags. |
| `Network::Service` | Delete the service. |
| `Posture::Integration` | Delete the integration. |
| `IAM::FederatedIdentity`, `IAM::AuthKey`, `Events::Webhook` | Delete the underlying object. |
| `Logging::LogstreamConfiguration` | Delete the logstream configuration for the log type. |
| `Logging::AWSExternalID` | No-op success (external IDs are not deletable). |
| `Device::Device`, `IAM::User` | Unsupported — returns `InvalidRequest` (read-only inventory). |

## Error mapping

API failures are mapped through `mapTailscaleError` to a stable
`OperationErrorCode`:

- `401` → `InvalidCredentials`; `403` → `AccessDenied`; `404` → `NotFound`
- `409` → `ResourceConflict`; `429` → `Throttling`
- `400`/`422` → `InvalidRequest`; `5xx`/timeouts → `ServiceInternalError`/`ServiceTimeout`

Validation failures (malformed properties, missing required create-only fields)
return `InvalidRequest`. Immutable updates (auth key fields, service rename,
AWS external ID) return `NotUpdatable`. Write operations on read-only resources
(`Device`, `User`) return `InvalidRequest` via the shared `notSupported` helper.

## Device resources

Devices join the tailnet naturally and cannot be created via the API, so the
device aspect resources (`Authorization`, `Key`, `SubnetRoutes`, `Tags`) adopt an
existing device by `deviceId`. Destroy reverses only what was managed: subnet
routes and tags are cleared, while authorization and key-expiry state are left
untouched (their baseline depends on tailnet-wide settings). `Device` itself is a
read-only inventory resource whose write operations return `InvalidRequest`.

## Test coverage matrix

| Resource | Unit | Live smoke | Conformance fixture |
|----------|:----:|:----------:|:-------------------:|
| AuthKey, OAuthClient, Webhook | yes | yes | yes |
| ACL, TailnetSettings, Contacts, DNS::* | yes | manual | n/a (singleton) |
| Device::* , Device | yes | manual | n/a (device-bound / read-only) |
| Service | yes | yes | yes |
| PostureIntegration, FederatedIdentity, LogstreamConfiguration, AWSExternalID | yes | manual (needs provider/IdP creds) | n/a (external creds) |
| User | yes | manual | n/a (read-only) |

Resources marked "n/a" for conformance do not fit the create→delete→NotFound
harness shape; they are validated via unit tests and targeted live runs against a
dedicated test tailnet.
