// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	ts "tailscale.com/client/tailscale/v2"
)

type fakeAPI struct {
	keys            fakeKeys
	webhooks        fakeWebhooks
	policyFile      fakePolicyFile
	tailnetSettings fakeTailnetSettings
	contacts        fakeContacts
	dns             fakeDNS
	devices         fakeDevices
	services        fakeServices
	logging         fakeLogging
	devicePosture   fakeDevicePosture
	users           fakeUsers
}

func (f fakeAPI) Keys() keysAPI                       { return f.keys }
func (f fakeAPI) Webhooks() webhooksAPI               { return f.webhooks }
func (f fakeAPI) PolicyFile() policyFileAPI           { return f.policyFile }
func (f fakeAPI) TailnetSettings() tailnetSettingsAPI { return f.tailnetSettings }
func (f fakeAPI) Contacts() contactsAPI               { return f.contacts }
func (f fakeAPI) DNS() dnsAPI                         { return f.dns }
func (f fakeAPI) Devices() devicesAPI                 { return f.devices }
func (f fakeAPI) Services() servicesAPI               { return f.services }
func (f fakeAPI) Logging() loggingAPI                 { return f.logging }
func (f fakeAPI) DevicePosture() devicePostureAPI     { return f.devicePosture }
func (f fakeAPI) Users() usersAPI                     { return f.users }

type fakeKeys struct {
	createAuthKey           func(context.Context, ts.CreateKeyRequest) (*ts.Key, error)
	createOAuthClient       func(context.Context, ts.CreateOAuthClientRequest) (*ts.Key, error)
	setOAuthClient          func(context.Context, string, ts.SetOAuthClientRequest) (*ts.Key, error)
	createFederatedIdentity func(context.Context, ts.CreateFederatedIdentityRequest) (*ts.Key, error)
	setFederatedIdentity    func(context.Context, string, ts.SetFederatedIdentityRequest) (*ts.Key, error)
	get                     func(context.Context, string) (*ts.Key, error)
	list                    func(context.Context, bool) ([]ts.Key, error)
	delete                  func(context.Context, string) error
}

func (f fakeKeys) CreateAuthKey(ctx context.Context, req ts.CreateKeyRequest) (*ts.Key, error) {
	if f.createAuthKey == nil {
		return nil, nil
	}
	return f.createAuthKey(ctx, req)
}

func (f fakeKeys) CreateOAuthClient(ctx context.Context, req ts.CreateOAuthClientRequest) (*ts.Key, error) {
	if f.createOAuthClient == nil {
		return nil, nil
	}
	return f.createOAuthClient(ctx, req)
}

func (f fakeKeys) SetOAuthClient(ctx context.Context, id string, req ts.SetOAuthClientRequest) (*ts.Key, error) {
	if f.setOAuthClient == nil {
		return nil, nil
	}
	return f.setOAuthClient(ctx, id, req)
}

func (f fakeKeys) CreateFederatedIdentity(ctx context.Context, req ts.CreateFederatedIdentityRequest) (*ts.Key, error) {
	if f.createFederatedIdentity == nil {
		return nil, nil
	}
	return f.createFederatedIdentity(ctx, req)
}

func (f fakeKeys) SetFederatedIdentity(ctx context.Context, id string, req ts.SetFederatedIdentityRequest) (*ts.Key, error) {
	if f.setFederatedIdentity == nil {
		return nil, nil
	}
	return f.setFederatedIdentity(ctx, id, req)
}

func (f fakeKeys) Get(ctx context.Context, id string) (*ts.Key, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx, id)
}

func (f fakeKeys) List(ctx context.Context, all bool) ([]ts.Key, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, all)
}

func (f fakeKeys) Delete(ctx context.Context, id string) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, id)
}

type fakeWebhooks struct {
	create func(context.Context, ts.CreateWebhookRequest) (*ts.Webhook, error)
	get    func(context.Context, string) (*ts.Webhook, error)
	list   func(context.Context) ([]ts.Webhook, error)
	update func(context.Context, string, []ts.WebhookSubscriptionType) (*ts.Webhook, error)
	delete func(context.Context, string) error
}

