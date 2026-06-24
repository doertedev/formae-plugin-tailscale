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

func TestAuthKeyCreateSuccess(t *testing.T) {
	var captured ts.CreateKeyRequest
	var sawDeadline bool
	key := sampleAuthKey()
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		createAuthKey: func(ctx context.Context, req ts.CreateKeyRequest) (*ts.Key, error) {
			captured = req
			deadline, ok := ctx.Deadline()
			sawDeadline = ok && time.Until(deadline) <= 15*time.Second && time.Until(deadline) > 0
			return key, nil
		},
	}})

	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: AuthKeyResourceType,
		Properties:   json.RawMessage(`{"reusable":true,"ephemeral":true,"tags":["tag:ci"],"preauthorized":true,"expirySeconds":3600,"description":"ci"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status: want Success, got %q", res.ProgressResult.OperationStatus)
	}
	if res.ProgressResult.NativeID != "key-1" {
		t.Fatalf("NativeID: want key-1, got %q", res.ProgressResult.NativeID)
	}
	if !captured.Capabilities.Devices.Create.Reusable || !captured.Capabilities.Devices.Create.Ephemeral || !captured.Capabilities.Devices.Create.Preauthorized {
		t.Fatalf("capabilities not forwarded: %+v", captured.Capabilities.Devices.Create)
	}
	if captured.ExpirySeconds != 3600 || captured.Description != "ci" {
		t.Fatalf("request fields not forwarded: %+v", captured)
	}
	if !sawDeadline {
		t.Fatal("expected CreateAuthKey to receive a bounded API context")
	}
}

func TestAuthKeyCreateDeadlineExceededMapsToServiceTimeout(t *testing.T) {
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		createAuthKey: func(context.Context, ts.CreateKeyRequest) (*ts.Key, error) {
			return nil, context.DeadlineExceeded
		},
	}})

	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: AuthKeyResourceType,
		Properties:   json.RawMessage(`{"description":"timeout"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeServiceTimeout {
		t.Fatalf("ErrorCode: want ServiceTimeout, got %q", res.ProgressResult.ErrorCode)
	}
}

func TestAuthKeyReadNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		get: func(context.Context, string) (*ts.Key, error) { return nil, nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: AuthKeyResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound, got %q", res.ErrorCode)
	}
}

func TestAuthKeyDeleteNotFoundIsFailureWithNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		delete: func(context.Context, string) error { return ts.APIError{Status: 404} },
	}})
	res, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: AuthKeyResourceType, NativeID: "gone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound, got %q", res.ProgressResult.ErrorCode)
	}
}

func TestAuthKeyReadRevokedIsNotFound(t *testing.T) {
	// Tailscale keeps returning a deleted key via Get with a non-zero Revoked
	// timestamp; read must treat such a key as gone so formae can tombstone it.
	revoked := sampleAuthKey()
	revoked.Revoked = time.Date(2026, 6, 24, 5, 26, 4, 0, time.UTC)
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		get: func(context.Context, string) (*ts.Key, error) { return revoked, nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: AuthKeyResourceType, NativeID: "key-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}

func TestAuthKeyListFiltersAuthKeys(t *testing.T) {
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		list: func(_ context.Context, all bool) ([]ts.Key, error) {
			if !all {
				t.Fatal("expected List(all=true)")
			}
			return []ts.Key{{ID: "auth", KeyType: "auth"}, {ID: "client", KeyType: "client"}}, nil
		},
	}})
	res, err := p.List(context.Background(), &resource.ListRequest{ResourceType: AuthKeyResourceType})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.NativeIDs) != 1 || res.NativeIDs[0] != "auth" {
		t.Fatalf("NativeIDs: want [auth], got %v", res.NativeIDs)
	}
}

func TestAuthKeyReadOmitsKeyMaterial(t *testing.T) {
	// The raw key string is emitted once at create time; read must never
	// re-surface it even if (hypothetically) the API returned it.
	key := sampleAuthKey()
	key.Key = "tskey-auth-leak"
	p := newPluginWithClient(fakeAPI{keys: fakeKeys{
		get: func(context.Context, string) (*ts.Key, error) { return key, nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: AuthKeyResourceType, NativeID: "key-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props AuthKeyProperties
	decodeJSON(t, []byte(res.Properties), &props)
	if props.Key != "" {
		t.Fatalf("read must not return key material, got %q", props.Key)
	}
}

func sampleAuthKey() *ts.Key {
	exp := 3600 * time.Second
	k := &ts.Key{
		ID:            "key-1",
		KeyType:       "auth",
		Key:           "tskey-auth-secret",
		Description:   "ci",
		ExpirySeconds: &exp,
		Created:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Expires:       time.Date(2026, 1, 2, 4, 4, 5, 0, time.UTC),
		UserID:        "u1",
	}
	k.Capabilities.Devices.Create.Reusable = true
	k.Capabilities.Devices.Create.Ephemeral = true
	k.Capabilities.Devices.Create.Preauthorized = true
	k.Capabilities.Devices.Create.Tags = []string{"tag:ci"}
	return k
}
