// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

// Resource type identifiers for resources added beyond the initial auth key,
// OAuth client, and webhook handlers. Centralized here so the manifest label
// configuration and handler registrations share a single source of truth.

const (
	// Wave 1: core tailnet management.
	ACLResourceType             = "TAILSCALE::Policy::ACL"
	TailnetSettingsResourceType = "TAILSCALE::Tailnet::Settings"
	ContactsResourceType        = "TAILSCALE::Tailnet::Contacts"

	// Wave 2: DNS resources.
	DNSConfigurationResourceType    = "TAILSCALE::DNS::Configuration"
	DNSNameserversResourceType      = "TAILSCALE::DNS::Nameservers"
	DNSPreferencesResourceType      = "TAILSCALE::DNS::Preferences"
	DNSSearchPathsResourceType      = "TAILSCALE::DNS::SearchPaths"
	DNSSplitNameserversResourceType = "TAILSCALE::DNS::SplitNameservers"

	// Wave 3: device operations.
	DeviceAuthorizationResourceType = "TAILSCALE::Device::Authorization"
	DeviceKeyResourceType           = "TAILSCALE::Device::Key"
	DeviceSubnetRoutesResourceType  = "TAILSCALE::Device::SubnetRoutes"
	DeviceTagsResourceType          = "TAILSCALE::Device::Tags"
	DeviceResourceType              = "TAILSCALE::Device::Device"

	// Wave 4: services, posture, and identity.
	ServiceResourceType                = "TAILSCALE::Network::Service"
	PostureIntegrationResourceType     = "TAILSCALE::Posture::Integration"
	FederatedIdentityResourceType      = "TAILSCALE::IAM::FederatedIdentity"
	LogstreamConfigurationResourceType = "TAILSCALE::Logging::LogstreamConfiguration"
	AWSExternalIDResourceType          = "TAILSCALE::Logging::AWSExternalID"

	// Wave 5: read-only/query features.
	UserResourceType = "TAILSCALE::IAM::User"
)
