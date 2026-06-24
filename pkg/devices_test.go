// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestDeviceAuthorizationLifecycle(t *testing.T) {
	var authed bool
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		setAuthorized: func(_ context.Context, _ string, a bool) error { authed = a; return nil },
		get: func(_ context.Context, _ string) (*ts.Device, error) {
			return &ts.Device{NodeID: "node-1", Authorized: true}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DeviceAuthorizationResourceType,
		Properties:   rawJSON(t, map[string]any{"deviceId": "node-1", "authorized": true}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if create.ProgressResult.NativeID != "node-1" {
		t.Fatalf("nativeID: want node-1 got %q", create.ProgressResult.NativeID)
	}
	if !authed {
		t.Fatal("authorized not forwarded")
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceAuthorizationResourceType, NativeID: "node-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DeviceAuthorizationProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if !props.Authorized || props.DeviceID != "node-1" {
		t.Fatalf("read mapping: %+v", props)
	}
}

func TestDeviceAuthorizationMissingID(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: DeviceAuthorizationResourceType,
		Properties:   rawJSON(t, map[string]any{"authorized": true}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode: want InvalidRequest got %q", res.ProgressResult.ErrorCode)
	}
}

func TestDeviceKeyReadMapsError(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		get: func(context.Context, string) (*ts.Device, error) {
			return nil, ts.APIError{Status: http.StatusNotFound}
		},
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceKeyResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}

func TestDeviceSubnetRoutesClearsOnDelete(t *testing.T) {
	var sent []string
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		setSubnetRoutes: func(_ context.Context, _ string, r []string) error { sent = r; return nil },
	}})

	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: DeviceSubnetRoutesResourceType, NativeID: "node-1"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if len(sent) != 0 {
		t.Fatalf("delete should clear routes, got %v", sent)
	}
}

func TestDeviceTagsUpdate(t *testing.T) {
	var sent []string
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		setTags: func(_ context.Context, _ string, tags []string) error { sent = tags; return nil },
	}})

	upd, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      DeviceTagsResourceType,
		NativeID:          "node-1",
		DesiredProperties: rawJSON(t, map[string]any{"deviceId": "node-1", "tags": []string{"tag:web"}}),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireSuccess(t, upd.ProgressResult)
	if len(sent) != 1 || sent[0] != "tag:web" {
		t.Fatalf("tags forwarded: %v", sent)
	}
}

func TestDeviceTagsReadIsSorted(t *testing.T) {
	// Reads must normalize set-like collections so repeated reads of the same
	// state are byte-identical even if the API reorders values.
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		get: func(_ context.Context, _ string) (*ts.Device, error) {
			return &ts.Device{NodeID: "node-1", Tags: []string{"tag:web", "tag:api", "tag:db"}}, nil
		},
	}})
	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceTagsResourceType, NativeID: "node-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DeviceTagsProperties
	decodeJSON(t, []byte(read.Properties), &props)
	want := []string{"tag:api", "tag:db", "tag:web"}
	if len(props.Tags) != len(want) {
		t.Fatalf("tags: want %v got %v", want, props.Tags)
	}
	for i := range want {
		if props.Tags[i] != want[i] {
			t.Fatalf("tags not sorted: want %v got %v", want, props.Tags)
		}
	}
}

func TestDeviceInventoryReadAndList(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		get: func(_ context.Context, _ string) (*ts.Device, error) {
			return &ts.Device{NodeID: "node-1", Hostname: "host1", Authorized: true}, nil
		},
		list: func(context.Context, ...ts.ListDevicesOptions) ([]ts.Device, error) {
			return []ts.Device{{NodeID: "node-1"}, {NodeID: "node-2"}}, nil
		},
	}})

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: DeviceResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.NativeIDs) != 2 || list.NativeIDs[0] != "node-1" {
		t.Fatalf("list: %v", list.NativeIDs)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceResourceType, NativeID: "node-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props DeviceProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.Hostname != "host1" || !props.Authorized {
		t.Fatalf("read mapping: %+v", props)
	}

	// Device inventory is read-only: writes are not supported.
	requireUnsupportedWriteOps(t, p, DeviceResourceType, "node-1")
}

func TestDeviceInventoryReadNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		get: func(context.Context, string) (*ts.Device, error) { return nil, errors.New("boom") },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceResourceType, NativeID: "gone"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode == "" {
		t.Fatal("expected non-empty error code")
	}
}

// TestDeviceAspectMissingDeviceID verifies each writable device-aspect resource
// rejects create when the required deviceId is omitted.
func TestDeviceAspectMissingDeviceID(t *testing.T) {
	for _, rt := range []string{DeviceAuthorizationResourceType, DeviceKeyResourceType, DeviceSubnetRoutesResourceType, DeviceTagsResourceType} {
		p := newPluginWithClient(fakeAPI{})
		res, err := p.Create(context.Background(), &resource.CreateRequest{ResourceType: rt, Properties: rawJSON(t, map[string]any{})})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", rt, err)
		}
		requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
	}
}

// TestDeviceAspectUpdateRejectsMismatchedDeviceID verifies an update that
// supplies a deviceId different from the native id is rejected rather than
// silently redirected to another device.
func TestDeviceAspectUpdateRejectsMismatchedDeviceID(t *testing.T) {
	for _, rt := range []string{DeviceAuthorizationResourceType, DeviceKeyResourceType, DeviceSubnetRoutesResourceType, DeviceTagsResourceType} {
		p := newPluginWithClient(fakeAPI{})
		res, err := p.Update(context.Background(), &resource.UpdateRequest{
			ResourceType:      rt,
			NativeID:          "node-1",
			DesiredProperties: rawJSON(t, map[string]any{"deviceId": "node-other"}),
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", rt, err)
		}
		requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
	}
}

func TestDeviceSubnetRoutesReadMapsError(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devices: fakeDevices{
		subnetRoutes: func(context.Context, string) (*ts.DeviceRoutes, error) {
			return nil, ts.APIError{Status: http.StatusNotFound}
		},
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: DeviceSubnetRoutesResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}
