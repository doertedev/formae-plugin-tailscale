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

func TestWebhookCreateSuccess(t *testing.T) {
	var captured ts.CreateWebhookRequest
	p := newPluginWithClient(fakeAPI{webhooks: fakeWebhooks{
		create: func(_ context.Context, req ts.CreateWebhookRequest) (*ts.Webhook, error) {
			captured = req
			secret := "secret"
			return sampleWebhook(&secret), nil
		},
	}})
	res, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: WebhookResourceType,
		Properties:   json.RawMessage(`{"endpointUrl":"https://example.com/hook","providerType":"slack","subscriptions":["nodeCreated"]}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess || res.ProgressResult.NativeID != "hook-1" {
		t.Fatalf("progress: %+v", res.ProgressResult)
	}
	if captured.EndpointURL != "https://example.com/hook" || captured.ProviderType != ts.WebhookSlackProviderType || captured.Subscriptions[0] != ts.WebhookNodeCreated {
		t.Fatalf("request not forwarded: %+v", captured)
	}
}

func TestWebhookUpdateSubscriptionsOnly(t *testing.T) {
	var captured []ts.WebhookSubscriptionType
	p := newPluginWithClient(fakeAPI{webhooks: fakeWebhooks{
		update: func(_ context.Context, id string, subs []ts.WebhookSubscriptionType) (*ts.Webhook, error) {
			if id != "hook-1" {
				t.Fatalf("id: want hook-1, got %q", id)
			}
			captured = subs
			return sampleWebhook(nil), nil
		},
	}})
	res, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      WebhookResourceType,
		NativeID:          "hook-1",
		DesiredProperties: json.RawMessage(`{"endpointUrl":"https://example.com/hook","providerType":"slack","subscriptions":["nodeDeleted"]}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status: %q", res.ProgressResult.OperationStatus)
	}
	if len(captured) != 1 || captured[0] != ts.WebhookNodeDeleted {
		t.Fatalf("subscriptions: got %v", captured)
	}
}

func TestWebhookReadOmitsSecret(t *testing.T) {
	secret := "leak-me-not"
	p := newPluginWithClient(fakeAPI{webhooks: fakeWebhooks{
		get: func(context.Context, string) (*ts.Webhook, error) { return sampleWebhook(&secret), nil },
	}})
	res, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: WebhookResourceType, NativeID: "hook-1"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props WebhookProperties
	decodeJSON(t, []byte(res.Properties), &props)
	if props.Secret != "" {
		t.Fatalf("read must not return the signing secret, got %q", props.Secret)
	}
}

func sampleWebhook(secret *string) *ts.Webhook {
	return &ts.Webhook{
		EndpointID:       "hook-1",
		EndpointURL:      "https://example.com/hook",
		ProviderType:     ts.WebhookSlackProviderType,
		CreatorLoginName: "admin@example.com",
		Created:          time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		LastModified:     time.Date(2026, 1, 2, 3, 5, 5, 0, time.UTC),
		Subscriptions:    []ts.WebhookSubscriptionType{ts.WebhookNodeCreated},
		Secret:           secret,
	}
}
