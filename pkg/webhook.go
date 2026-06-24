// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

const WebhookResourceType = "TAILSCALE::Events::Webhook"

func init() { register(WebhookResourceType, webhookHandler{}) }

type WebhookProperties struct {
	EndpointURL      string   `json:"endpointUrl"`
	ProviderType     string   `json:"providerType,omitempty"`
	Subscriptions    []string `json:"subscriptions"`
	Secret           string   `json:"secret,omitempty"`
	EndpointID       string   `json:"endpointId,omitempty"`
	CreatorLoginName string   `json:"creatorLoginName,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	LastModifiedAt   string   `json:"lastModifiedAt,omitempty"`
}

type webhookHandler struct{}

func parseWebhookProperties(data json.RawMessage) (*WebhookProperties, error) {
	var props WebhookProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid webhook properties: %w", err)
	}
	if props.EndpointURL == "" {
		return nil, errors.New("webhook properties missing 'endpointUrl'")
	}
	if len(props.Subscriptions) == 0 {
		return nil, errors.New("webhook properties missing 'subscriptions'")
	}
	return &props, nil
}

func (webhookHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseWebhookProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	wh, err := traceAPICall(apiCtx, WebhookResourceType, "create", "Webhooks.Create", func(ctx context.Context) (*ts.Webhook, error) {
		return c.Webhooks().Create(ctx, ts.CreateWebhookRequest{
			EndpointURL:   props.EndpointURL,
			ProviderType:  ts.WebhookProviderType(props.ProviderType),
			Subscriptions: webhookSubscriptionTypes(props.Subscriptions),
		})
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	props = webhookFrom(wh)
	// The signing secret is emitted once on create and never re-read; surface it
	// only here so it does not leak into read/list/update state.
	if wh.Secret != nil {
		props.Secret = *wh.Secret
	}
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.EndpointID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (webhookHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	wh, err := traceAPICall(apiCtx, WebhookResourceType, "read", "Webhooks.Get", func(ctx context.Context) (*ts.Webhook, error) {
		return c.Webhooks().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if wh == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	props := webhookFrom(wh)
	b, _ := json.Marshal(props)
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (webhookHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseWebhookProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	wh, err := traceAPICall(apiCtx, WebhookResourceType, "update", "Webhooks.Update", func(ctx context.Context) (*ts.Webhook, error) {
		return c.Webhooks().Update(ctx, req.NativeID, webhookSubscriptionTypes(props.Subscriptions))
	})
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props = webhookFrom(wh)
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (webhookHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, WebhookResourceType, "delete", "Webhooks.Delete", func(ctx context.Context) error {
		return c.Webhooks().Delete(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (webhookHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	webhooks, err := traceAPICall(apiCtx, WebhookResourceType, "list", "Webhooks.List", func(ctx context.Context) ([]ts.Webhook, error) {
		return c.Webhooks().List(ctx)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(webhooks))
	for _, wh := range webhooks {
		ids = append(ids, wh.EndpointID)
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}

func webhookSubscriptionTypes(in []string) []ts.WebhookSubscriptionType {
	out := make([]ts.WebhookSubscriptionType, 0, len(in))
	for _, s := range in {
		out = append(out, ts.WebhookSubscriptionType(s))
	}
	return out
}

func webhookSubscriptionStrings(in []ts.WebhookSubscriptionType) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func webhookFrom(wh *ts.Webhook) *WebhookProperties {
	if wh == nil {
		return nil
	}
	return &WebhookProperties{
		EndpointURL:      wh.EndpointURL,
		ProviderType:     string(wh.ProviderType),
		Subscriptions:    sortedStrings(webhookSubscriptionStrings(wh.Subscriptions)),
		EndpointID:       wh.EndpointID,
		CreatorLoginName: wh.CreatorLoginName,
		CreatedAt:        formatTime(wh.Created),
		LastModifiedAt:   formatTime(wh.LastModified),
		// Secret intentionally omitted: surfaced only on create.
	}
}
