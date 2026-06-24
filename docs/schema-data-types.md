# Schema Data Type Examples

This page is intentionally repetitive. It is for editing Pkl without relying
on syntax highlighting or editor completion. Copy a block, change the import
and names, then run `make verify-schema` and `pkl eval` from the relevant Pkl
project root.

The examples use this repo's conventions:

- Resource schemas live under `schema/pkl/<category>/<type>.pkl`.
- Conformance fixtures live under `testdata/<type>.pkl`.
- Example forma files live under `examples/`.
- Resource schemas import `@formae/formae.pkl` and `../tailscale.pkl`.
- Fixtures amend `@formae/forma.pkl`, import one Tailscale resource module,
  and reuse `testdata/config/vars.pkl`.
- Field hints use `@tailscale.FieldHint`.
- Resource hints use `@tailscale.ResourceHint`.

## Complete Schema File Skeleton

Use this when adding a new resource schema under `schema/pkl/<category>/`.

```pkl
// SPDX-License-Identifier: Apache-2.0

module tailscale.category.example

import "@formae/formae.pkl"
import "../tailscale.pkl"

const type = "TAILSCALE::Category::Example"

open class ExampleResolvable extends formae.Resolvable {
    hidden type = module.type

    hidden id: ExampleResolvable = (this) { property = "id" }
}

@tailscale.ResourceHint {
    type = module.type
    identifier = "id"
    portable = true
}
open class Example extends formae.Resource {
    @tailscale.FieldHint { required = true }
    name: String

    @tailscale.FieldHint { hasProviderDefault = true }
    id: String?

    local parent = this

    hidden res: ExampleResolvable = new {
        label = parent.label
        stack = parent.stack?.label
    }
}
```

## Complete Fixture File Skeleton

Use this when adding `testdata/<type>.pkl` for a resource that fits generic
conformance create/read/update/delete behavior.

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
    name = "formae-test-\(v.testRunID)"
  }
}
```

## String

Schema field, required string:

```pkl
@tailscale.FieldHint { required = true }
endpointUrl: String
```

Schema field, optional string:

```pkl
@tailscale.FieldHint { hasProviderDefault = true }
description: String?
```

Schema field, create-only string:

```pkl
@tailscale.FieldHint { required = true; createOnly = true }
deviceId: String
```

Fixture value, literal:

```pkl
description = "formae-test-\(v.testRunID)-oauth-client"
```

Fixture value, environment fallback:

```pkl
tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
```

## Int

Schema field, optional integer:

```pkl
@tailscale.FieldHint { hasProviderDefault = true }
devicesKeyDurationDays: Int?
```

Fixture value:

```pkl
expirySeconds = 3600
```

Target config timeout:

```pkl
apiTimeoutSeconds = 45
```

## Boolean

Schema field, provider-defaulted boolean:

```pkl
@tailscale.FieldHint { hasProviderDefault = true }
networkFlowLoggingOn: Boolean?
```

Fixture value:

```pkl
preauthorized = false
```

Nested preferences:

```pkl
preferences = new {
  overrideLocalDNS = true
  magicDNS = true
}
```

## Enum Strings

Pkl represents this plugin's enums as string literal unions.

Webhook provider:

```pkl
providerType: (""|"slack"|"mattermost"|"googlechat"|"discord") = ""
```

Log type:

```pkl
logType: ("configuration"|"network")?
```

Log destination:

```pkl
destinationType: ("splunk"|"elastic"|"panther"|"cribl"|"datadog"|"axiom"|"s3"|"gcs")?
```

Posture provider:

```pkl
provider: ("falcon"|"fleet"|"huntress"|"intune"|"jamfpro"|"kandji"|"kolide"|"sentinelone")
```

Tailnet external-tailnet join role:

```pkl
usersRoleAllowedToJoinExternalTailnets: ("none"|"admin"|"member")?
```

## Listing of Strings

Schema field:

```pkl
@tailscale.FieldHint { required = true }
subscriptions: Listing<String>
```

Fixture value:

```pkl
subscriptions = new Listing {
  "nodeCreated"
  "nodeDeleted"
}
```

OAuth scopes:

```pkl
scopes = new Listing {
  "auth_keys:read"
}
```

Device tags:

```pkl
tags = new Listing {
  "tag:server"
  "tag:formae"
}
```

Subnet routes:

```pkl
routes = new Listing {
  "10.0.0.0/24"
  "fd7a:115c:a1e0::/48"
}
```

## Mapping

Federated identity custom claim rules:

```pkl
customClaimRules = new Mapping {
  ["email"] = "alice@example.com"
  ["team"] = "platform"
}
```

Service annotations:

```pkl
annotations = new Mapping {
  ["owner"] = "platform"
  ["env"] = "test"
}
```

Split DNS:

```pkl
splitDNS = new Mapping {
  ["corp.example.com"] = new Listing {
    "100.100.100.100"
  }
}
```

## Nested Classes

DNS full-configuration resolver:

```pkl
nameservers = new Listing {
  new {
    address = "1.1.1.1"
    useWithExitNode = true
  }
}
```

DNS preferences:

```pkl
preferences = new {
  overrideLocalDNS = true
  magicDNS = true
}
```

Logstream S3 role configuration:

```pkl
new logstream_configuration.LogstreamConfiguration {
  label = "network-logstream"
  logType = "network"
  destinationType = "s3"
  s3Bucket = "tailscale-logs"
  s3Region = "eu-central-1"
  s3AuthenticationType = "rolearn"
  s3RoleArn = "arn:aws:iam::123456789012:role/tailscale-logstream"
  s3ExternalId = someExternalId.res.externalId
}
```

## Provider-Default Fields

Use `hasProviderDefault = true` for values returned or normalized by
Tailscale, including IDs, timestamps, observed state, and API-populated
defaults.

```pkl
@tailscale.FieldHint { hasProviderDefault = true }
id: String?

