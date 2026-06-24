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

func TestTailnetSettingsLifecycle(t *testing.T) {
	var captured ts.UpdateTailnetSettingsRequest
	p := newPluginWithClient(fakeAPI{tailnetSettings: fakeTailnetSettings{
		update: func(_ context.Context, req ts.UpdateTailnetSettingsRequest) error {
			captured = req
			return nil
		},
		get: func(context.Context) (*ts.TailnetSettings, error) {
			return &ts.TailnetSettings{NetworkFlowLoggingOn: true, HTTPSEnabled: true}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: TailnetSettingsResourceType,
		Properties: rawJSON(t, map[string]any{
			"networkFlowLoggingOn": true, "httpsEnabled": true, "usersRoleAllowedToJoinExternalTailnets": "member",
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if captured.NetworkFlowLoggingOn == nil || !*captured.NetworkFlowLoggingOn {
		t.Fatal("networkFlowLoggingOn not forwarded")
	}
	if captured.UsersRoleAllowedToJoinExternalTailnets == nil || *captured.UsersRoleAllowedToJoinExternalTailnets != ts.RoleAllowedToJoinExternalTailnetsMember {
		t.Fatal("role not forwarded")
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: TailnetSettingsResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props TailnetSettingsProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.NetworkFlowLoggingOn == nil || !*props.NetworkFlowLoggingOn || props.HTTPSEnabled == nil || !*props.HTTPSEnabled {
		t.Fatalf("read mapping: %+v", props)
	}
}

// TestTailnetSettingsPartialUpdateLeavesOmittedUnchanged verifies that a
// partial desired state (only networkFlowLoggingOn) produces an update request
// whose other pointer fields are nil, so the API leaves those settings unchanged.
func TestTailnetSettingsPartialUpdateLeavesOmittedUnchanged(t *testing.T) {
	var captured ts.UpdateTailnetSettingsRequest
	p := newPluginWithClient(fakeAPI{tailnetSettings: fakeTailnetSettings{
		update: func(_ context.Context, req ts.UpdateTailnetSettingsRequest) error {
			captured = req
			return nil
		},
	}})

	_, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      TailnetSettingsResourceType,
		NativeID:          singletonNativeID,
		DesiredProperties: rawJSON(t, map[string]any{"networkFlowLoggingOn": true}),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if captured.NetworkFlowLoggingOn == nil || !*captured.NetworkFlowLoggingOn {
		t.Fatalf("networkFlowLoggingOn should be set: %+v", captured)
	}
	// Every other pointer must be nil so the PATCH leaves those settings
	// unchanged. (Compare typed pointers directly: a nil *bool wrapped in an
	// interface is itself non-nil.)
	for name, ptr := range map[string]*bool{
		"devicesApprovalOn":           captured.DevicesApprovalOn,
		"devicesAutoUpdatesOn":        captured.DevicesAutoUpdatesOn,
		"usersApprovalOn":             captured.UsersApprovalOn,
		"regionalRoutingOn":           captured.RegionalRoutingOn,
		"postureIdentityCollectionOn": captured.PostureIdentityCollectionOn,
		"httpsEnabled":                captured.HTTPSEnabled,
	} {
		if ptr != nil {
			t.Fatalf("omitted field %s should be nil, got %v", name, *ptr)
		}
	}
	if captured.DevicesKeyDurationDays != nil {
		t.Fatalf("omitted devicesKeyDurationDays should be nil, got %d", *captured.DevicesKeyDurationDays)
	}
	if captured.UsersRoleAllowedToJoinExternalTailnets != nil {
		t.Fatalf("omitted role should be nil, got %v", *captured.UsersRoleAllowedToJoinExternalTailnets)
	}
}

func TestTailnetSettingsInvalidRole(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: TailnetSettingsResourceType,
		Properties:   rawJSON(t, map[string]any{"usersRoleAllowedToJoinExternalTailnets": "superuser"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
}

func TestTailnetSettingsReadMapsError(t *testing.T) {
	p := newPluginWithClient(fakeAPI{tailnetSettings: fakeTailnetSettings{
		get: func(context.Context) (*ts.TailnetSettings, error) {
			return nil, ts.APIError{Status: http.StatusUnauthorized}
		},
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: TailnetSettingsResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeInvalidCredentials {
		t.Fatalf("ErrorCode: want InvalidCredentials got %q", res.ErrorCode)
	}
}
