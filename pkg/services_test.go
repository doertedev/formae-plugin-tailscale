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

func TestServiceLifecycle(t *testing.T) {
	var sent ts.Service
	p := newPluginWithClient(fakeAPI{services: fakeServices{
		createOrUpdate: func(_ context.Context, s ts.Service) error { sent = s; return nil },
		get: func(_ context.Context, _ string) (*ts.Service, error) {
			return &ts.Service{Name: "svc-1", Comment: "managed", Ports: []string{"443"}}, nil
		},
		list: func(context.Context) ([]ts.Service, error) {
			return []ts.Service{{Name: "svc-1"}, {Name: "svc-2"}}, nil
		},
		delete: func(context.Context, string) error { return nil },
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ServiceResourceType,
		Properties:   rawJSON(t, map[string]any{"name": "svc-1", "comment": "managed", "ports": []string{"443"}}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if sent.Name != "svc-1" || len(sent.Ports) != 1 {
		t.Fatalf("service forwarded: %+v", sent)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ServiceResourceType, NativeID: "svc-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props ServiceProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.Comment != "managed" || len(props.Ports) != 1 {
		t.Fatalf("read mapping: %+v", props)
	}

	if _, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: ServiceResourceType, NativeID: "svc-1"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestServiceUpdateRejectsRename(t *testing.T) {
	p := newPluginWithClient(fakeAPI{})
	res, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      ServiceResourceType,
		NativeID:          "svc-1",
		DesiredProperties: rawJSON(t, map[string]any{"name": "svc-2"}),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeNotUpdatable)
}

func TestServiceReadMapsNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{services: fakeServices{
		get: func(context.Context, string) (*ts.Service, error) { return nil, ts.APIError{Status: 404} },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ServiceResourceType, NativeID: "missing"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode: want NotFound got %q", res.ErrorCode)
	}
}
