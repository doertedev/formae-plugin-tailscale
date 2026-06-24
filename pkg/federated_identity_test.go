// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestFederatedIdentityLifecycle(t *testing.T) {
	var createdReq ts.CreateFederatedIdentityRequest
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		createFederatedIdentity: func(_ context.Context, r ts.CreateFederatedIdentityRequest) (*ts.Key, error) {
			createdReq = r
			return &ts.Key{ID: "fid-1", KeyType: "federated", Description: r.Description, Scopes: r.Scopes, Audience: r.Audience}, nil
		},
		setFederatedIdentity: func(_ context.Context, _ string, _ ts.SetFederatedIdentityRequest) (*ts.Key, error) {
			return &ts.Key{ID: "fid-1", KeyType: "federated", Description: "updated"}, nil
		},
		get: func(_ context.Context, _ string) (*ts.Key, error) {
			return &ts.Key{ID: "fid-1", KeyType: "federated", Scopes: []string{"auth_keys:read"}}, nil
		},
		list: func(context.Context, bool) ([]ts.Key, error) {
			return []ts.Key{{ID: "fid-1", KeyType: "federated"}, {ID: "oc-1", KeyType: "client"}}, nil
		},
		delete: func(context.Context, string) error { return nil },
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: FederatedIdentityResourceType,
		Properties: rawJSON(t, map[string]any{
			"description": "ci", "scopes": []string{"auth_keys:read"}, "audience": "aud", "issuer": "iss", "subject": "sub",
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if createdReq.Audience != "aud" {
		t.Fatalf("audience not forwarded: %+v", createdReq)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: FederatedIdentityResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	requireNativeIDs(t, list, "fid-1")
}

func TestFederatedIdentityMissingScopes(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: FederatedIdentityResourceType,
		Properties:   rawJSON(t, map[string]any{"description": "no scopes"}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
}

func TestFederatedIdentityReadRevokedIsNotFound(t *testing.T) {
	// A revoked/deleted key is still returned by Get for a while; read must
	// surface NotFound so formae can tombstone it.
	revoked := &ts.Key{ID: "fid-1", KeyType: "federated"}
	revoked.Revoked = time.Date(2026, 6, 24, 5, 26, 4, 0, time.UTC)
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		get: func(context.Context, string) (*ts.Key, error) { return revoked, nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: FederatedIdentityResourceType, NativeID: "fid-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}
