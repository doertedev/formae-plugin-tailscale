// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() { register(ACLResourceType, aclHandler{}) }

// defaultACLPolicy is the permissive "allow all" policy that Tailscale applies
// to freshly created tailnets. It is used to reset the ACL on delete so the
// tailnet returns to an unmanaged, broadly-open baseline.
const defaultACLPolicy = `{
  // Allow all traffic by default (Tailscale tailnet default policy).
  "acls": [
    {"action": "accept", "src": ["*"], "dst": ["*:*"]}
  ]
}`

type ACLProperties struct {
	// Policy is the tailnet ACL expressed as a HuJSON/JSON string. It is stored
	// and compared verbatim so user formatting and comments are preserved.
	Policy string `json:"policy"`
	// Tailnet is a read-only label sentinel; it is always the singleton id.
	Tailnet string `json:"tailnet,omitempty"`
	// ETag reflects the ACL version observed on the most recent read.
	ETag string `json:"etag,omitempty"`
}

type aclHandler struct{}

func parseACLProperties(data json.RawMessage) (*ACLProperties, error) {
	var props ACLProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid acl properties: %w", err)
	}
	props.Policy = strings.TrimSpace(props.Policy)
	if props.Policy == "" {
		return nil, fmt.Errorf("acl properties missing 'policy'")
	}
	return &props, nil
}

func (aclHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID, policy string) (*resource.ProgressResult, error) {
	if err := traceAPICallNoResult(ctx, ACLResourceType, opLabel(op), "PolicyFile.Validate", func(ctx context.Context) error {
		return c.PolicyFile().Validate(ctx, policy)
	}); err != nil {
		return fail(op, nativeID, "acl policy validation failed: "+err.Error(), resource.OperationErrorCodeInvalidRequest), nil
	}
	if err := traceAPICallNoResult(ctx, ACLResourceType, opLabel(op), "PolicyFile.Set", func(ctx context.Context) error {
		return c.PolicyFile().Set(ctx, policy, "")
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	props := &ACLProperties{Policy: policy, Tailnet: nativeID}
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (aclHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseACLProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := aclHandler{}.apply(apiCtx, c, resource.OperationCreate, singletonNativeID, props.Policy)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (aclHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	raw, err := traceAPICall(apiCtx, ACLResourceType, "read", "PolicyFile.Raw", func(ctx context.Context) (*ts.RawACL, error) {
		return c.PolicyFile().Raw(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if raw == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	props := &ACLProperties{Policy: strings.TrimSpace(raw.HuJSON), Tailnet: req.NativeID, ETag: raw.ETag}
	b, _ := json.Marshal(props)
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (aclHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseACLProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := aclHandler{}.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, props.Policy)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete resets the ACL to the permissive Tailscale default policy. A tailnet
// always has exactly one ACL, so it cannot be removed outright; resetting to the
// documented default is the closest unmanaged baseline.
func (aclHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, ACLResourceType, "delete", "PolicyFile.Set", func(ctx context.Context) error {
		return c.PolicyFile().Set(ctx, defaultACLPolicy, "")
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (aclHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}