@tailscale.FieldHint { hasProviderDefault = true }
createdAt: String?

@tailscale.FieldHint { hasProviderDefault = true }
tags: Listing<String>?
```

Handlers should return canonical read state for provider-default fields. For
set-like arrays, sort the values before read-back.

## Create-Only Fields

Use `createOnly = true` for immutable fields:

```pkl
@tailscale.FieldHint { required = true; createOnly = true }
name: String
```

Examples:

- auth key `reusable`, `ephemeral`, `tags`, `preauthorized`,
  `expirySeconds`, and `description`;
- device aspect `deviceId`;
- service `name`;
- webhook `endpointUrl` and `providerType`;
- posture integration `provider`;
- logstream `logType`.

## Write-Only Fields

Use `writeOnly = true` for inputs accepted by the API but never returned.

```pkl
@tailscale.FieldHint { writeOnly = true; hasProviderDefault = true }
clientSecret: String?
```

Current durable write-only fields:

- `Posture::Integration.clientSecret`
- `Logging::LogstreamConfiguration.token`
- `Logging::LogstreamConfiguration.s3SecretAccessKey`
- `Logging::LogstreamConfiguration.gcsCredentials`

One-time secrets such as auth key material, OAuth client secrets, federated
identity key material, and webhook signing secrets are emitted only on create
and omitted from read state.

## Resolvables

Every resource schema declares a small resolvable class so other resources can
reference provider outputs.

```pkl
open class OAuthClientResolvable extends formae.Resolvable {
    hidden type = module.type

    hidden id: OAuthClientResolvable = (this) { property = "id" }
    hidden key: OAuthClientResolvable = (this) { property = "key" }
}
```

Resource class tail:

```pkl
local parent = this

hidden res: OAuthClientResolvable = new {
    label = parent.label
    stack = parent.stack?.label
}
```

Fixture usage:

```pkl
someField = otherResource.res.id
```

## Target Config

API key target:

```pkl
new formae.Target {
  label = "tailscale"
  config = new tailscale.Config {
    apiKey = read?("env:TAILSCALE_API_KEY")
    tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
  }
}
```

OAuth target:

```pkl
new formae.Target {
  label = "tailscale"
  config = new tailscale.Config {
    oauthClientID = read("env:TAILSCALE_OAUTH_CLIENT_ID")
    oauthClientSecret = read("env:TAILSCALE_OAUTH_CLIENT_SECRET")
    oauthScopes = new Listing { "auth_keys"; "webhooks" }
    tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
    apiTimeoutSeconds = 45
  }
}
```

## Singleton Resource Fragment

Tailnet settings:

```pkl
new settings.TailnetSettings {
  label = "tailnet-settings"
  devicesAutoUpdatesOn = true
  networkFlowLoggingOn = true
}
```

ACL:

```pkl
new acl.ACL {
  label = "tailnet-acl"
  policy = """
  {
    "acls": [
      {"action": "accept", "src": ["*"], "dst": ["*:*"]}
    ]
  }
  """
}
```

DNS nameservers:

```pkl
new nameservers.DNSNameservers {
  label = "dns-nameservers"
  nameservers = new Listing {
    "1.1.1.1"
    "8.8.8.8"
  }
}
```

## Device Resource Fragment

Device tags:

```pkl
new tags.DeviceTags {
  label = "device-tags"
  deviceId = read("env:TAILSCALE_TEST_DEVICE_ID")
  tags = new Listing {
    "tag:formae"
  }
}
```

Device subnet routes:

```pkl
new subnet_routes.DeviceSubnetRoutes {
  label = "device-routes"
  deviceId = read("env:TAILSCALE_TEST_DEVICE_ID")
  routes = new Listing {
    "10.10.0.0/24"
  }
}
```

## Managed Resource Fragment

Auth key:

```pkl
new auth_key.AuthKey {
  label = "auth-key-\(v.testRunID)"
  reusable = true
  ephemeral = false
  preauthorized = false
  expirySeconds = 3600
  description = "formae-test-\(v.testRunID)-auth-key"
}
```

OAuth client:

```pkl
new oauth_client.OAuthClient {
  label = "oauth-client-\(v.testRunID)"
  description = "formae-test-\(v.testRunID)-oauth-client"
  scopes = new Listing {
    "auth_keys:read"
  }
}
```

Webhook:

```pkl
new webhook.Webhook {
  label = "webhook-\(v.testRunID)"
  endpointUrl = "https://example.com/formae-test-\(v.testRunID)"
  providerType = "slack"
  subscriptions = new Listing {
    "nodeCreated"
  }
}
```

Service:

```pkl
new service.Service {
  label = "service-\(v.testRunID)"
  name = "svc:formae-test-\(v.testRunID)-service"
  comment = "formae-test-\(v.testRunID)-service"
  ports = new Listing {
    "443"
  }
}
```
