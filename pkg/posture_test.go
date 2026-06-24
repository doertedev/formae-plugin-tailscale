// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestPostureIntegrationLifecycle(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devicePosture: fakeDevicePosture{
		createIntegration: func(_ context.Context, r ts.CreatePostureIntegrationRequest) (*ts.PostureIntegration, error) {
			if r.ClientSecret == "" {
				t.Fatal("client secret not forwarded")
			}
			return &ts.PostureIntegration{ID: "pid-1", Provider: r.Provider, CloudID: r.CloudID}, nil
		},
		getIntegration: func(_ context.Context, id string) (*ts.PostureIntegration, error) {
			return &ts.PostureIntegration{ID: id, Provider: ts.PostureIntegrationProviderFalcon, CloudID: "cloud-9"}, nil
		},
		updateIntegration: func(_ context.Context, _ string, r ts.UpdatePostureIntegrationRequest) (*ts.PostureIntegration, error) {
			if r.ClientSecret != nil {
				t.Fatal("empty client secret should be omitted to preserve existing value")
			}
			return &ts.PostureIntegration{ID: "pid-1", CloudID: r.CloudID}, nil
		},
		deleteIntegration: func(context.Context, string) error { return nil },
		listIntegrations: func(context.Context) ([]ts.PostureIntegration, error) {
			return []ts.PostureIntegration{{ID: "pid-1"}}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: PostureIntegrationResourceType,
		Properties: rawJSON(t, map[string]any{
			"provider": "falcon", "cloudId": "cloud-9", "clientSecret": "shh",
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	requireNativeID(t, create.ProgressResult, "pid-1")

	if _, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      PostureIntegrationResourceType,
		NativeID:          "pid-1",
		DesiredProperties: rawJSON(t, map[string]any{"provider": "falcon", "cloudId": "cloud-10"}),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestPostureMissingProvider(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: PostureIntegrationResourceType,
		Properties:   rawJSON(t, map[string]any{"cloudId": "cloud-1"}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeInvalidRequest)
}

func TestPostureReadMapsNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{devicePosture: fakeDevicePosture{
		getIntegration: func(context.Context, string) (*ts.PostureIntegration, error) {
			return nil, ts.APIError{Status: 404}
		},
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: PostureIntegrationResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}
