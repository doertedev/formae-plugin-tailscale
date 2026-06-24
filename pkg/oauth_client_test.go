// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestOAuthClientCreateSuccess(t *testing.T) {
	var captured ts.CreateOAuthClientRequest
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		createOAuthClient: func(_ context.Context, req ts.CreateOAuthClientRequest) (*ts.Key, error) {
			captured = req
			return sampleOAuthClient(), nil
		},
	}})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: OAuthClientResourceType,
		Properties:   json.RawMessage(`{"description":"automation","scopes":["auth_keys:read"],"tags":["tag:ci"]}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess || res.ProgressResult.NativeID != "client-1" {
		t.Fatalf("progress: %+v", res.ProgressResult)
	}
	if captured.Description != "automation" || len(captured.Scopes) != 1 || captured.Scopes[0] != "auth_keys:read" {
		t.Fatalf("request not forwarded: %+v", captured)
	}
}

func TestOAuthClientUpdateSuccess(t *testing.T) {
	var captured ts.SetOAuthClientRequest
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		setOAuthClient: func(_ context.Context, id string, req ts.SetOAuthClientRequest) (*ts.Key, error) {
			if id != "client-1" {
				t.Fatalf("id: want client-1, got %q", id)
			}
			captured = req
			return sampleOAuthClient(), nil
		},
	}})
	res, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      OAuthClientResourceType,
		NativeID:          "client-1",
		DesiredProperties: json.RawMessage(`{"description":"updated","scopes":["devices:core"],"tags":[]}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status: %q", res.ProgressResult.OperationStatus)
	}
	if captured.Description != "updated" || captured.Scopes[0] != "devices:core" {
		t.Fatalf("request not forwarded: %+v", captured)
	}
}

func TestOAuthClientReadOmitsKeyMaterial(t *testing.T) {
	key := sampleOAuthClient()
	key.Key = "tskey-client-leak"
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		get: func(context.Context, string) (*ts.Key, error) { return key, nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: OAuthClientResourceType, NativeID: "client-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props OAuthClientProperties
	decodeJSON(t, []byte(res.Properties), &props)
	if props.Key != "" {
		t.Fatalf("read must not return client secret, got %q", props.Key)
	}
}

func TestOAuthClientListFiltersClients(t *testing.T) {
	// OAuth client list must include only client keys, excluding auth and
	// federated keys that share the same Keys.List endpoint.
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		list: func(context.Context, bool) ([]ts.Key, error) {
			return []ts.Key{
				{ID: "auth-1", KeyType: "auth"},
				{ID: "client-1", KeyType: "client"},
				{ID: "fid-1", KeyType: "federated"},
				{ID: "client-2", KeyType: "client"},
			}, nil
		},
	}})
	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: OAuthClientResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	requireNativeIDs(t, list, "client-1", "client-2")
}

func sampleOAuthClient() *ts.Key {
	return &ts.Key{
		ID:          "client-1",
		KeyType:     "client",
		Key:         "tskey-client-secret",
		Description: "automation",
		Scopes:      []string{"auth_keys:read"},
		Tags:        []string{"tag:ci"},
		Created:     time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Updated:     time.Date(2026, 1, 2, 3, 5, 5, 0, time.UTC),
		UserID:      "u1",
	}
}