func (f fakeWebhooks) Create(ctx context.Context, req ts.CreateWebhookRequest) (*ts.Webhook, error) {
	if f.create == nil {
		return nil, nil
	}
	return f.create(ctx, req)
}

func (f fakeWebhooks) Get(ctx context.Context, id string) (*ts.Webhook, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx, id)
}

func (f fakeWebhooks) List(ctx context.Context) ([]ts.Webhook, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx)
}

func (f fakeWebhooks) Update(ctx context.Context, id string, subs []ts.WebhookSubscriptionType) (*ts.Webhook, error) {
	if f.update == nil {
		return nil, nil
	}
	return f.update(ctx, id, subs)
}

func (f fakeWebhooks) Delete(ctx context.Context, id string) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, id)
}

type fakePolicyFile struct {
	get      func(context.Context) (*ts.ACL, error)
	raw      func(context.Context) (*ts.RawACL, error)
	set      func(context.Context, any, string) error
	validate func(context.Context, any) error
}

func (f fakePolicyFile) Get(ctx context.Context) (*ts.ACL, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx)
}
func (f fakePolicyFile) Raw(ctx context.Context) (*ts.RawACL, error) {
	if f.raw == nil {
		return nil, nil
	}
	return f.raw(ctx)
}
func (f fakePolicyFile) Set(ctx context.Context, acl any, etag string) error {
	if f.set == nil {
		return nil
	}
	return f.set(ctx, acl, etag)
}
func (f fakePolicyFile) Validate(ctx context.Context, acl any) error {
	if f.validate == nil {
		return nil
	}
	return f.validate(ctx, acl)
}

type fakeTailnetSettings struct {
	get    func(context.Context) (*ts.TailnetSettings, error)
	update func(context.Context, ts.UpdateTailnetSettingsRequest) error
}

func (f fakeTailnetSettings) Get(ctx context.Context) (*ts.TailnetSettings, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx)
}
func (f fakeTailnetSettings) Update(ctx context.Context, req ts.UpdateTailnetSettingsRequest) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, req)
}

type fakeContacts struct {
	get    func(context.Context) (*ts.Contacts, error)
	update func(context.Context, ts.ContactType, ts.UpdateContactRequest) error
}

func (f fakeContacts) Get(ctx context.Context) (*ts.Contacts, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx)
}
func (f fakeContacts) Update(ctx context.Context, t ts.ContactType, req ts.UpdateContactRequest) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, t, req)
}

type fakeDNS struct {
	configuration    func(context.Context) (*ts.DNSConfiguration, error)
	setConfiguration func(context.Context, ts.DNSConfiguration) error
	nameservers      func(context.Context) ([]string, error)
	setNameservers   func(context.Context, []string) error
	preferences      func(context.Context) (*ts.DNSPreferences, error)
	setPreferences   func(context.Context, ts.DNSPreferences) error
	searchPaths      func(context.Context) ([]string, error)
	setSearchPaths   func(context.Context, []string) error
	splitDNS         func(context.Context) (ts.SplitDNSResponse, error)
	setSplitDNS      func(context.Context, ts.SplitDNSRequest) error
}

