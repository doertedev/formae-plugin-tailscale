// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() { register(TailnetSettingsResourceType, tailnetSettingsHandler{}) }

// TailnetSettingsProperties mirrors the fields exposed by the Tailscale tailnet
// settings API. Pointer fields preserve partial-state semantics: an omitted
// property unmarshals to a nil pointer and is forwarded as nil to the API's
// PATCH endpoint, which leaves the existing setting unchanged. A non-nil
// pointer (including one to false/0) is always written.
type TailnetSettingsProperties struct {
	Tailnet string `json:"tailnet,omitempty"`

	DevicesApprovalOn      *bool `json:"devicesApprovalOn,omitempty"`
	DevicesAutoUpdatesOn   *bool `json:"devicesAutoUpdatesOn,omitempty"`
	DevicesKeyDurationDays *int  `json:"devicesKeyDurationDays,omitempty"`

	UsersApprovalOn                        *bool  `json:"usersApprovalOn,omitempty"`
	UsersRoleAllowedToJoinExternalTailnets string `json:"usersRoleAllowedToJoinExternalTailnets,omitempty"`

	NetworkFlowLoggingOn        *bool `json:"networkFlowLoggingOn,omitempty"`
	RegionalRoutingOn           *bool `json:"regionalRoutingOn,omitempty"`
	PostureIdentityCollectionOn *bool `json:"postureIdentityCollectionOn,omitempty"`
	HTTPSEnabled                *bool `json:"httpsEnabled,omitempty"`
}

type tailnetSettingsHandler struct{}

func parseTailnetSettingsProperties(data json.RawMessage) (*TailnetSettingsProperties, error) {
	var props TailnetSettingsProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid tailnet settings properties: %w", err)
	}
	if props.UsersRoleAllowedToJoinExternalTailnets != "" {
		switch ts.RoleAllowedToJoinExternalTailnets(props.UsersRoleAllowedToJoinExternalTailnets) {
		case ts.RoleAllowedToJoinExternalTailnetsNone, ts.RoleAllowedToJoinExternalTailnetsAdmin, ts.RoleAllowedToJoinExternalTailnetsMember:
		default:
			return nil, fmt.Errorf("invalid usersRoleAllowedToJoinExternalTailnets %q: want none|admin|member", props.UsersRoleAllowedToJoinExternalTailnets)
		}
	}
	return &props, nil
}

func settingsUpdateRequest(props *TailnetSettingsProperties) ts.UpdateTailnetSettingsRequest {
	// Only set pointers for fields present in desired state. The Tailscale
	// PATCH endpoint treats a nil pointer as "leave unchanged", so a partial
	// resource only updates the settings it declares.
	req := ts.UpdateTailnetSettingsRequest{
		DevicesApprovalOn:           props.DevicesApprovalOn,
		DevicesAutoUpdatesOn:        props.DevicesAutoUpdatesOn,
		DevicesKeyDurationDays:      props.DevicesKeyDurationDays,
		UsersApprovalOn:             props.UsersApprovalOn,
		NetworkFlowLoggingOn:        props.NetworkFlowLoggingOn,
		RegionalRoutingOn:           props.RegionalRoutingOn,
		PostureIdentityCollectionOn: props.PostureIdentityCollectionOn,
		HTTPSEnabled:                props.HTTPSEnabled,
	}
	if props.UsersRoleAllowedToJoinExternalTailnets != "" {
		role := ts.RoleAllowedToJoinExternalTailnets(props.UsersRoleAllowedToJoinExternalTailnets)
		req.UsersRoleAllowedToJoinExternalTailnets = &role
	}
	return req
}

func settingsFrom(s *ts.TailnetSettings, nativeID string) *TailnetSettingsProperties {
	if s == nil {
		return nil
	}
	return &TailnetSettingsProperties{
		Tailnet:                                nativeID,
		DevicesApprovalOn:                      &s.DevicesApprovalOn,
		DevicesAutoUpdatesOn:                   &s.DevicesAutoUpdatesOn,
		DevicesKeyDurationDays:                 &s.DevicesKeyDurationDays,
		UsersApprovalOn:                        &s.UsersApprovalOn,
		UsersRoleAllowedToJoinExternalTailnets: string(s.UsersRoleAllowedToJoinExternalTailnets),
		NetworkFlowLoggingOn:                   &s.NetworkFlowLoggingOn,
		RegionalRoutingOn:                      &s.RegionalRoutingOn,
		PostureIdentityCollectionOn:            &s.PostureIdentityCollectionOn,
		HTTPSEnabled:                           &s.HTTPSEnabled,
	}
}

func (tailnetSettingsHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseTailnetSettingsProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, TailnetSettingsResourceType, "create", "TailnetSettings.Update", func(ctx context.Context) error {
		return c.TailnetSettings().Update(ctx, settingsUpdateRequest(props))
	}); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, singletonNativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props.Tailnet = singletonNativeID
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, singletonNativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (tailnetSettingsHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	s, err := traceAPICall(apiCtx, TailnetSettingsResourceType, "read", "TailnetSettings.Get", func(ctx context.Context) (*ts.TailnetSettings, error) {
		return c.TailnetSettings().Get(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if s == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(settingsFrom(s, req.NativeID))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (tailnetSettingsHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseTailnetSettingsProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, TailnetSettingsResourceType, "update", "TailnetSettings.Update", func(ctx context.Context) error {
		return c.TailnetSettings().Update(ctx, settingsUpdateRequest(props))
	}); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props.Tailnet = req.NativeID
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete is a no-op success: tailnet settings are an always-present singleton
// with no meaningful "removed" state, so destroying the managed resource simply
// relinquishes management rather than mutating tailnet behavior.
func (tailnetSettingsHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (tailnetSettingsHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}
