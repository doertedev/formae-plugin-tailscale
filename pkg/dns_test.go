// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestDNSNameserversLifecycle(t *testing.T) {
	var set []string
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setNameservers: func(_ context.Context, ns []string) error { set = ns; return nil },
		nameservers:    func(context.Context) ([]string, error) { return []string{"1.1.1.1"}, nil },
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DNSNameserversResourceType,
		Properties:   rawJSON(t, map[string]any{"nameservers": []string{"1.1.1.1", "8.8.8.8"}}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if len(set) != 2 || set[0] != "1.1.1.1" {
		t.Fatalf("nameservers forwarded: %v", set)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DNSNameserversResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DNSNameserversProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if len(props.Nameservers) != 1 || props.Nameservers[0] != "1.1.1.1" {
		t.Fatalf("read mapping: %+v", props)
	}

	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DNSNameserversResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if len(set) != 0 {
		t.Fatalf("delete should clear nameservers, got %v", set)
	}
}

func TestDNSPreferencesLifecycle(t *testing.T) {
	var magicSet bool
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setPreferences: func(_ context.Context, prefs ts.DNSPreferences) error { magicSet = prefs.MagicDNS; return nil },
		preferences:    func(context.Context) (*ts.DNSPreferences, error) { return &ts.DNSPreferences{MagicDNS: true}, nil },
	}})

	if _, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DNSPreferencesResourceType,
		Properties:   rawJSON(t, map[string]any{"magicDNS": true}),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !magicSet {
		t.Fatal("magicDNS not forwarded")
	}
}

func TestDNSSearchPathsUpdateAndRead(t *testing.T) {
	var paths []string
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setSearchPaths: func(_ context.Context, sp []string) error { paths = sp; return nil },
		searchPaths:    func(context.Context) ([]string, error) { return []string{"example.com"}, nil },
	}})

	upd, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      DNSSearchPathsResourceType,
		NativeID:          singletonNativeID,
		DesiredProperties: rawJSON(t, map[string]any{"searchPaths": []string{"foo.com", "bar.com"}}),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireSuccess(t, upd.ProgressResult)
	if len(paths) != 2 {
		t.Fatalf("search paths forwarded: %v", paths)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DNSSearchPathsResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DNSSearchPathsProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if len(props.SearchPaths) != 1 || props.SearchPaths[0] != "example.com" {
		t.Fatalf("read mapping: %+v", props)
	}
}

