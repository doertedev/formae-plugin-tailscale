// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() { register(PostureIntegrationResourceType, postureIntegrationHandler{}) }

// PostureIntegrationProperties models a configured device posture integration.
// ClientSecret is write-only: it is accepted on create/update but never returned
// by the API, so it is omitted from read state.
type PostureIntegrationProperties struct {
	ID           string `json:"id,omitempty"`
	Provider     string `json:"provider"`
	CloudID      string `json:"cloudId,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	TenantID     string `json:"tenantId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

type postureIntegrationHandler struct{}

func postureIntegrationFrom(p *ts.PostureIntegration) *PostureIntegrationProperties {
	if p == nil {
		return nil
	}
	return &PostureIntegrationProperties{
		ID:       p.ID,
		Provider: string(p.Provider),
		CloudID:  p.CloudID,
		ClientID: p.ClientID,
		TenantID: p.TenantID,
	}
}

func (postureIntegrationHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props PostureIntegrationProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid posture integration properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.Provider == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "posture integration properties missing 'provider'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	intg, err := traceAPICall(apiCtx, PostureIntegrationResourceType, "create", "DevicePosture.CreateIntegration", func(ctx context.Context) (*ts.PostureIntegration, error) {
		return c.DevicePosture().CreateIntegration(ctx, ts.CreatePostureIntegrationRequest{
			Provider:     ts.PostureIntegrationProvider(props.Provider),
			CloudID:      props.CloudID,
			ClientID:     props.ClientID,
			TenantID:     props.TenantID,
			ClientSecret: props.ClientSecret,
		})
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	props = *postureIntegrationFrom(intg)
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.ID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (postureIntegrationHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	intg, err := traceAPICall(apiCtx, PostureIntegrationResourceType, "read", "DevicePosture.GetIntegration", func(ctx context.Context) (*ts.PostureIntegration, error) {
		return c.DevicePosture().GetIntegration(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if intg == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(postureIntegrationFrom(intg))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (postureIntegrationHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props PostureIntegrationProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid posture integration properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	updateReq := ts.UpdatePostureIntegrationRequest{
		CloudID:  props.CloudID,
		ClientID: props.ClientID,
		TenantID: props.TenantID,
	}
	if props.ClientSecret != "" {
		updateReq.ClientSecret = &props.ClientSecret
	}
	intg, err := traceAPICall(apiCtx, PostureIntegrationResourceType, "update", "DevicePosture.UpdateIntegration", func(ctx context.Context) (*ts.PostureIntegration, error) {
		return c.DevicePosture().UpdateIntegration(ctx, req.NativeID, updateReq)
	})
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props = *postureIntegrationFrom(intg)
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (postureIntegrationHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, PostureIntegrationResourceType, "delete", "DevicePosture.DeleteIntegration", func(ctx context.Context) error {
		return c.DevicePosture().DeleteIntegration(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (postureIntegrationHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	intgs, err := traceAPICall(apiCtx, PostureIntegrationResourceType, "list", "DevicePosture.ListIntegrations", func(ctx context.Context) ([]ts.PostureIntegration, error) {
		return c.DevicePosture().ListIntegrations(ctx)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(intgs))
	for _, i := range intgs {
		ids = append(ids, i.ID)
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