func (f fakeDNS) Configuration(ctx context.Context) (*ts.DNSConfiguration, error) {
	if f.configuration == nil {
		return nil, nil
	}
	return f.configuration(ctx)
}
func (f fakeDNS) SetConfiguration(ctx context.Context, c ts.DNSConfiguration) error {
	if f.setConfiguration == nil {
		return nil
	}
	return f.setConfiguration(ctx, c)
}
func (f fakeDNS) Nameservers(ctx context.Context) ([]string, error) {
	if f.nameservers == nil {
		return nil, nil
	}
	return f.nameservers(ctx)
}
func (f fakeDNS) SetNameservers(ctx context.Context, ns []string) error {
	if f.setNameservers == nil {
		return nil
	}
	return f.setNameservers(ctx, ns)
}
func (f fakeDNS) Preferences(ctx context.Context) (*ts.DNSPreferences, error) {
	if f.preferences == nil {
		return nil, nil
	}
	return f.preferences(ctx)
}
func (f fakeDNS) SetPreferences(ctx context.Context, p ts.DNSPreferences) error {
	if f.setPreferences == nil {
		return nil
	}
	return f.setPreferences(ctx, p)
}
func (f fakeDNS) SearchPaths(ctx context.Context) ([]string, error) {
	if f.searchPaths == nil {
		return nil, nil
	}
	return f.searchPaths(ctx)
}
func (f fakeDNS) SetSearchPaths(ctx context.Context, sp []string) error {
	if f.setSearchPaths == nil {
		return nil
	}
	return f.setSearchPaths(ctx, sp)
}
func (f fakeDNS) SplitDNS(ctx context.Context) (ts.SplitDNSResponse, error) {
	if f.splitDNS == nil {
		return nil, nil
	}
	return f.splitDNS(ctx)
}
func (f fakeDNS) SetSplitDNS(ctx context.Context, r ts.SplitDNSRequest) error {
	if f.setSplitDNS == nil {
		return nil
	}
	return f.setSplitDNS(ctx, r)
}

type fakeDevices struct {
	list             func(context.Context, ...ts.ListDevicesOptions) ([]ts.Device, error)
	get              func(context.Context, string) (*ts.Device, error)
	getWithAllFields func(context.Context, string) (*ts.Device, error)
	setAuthorized    func(context.Context, string, bool) error
	setKey           func(context.Context, string, ts.DeviceKey) error
	setSubnetRoutes  func(context.Context, string, []string) error
	subnetRoutes     func(context.Context, string) (*ts.DeviceRoutes, error)
	setTags          func(context.Context, string, []string) error
	delete           func(context.Context, string) error
}

func (f fakeDevices) List(ctx context.Context, opts ...ts.ListDevicesOptions) ([]ts.Device, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, opts...)
}
func (f fakeDevices) Get(ctx context.Context, id string) (*ts.Device, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx, id)
}
func (f fakeDevices) GetWithAllFields(ctx context.Context, id string) (*ts.Device, error) {
	if f.getWithAllFields == nil {
		return nil, nil
	}
	return f.getWithAllFields(ctx, id)
}
func (f fakeDevices) SetAuthorized(ctx context.Context, id string, a bool) error {
	if f.setAuthorized == nil {
		return nil
	}
	return f.setAuthorized(ctx, id, a)
}
func (f fakeDevices) SetKey(ctx context.Context, id string, k ts.DeviceKey) error {
	if f.setKey == nil {
		return nil
	}
	return f.setKey(ctx, id, k)
}
func (f fakeDevices) SetSubnetRoutes(ctx context.Context, id string, r []string) error {
	if f.setSubnetRoutes == nil {
		return nil
	}
	return f.setSubnetRoutes(ctx, id, r)
}
func (f fakeDevices) SubnetRoutes(ctx context.Context, id string) (*ts.DeviceRoutes, error) {
	if f.subnetRoutes == nil {
		return nil, nil
	}
	return f.subnetRoutes(ctx, id)
}
func (f fakeDevices) SetTags(ctx context.Context, id string, t []string) error {
	if f.setTags == nil {
		return nil
	}
	return f.setTags(ctx, id, t)
}
func (f fakeDevices) Delete(ctx context.Context, id string) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, id)
}

type fakeServices struct {
	list           func(context.Context) ([]ts.Service, error)
	get            func(context.Context, string) (*ts.Service, error)
	createOrUpdate func(context.Context, ts.Service) error
	delete         func(context.Context, string) error
}

