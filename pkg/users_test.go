// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestUserReadAndList(t *testing.T) {
	p := newPluginWithClient(fakeAPI{users: fakeUsers{
		get: func(_ context.Context, _ string) (*ts.User, error) {
			return &ts.User{ID: "u-1", LoginName: "alice@example.com", Role: ts.UserRoleAdmin, Status: ts.UserStatusActive, DeviceCount: 3}, nil
		},
		list: func(context.Context, *ts.UserType, *ts.UserRole) ([]ts.User, error) {
			return []ts.User{{ID: "u-1"}, {ID: "u-2"}}, nil
		},
	}})

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: UserResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.NativeIDs) != 2 || list.NativeIDs[0] != "u-1" {
		t.Fatalf("list: %v", list.NativeIDs)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: UserResourceType, NativeID: "u-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props UserProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.LoginName != "alice@example.com" || props.Role != string(ts.UserRoleAdmin) || props.DeviceCount != 3 {
		t.Fatalf("read mapping: %+v", props)
	}
}

func TestUserWriteOpsNotSupported(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	requireUnsupportedWriteOps(t, p, UserResourceType, "u-1")
}

func TestUserReadMapsNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{users: fakeUsers{
		get: func(context.Context, string) (*ts.User, error) { return nil, ts.APIError{Status: http.StatusNotFound} },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: UserResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}

func TestUserListEmpty(t *testing.T) {
	p := newPluginWithClient(fakeAPI{users: fakeUsers{
		list: func(context.Context, *ts.UserType, *ts.UserRole) ([]ts.User, error) { return nil, nil },
	}})
	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: UserResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.NativeIDs) != 0 {
		t.Fatalf("expected empty user list, got %v", list.NativeIDs)
	}
}
