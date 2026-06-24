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

func TestACLLifecycle(t *testing.T) {
	var setPolicy any
	p := newPluginWithClient(fakeAPI{policyFile: fakePolicyFile{
		set: func(_ context.Context, acl any, _ string) error {
			setPolicy = acl
			return nil
		},
		raw: func(context.Context) (*ts.RawACL, error) {
			return &ts.RawACL{HuJSON: "{ \"acls\": [] }", ETag: "etag-1"}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ACLResourceType,
		Properties:   rawJSON(t, map[string]any{"policy": "{ \"acls\": [] }"}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if create.ProgressResult.NativeID != singletonNativeID {
		t.Fatalf("nativeID: want %q got %q", singletonNativeID, create.ProgressResult.NativeID)
	}
	if setPolicy != "{ \"acls\": [] }" {
		t.Fatalf("policy not forwarded verbatim: %v", setPolicy)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ACLResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props ACLProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.ETag != "etag-1" || props.Tailnet != singletonNativeID {
		t.Fatalf("read mapping: %+v", props)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: ACLResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.NativeIDs) != 1 || list.NativeIDs[0] != singletonNativeID {
		t.Fatalf("list: %v", list.NativeIDs)
	}

	del, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: ACLResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if del.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("delete status: %s", del.ProgressResult.OperationStatus)
	}
	if setPolicy != defaultACLPolicy {
		t.Fatalf("delete should reset to default policy")
	}
}

func TestACLValidationFailureRejected(t *testing.T) {
	p := newPluginWithClient(fakeAPI{policyFile: fakePolicyFile{
		validate: func(context.Context, any) error { return errors.New("bad acl: unknown field") },
	}})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ACLResourceType,
		Properties:   rawJSON(t, map[string]any{"policy": "garbage"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
}

func TestACLCreateReturnsProperties(t *testing.T) {
	// Create must echo the resolved properties (policy + tailnet sentinel), not
	// just the native id, so drift detection has a baseline immediately.
	p := newPluginWithClient(fakeAPI{policyFile: fakePolicyFile{
		raw: func(context.Context) (*ts.RawACL, error) { return &ts.RawACL{HuJSON: "{ \"acls\": [] }"}, nil },
	}})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ACLResourceType,
		Properties:   rawJSON(t, map[string]any{"policy": "{ \"acls\": [] }"}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, res.ProgressResult)
	var props ACLProperties
	decodeJSON(t, res.ProgressResult.ResourceProperties, &props)
	if props.Policy != "{ \"acls\": [] }" || props.Tailnet != singletonNativeID {
		t.Fatalf("create properties: %+v", props)
	}
}

func TestACLReadMapsError(t *testing.T) {
	p := newPluginWithClient(fakeAPI{policyFile: fakePolicyFile{
		raw: func(context.Context) (*ts.RawACL, error) { return nil, ts.APIError{Status: http.StatusForbidden} },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ACLResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeAccessDenied {
		t.Fatalf("ErrorCode: want AccessDenied got %q", res.ErrorCode)
	}
}