func (f fakeServices) List(ctx context.Context) ([]ts.Service, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx)
}
func (f fakeServices) Get(ctx context.Context, name string) (*ts.Service, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx, name)
}
func (f fakeServices) CreateOrUpdate(ctx context.Context, s ts.Service) error {
	if f.createOrUpdate == nil {
		return nil
	}
	return f.createOrUpdate(ctx, s)
}
func (f fakeServices) Delete(ctx context.Context, name string) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, name)
}

type fakeLogging struct {
	logstreamConfiguration       func(context.Context, ts.LogType) (*ts.LogstreamConfiguration, error)
	setLogstreamConfiguration    func(context.Context, ts.LogType, ts.SetLogstreamConfigurationRequest) error
	deleteLogstreamConfiguration func(context.Context, ts.LogType) error
	createOrGetAwsExternalId     func(context.Context, bool) (*ts.AWSExternalID, error)
}

func (f fakeLogging) LogstreamConfiguration(ctx context.Context, t ts.LogType) (*ts.LogstreamConfiguration, error) {
	if f.logstreamConfiguration == nil {
		return nil, nil
	}
	return f.logstreamConfiguration(ctx, t)
}
func (f fakeLogging) SetLogstreamConfiguration(ctx context.Context, t ts.LogType, r ts.SetLogstreamConfigurationRequest) error {
	if f.setLogstreamConfiguration == nil {
		return nil
	}
	return f.setLogstreamConfiguration(ctx, t, r)
}
func (f fakeLogging) DeleteLogstreamConfiguration(ctx context.Context, t ts.LogType) error {
	if f.deleteLogstreamConfiguration == nil {
		return nil
	}
	return f.deleteLogstreamConfiguration(ctx, t)
}
func (f fakeLogging) CreateOrGetAwsExternalId(ctx context.Context, reusable bool) (*ts.AWSExternalID, error) {
	if f.createOrGetAwsExternalId == nil {
		return nil, nil
	}
	return f.createOrGetAwsExternalId(ctx, reusable)
}

type fakeDevicePosture struct {
	listIntegrations  func(context.Context) ([]ts.PostureIntegration, error)
	createIntegration func(context.Context, ts.CreatePostureIntegrationRequest) (*ts.PostureIntegration, error)
	updateIntegration func(context.Context, string, ts.UpdatePostureIntegrationRequest) (*ts.PostureIntegration, error)
	deleteIntegration func(context.Context, string) error
	getIntegration    func(context.Context, string) (*ts.PostureIntegration, error)
}

func (f fakeDevicePosture) ListIntegrations(ctx context.Context) ([]ts.PostureIntegration, error) {
	if f.listIntegrations == nil {
		return nil, nil
	}
	return f.listIntegrations(ctx)
}
func (f fakeDevicePosture) CreateIntegration(ctx context.Context, r ts.CreatePostureIntegrationRequest) (*ts.PostureIntegration, error) {
	if f.createIntegration == nil {
		return nil, nil
	}
	return f.createIntegration(ctx, r)
}
func (f fakeDevicePosture) UpdateIntegration(ctx context.Context, id string, r ts.UpdatePostureIntegrationRequest) (*ts.PostureIntegration, error) {
	if f.updateIntegration == nil {
		return nil, nil
	}
	return f.updateIntegration(ctx, id, r)
}
func (f fakeDevicePosture) DeleteIntegration(ctx context.Context, id string) error {
	if f.deleteIntegration == nil {
		return nil
	}
	return f.deleteIntegration(ctx, id)
}
func (f fakeDevicePosture) GetIntegration(ctx context.Context, id string) (*ts.PostureIntegration, error) {
	if f.getIntegration == nil {
		return nil, nil
	}
	return f.getIntegration(ctx, id)
}

type fakeUsers struct {
	list func(context.Context, *ts.UserType, *ts.UserRole) ([]ts.User, error)
	get  func(context.Context, string) (*ts.User, error)
}

func (f fakeUsers) List(ctx context.Context, t *ts.UserType, r *ts.UserRole) ([]ts.User, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, t, r)
}
func (f fakeUsers) Get(ctx context.Context, id string) (*ts.User, error) {
	if f.get == nil {
		return nil, nil
	}
	return f.get(ctx, id)
}