func TestDNSSplitNameserversRoundTrip(t *testing.T) {
	var sent ts.SplitDNSRequest
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setSplitDNS: func(_ context.Context, r ts.SplitDNSRequest) error { sent = r; return nil },
		splitDNS: func(context.Context) (ts.SplitDNSResponse, error) {
			return ts.SplitDNSResponse{"example.com": {"1.1.1.1"}}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DNSSplitNameserversResourceType,
		Properties:   rawJSON(t, map[string]any{"splitDNS": map[string][]string{"example.com": {"9.9.9.9"}}}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if v := sent["example.com"]; len(v) != 1 || v[0] != "9.9.9.9" {
		t.Fatalf("split dns forwarded: %v", sent)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DNSSplitNameserversResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DNSSplitNameserversProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if v := props.SplitDNS["example.com"]; len(v) != 1 || v[0] != "1.1.1.1" {
		t.Fatalf("read mapping: %+v", props)
	}
}

func TestDNSConfigurationLifecycle(t *testing.T) {
	var sent ts.DNSConfiguration
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setConfiguration: func(_ context.Context, c ts.DNSConfiguration) error { sent = c; return nil },
		configuration: func(context.Context) (*ts.DNSConfiguration, error) {
			return &ts.DNSConfiguration{
				Nameservers: []ts.DNSConfigurationResolver{{Address: "1.1.1.1"}},
				Preferences: ts.DNSConfigurationPreferences{MagicDNS: true},
			}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DNSConfigurationResourceType,
		Properties: rawJSON(t, map[string]any{
			"nameservers": []map[string]any{{"address": "1.1.1.1", "useWithExitNode": true}},
			"searchPaths": []string{"internal.example.com"},
			"preferences": map[string]any{"magicDNS": true, "overrideLocalDNS": false},
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if len(sent.Nameservers) != 1 || sent.Nameservers[0].Address != "1.1.1.1" || !sent.Nameservers[0].UseWithExitNode {
		t.Fatalf("nameservers forwarded: %+v", sent.Nameservers)
	}
	if !sent.Preferences.MagicDNS {
		t.Fatal("preferences not forwarded")
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DNSConfigurationResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DNSConfigurationProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if len(props.Nameservers) != 1 || props.Nameservers[0].Address != "1.1.1.1" {
		t.Fatalf("read mapping: %+v", props)
	}
}

// TestDNSConfigurationPartialNameserversPreservesOthers verifies that applying
// only nameservers leaves the other live DNS sections (searchPaths,
// preferences) intact instead of wiping them to zero values.
func TestDNSConfigurationPartialNameserversPreservesOthers(t *testing.T) {
	var sent ts.DNSConfiguration
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setConfiguration: func(_ context.Context, c ts.DNSConfiguration) error { sent = c; return nil },
		configuration: func(context.Context) (*ts.DNSConfiguration, error) {
			return &ts.DNSConfiguration{
				SearchPaths: []string{"keep.example.com"},
				Preferences: ts.DNSConfigurationPreferences{MagicDNS: true, OverrideLocalDNS: true},
			}, nil
		},
	}})

	if _, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DNSConfigurationResourceType,
		Properties:   rawJSON(t, map[string]any{"nameservers": []map[string]any{{"address": "8.8.8.8"}}}),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(sent.Nameservers) != 1 || sent.Nameservers[0].Address != "8.8.8.8" {
		t.Fatalf("nameservers not applied: %+v", sent.Nameservers)
	}
	if len(sent.SearchPaths) != 1 || sent.SearchPaths[0] != "keep.example.com" {
		t.Fatalf("omitted searchPaths should be preserved: %+v", sent.SearchPaths)
	}
	if !sent.Preferences.MagicDNS || !sent.Preferences.OverrideLocalDNS {
		t.Fatalf("omitted preferences should be preserved: %+v", sent.Preferences)
	}
}

// TestDNSConfigurationPartialPreferencesMagicDNS verifies that applying only
// preferences.magicDNS preserves the other live sections and the omitted
// overrideLocalDNS preference.
func TestDNSConfigurationPartialPreferencesMagicDNS(t *testing.T) {
	var sent ts.DNSConfiguration
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setConfiguration: func(_ context.Context, c ts.DNSConfiguration) error { sent = c; return nil },
		configuration: func(context.Context) (*ts.DNSConfiguration, error) {
			return &ts.DNSConfiguration{
				Nameservers: []ts.DNSConfigurationResolver{{Address: "1.1.1.1"}},
				Preferences: ts.DNSConfigurationPreferences{OverrideLocalDNS: true},
			}, nil
		},
	}})

	if _, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      DNSConfigurationResourceType,
		NativeID:          singletonNativeID,
		DesiredProperties: rawJSON(t, map[string]any{"preferences": map[string]any{"magicDNS": true}}),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !sent.Preferences.MagicDNS {
		t.Fatal("magicDNS not applied")
	}
	if !sent.Preferences.OverrideLocalDNS {
		t.Fatalf("omitted overrideLocalDNS should be preserved: %+v", sent.Preferences)
	}
	if len(sent.Nameservers) != 1 || sent.Nameservers[0].Address != "1.1.1.1" {
		t.Fatalf("omitted nameservers should be preserved: %+v", sent.Nameservers)
	}
}

// TestDNSPreferencesResetOnDelete confirms delete resets MagicDNS to disabled.
func TestDNSPreferencesResetOnDelete(t *testing.T) {
	var got ts.DNSPreferences
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setPreferences: func(_ context.Context, prefs ts.DNSPreferences) error { got = prefs; return nil },
	}})
	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DNSPreferencesResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if got.MagicDNS {
		t.Fatalf("delete should disable MagicDNS, got %+v", got)
	}
}

func TestDNSSearchPathsResetOnDelete(t *testing.T) {
	var got []string
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setSearchPaths: func(_ context.Context, sp []string) error { got = sp; return nil },
	}})
	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DNSSearchPathsResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if len(got) != 0 {
		t.Fatalf("delete should clear search paths, got %v", got)
	}
}

func TestDNSSplitNameserversResetOnDelete(t *testing.T) {
	var got ts.SplitDNSRequest
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setSplitDNS: func(_ context.Context, r ts.SplitDNSRequest) error { got = r; return nil },
	}})
	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DNSSplitNameserversResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if len(got) != 0 {
		t.Fatalf("delete should clear split DNS, got %v", got)
	}
}

func TestDNSConfigurationResetOnDelete(t *testing.T) {
	var got ts.DNSConfiguration
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		setConfiguration: func(_ context.Context, c ts.DNSConfiguration) error { got = c; return nil },
	}})
	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DNSConfigurationResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if len(got.Nameservers) != 0 || len(got.SearchPaths) != 0 || len(got.SplitDNS) != 0 {
		t.Fatalf("delete should reset configuration to empty, got %+v", got)
	}
}

func TestDNSNameserversReadIsSorted(t *testing.T) {
	p := newPluginWithClient(fakeAPI{dns: fakeDNS{
		nameservers: func(context.Context) ([]string, error) { return []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"}, nil },
	}})
	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DNSNameserversResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DNSNameserversProperties
	decodeJSON(t, []byte(read.Properties), &props)
	want := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}
	if len(props.Nameservers) != len(want) {
		t.Fatalf("nameservers: want %v got %v", want, props.Nameservers)
	}
	for i := range want {
		if props.Nameservers[i] != want[i] {
			t.Fatalf("nameservers not sorted: want %v got %v", want, props.Nameservers)
		}
	}
}