// TestAPITimeoutPerTargetConfig verifies the per-call timeout is derived from
// each target config independently rather than shared globally. Two targets
// with different apiTimeoutSeconds must resolve to different timeouts.
func TestAPITimeoutPerTargetConfig(t *testing.T) {
	short := apiTimeoutFor(targetConfig{APITimeoutSeconds: 3})
	long := apiTimeoutFor(targetConfig{APITimeoutSeconds: 90})
	def := apiTimeoutFor(targetConfig{})
	if short != 3*time.Second || long != 90*time.Second {
		t.Fatalf("apiTimeoutFor: short=%s long=%s", short, long)
	}
	if def != defaultAPITimeout {
		t.Fatalf("apiTimeoutFor default: want %s got %s", defaultAPITimeout, def)
	}

	// getClient must surface the resolved timeout per target, not a single
	// global value, so concurrent multi-target operations stay isolated.
	p := &Plugin{}
	_, tA, err := p.getClient(rawJSON(t, map[string]any{"apiKey": "key-a", "apiTimeoutSeconds": 7}))
	if err != nil {
		t.Fatalf("getClient A: %v", err)
	}
	apiB, tB, err := p.getClient(rawJSON(t, map[string]any{"apiKey": "key-b", "apiTimeoutSeconds": 45}))
	if err != nil {
		t.Fatalf("getClient B: %v", err)
	}
	if tA != 7*time.Second || tB != 45*time.Second {
		t.Fatalf("getClient per-target timeout: A=%s B=%s", tA, tB)
	}
	if client, ok := apiB.(productionClient); !ok || client.c.HTTP == nil || client.c.HTTP.Timeout != 45*time.Second {
		t.Fatalf("getClient transport timeout: want 45s, got %#v", apiB)
	}

	apiC, tC, err := p.getClient(rawJSON(t, map[string]any{"apiKey": "key-b", "apiTimeoutSeconds": 90}))
	if err != nil {
		t.Fatalf("getClient C: %v", err)
	}
	if tC != 90*time.Second {
		t.Fatalf("getClient timeout C: want 90s got %s", tC)
	}
	if client, ok := apiC.(productionClient); !ok || client.c.HTTP == nil || client.c.HTTP.Timeout != 90*time.Second {
		t.Fatalf("getClient transport timeout after cache-key change: want 90s, got %#v", apiC)
	}
}

// TestAPIContextDerivesPerRequestDeadline verifies that the bounded context
// each handler receives carries its own deadline, derived from the per-request
// timeout attached by the plugin entry points.
func TestAPIContextDerivesPerRequestDeadline(t *testing.T) {
	c1, cancel1 := apiContext(contextWithAPITimeout(context.Background(), 5*time.Second))
	defer cancel1()
	c2, cancel2 := apiContext(contextWithAPITimeout(context.Background(), 30*time.Second))
	defer cancel2()

	d1, ok1 := c1.Deadline()
	d2, ok2 := c2.Deadline()
	if !ok1 || !ok2 {
		t.Fatalf("deadlines missing: ok1=%v ok2=%v", ok1, ok2)
	}
	// The longer timeout must yield a later deadline than the shorter one,
	// proving the two operations are bounded independently.
	if !d2.After(d1) {
		t.Fatalf("distinct timeouts should yield distinct deadlines: d1=%s d2=%s", d1, d2)
	}

	// A context with no attached timeout falls back to the default.
	c0, cancel0 := apiContext(context.Background())
	defer cancel0()
	d0, ok0 := c0.Deadline()
	if !ok0 {
		t.Fatal("default deadline missing")
	}
	if got := time.Until(d0); got > defaultAPITimeout || got <= defaultAPITimeout-time.Second {
		t.Fatalf("default deadline should be ~%s from now, got %s", defaultAPITimeout, got)
	}
}
