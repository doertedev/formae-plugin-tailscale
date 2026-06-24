// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

//go:build integration && !conformance

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const integrationPrefix = "formae-test-"

var integrationRunID = strconv.FormatInt(time.Now().Unix(), 10)

func TestMain(m *testing.M) {
	if os.Getenv("TAILSCALE_INTEGRATION") != "1" {
		fmt.Println("integration: skipping (set TAILSCALE_INTEGRATION=1 to enable)")
		os.Exit(0)
	}
	if !hasLiveCredentials() {
		fmt.Println("integration: skipping (TAILSCALE_API_KEY or OAuth credentials are empty)")
		os.Exit(0)
	}

	sweepIntegration("pre-suite")
	code := m.Run()
	sweepIntegration("post-suite")
	os.Exit(code)
}

func TestIntegration_Preflight(t *testing.T) {
	p := livePlugin(t)
	cfg := liveTargetConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	keys, _, err := p.getClient(cfg)
	if err != nil {
		t.Fatalf("get client: %v", err)
	}
	if _, err := keys.Keys().List(ctx, true); err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if _, err := keys.Webhooks().List(ctx); err != nil {
		t.Fatalf("list webhooks: %v", err)
	}
}

func TestIntegration_AuthKeyLifecycle(t *testing.T) {
	p := livePlugin(t)
	cfg := liveTargetConfig(t)
	desc := liveName("auth-key")

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: AuthKeyResourceType,
		Properties:   rawJSON(t, map[string]any{"reusable": true, "ephemeral": false, "preauthorized": false, "expirySeconds": 3600, "description": desc}),
		TargetConfig: cfg,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	id := create.ProgressResult.NativeID
	mustDeleteResource(t, p, cfg, AuthKeyResourceType, id)

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: AuthKeyResourceType, NativeID: id, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != "" {
		t.Fatalf("read error code: %s", read.ErrorCode)
	}
	var props AuthKeyProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.Description != desc {
		t.Fatalf("description: want %q, got %q", desc, props.Description)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: AuthKeyResourceType, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !contains(list.NativeIDs, id) {
		t.Fatalf("list did not include auth key %q: %v", id, list.NativeIDs)
	}

	update, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      AuthKeyResourceType,
		NativeID:          id,
		DesiredProperties: rawJSON(t, map[string]any{"description": desc + "-updated"}),
		TargetConfig:      cfg,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if update.ProgressResult.ErrorCode != resource.OperationErrorCodeNotUpdatable {
		t.Fatalf("auth key update error: want NotUpdatable, got %q", update.ProgressResult.ErrorCode)
	}

	deleteResourceNow(t, p, cfg, AuthKeyResourceType, id)
}

func TestIntegration_OAuthClientLifecycle(t *testing.T) {
	p := livePlugin(t)
	cfg := liveTargetConfig(t)
	desc := liveName("oauth-client")

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: OAuthClientResourceType,
		Properties:   rawJSON(t, map[string]any{"description": desc, "scopes": []string{"auth_keys:read"}}),
		TargetConfig: cfg,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	id := create.ProgressResult.NativeID
	mustDeleteResource(t, p, cfg, OAuthClientResourceType, id)

	update, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      OAuthClientResourceType,
		NativeID:          id,
		DesiredProperties: rawJSON(t, map[string]any{"description": desc + "-updated", "scopes": []string{"auth_keys:read"}}),
		TargetConfig:      cfg,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireSuccess(t, update.ProgressResult)

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: OAuthClientResourceType, NativeID: id, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != "" {
		t.Fatalf("read error code: %s", read.ErrorCode)
	}
	var props OAuthClientProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.Description != desc+"-updated" {
		t.Fatalf("description: want %q, got %q", desc+"-updated", props.Description)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: OAuthClientResourceType, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !contains(list.NativeIDs, id) {
		t.Fatalf("list did not include oauth client %q: %v", id, list.NativeIDs)
	}

	deleteResourceNow(t, p, cfg, OAuthClientResourceType, id)
}

func TestIntegration_WebhookLifecycle(t *testing.T) {
	p := livePlugin(t)
	cfg := liveTargetConfig(t)
	token := liveName("webhook")
	endpoint := "https://example.com/" + token

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: WebhookResourceType,
		Properties:   rawJSON(t, map[string]any{"endpointUrl": endpoint, "providerType": "slack", "subscriptions": []string{"nodeCreated"}}),
		TargetConfig: cfg,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	id := create.ProgressResult.NativeID
	mustDeleteResource(t, p, cfg, WebhookResourceType, id)

	update, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      WebhookResourceType,
		NativeID:          id,
		DesiredProperties: rawJSON(t, map[string]any{"endpointUrl": endpoint, "providerType": "slack", "subscriptions": []string{"nodeCreated", "nodeDeleted"}}),
		TargetConfig:      cfg,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireSuccess(t, update.ProgressResult)

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: WebhookResourceType, NativeID: id, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != "" {
		t.Fatalf("read error code: %s", read.ErrorCode)
	}
	var props WebhookProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.EndpointURL != endpoint {
		t.Fatalf("endpointUrl: want %q, got %q", endpoint, props.EndpointURL)
	}
	if !contains(props.Subscriptions, "nodeDeleted") {
		t.Fatalf("subscriptions did not include nodeDeleted: %v", props.Subscriptions)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: WebhookResourceType, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !contains(list.NativeIDs, id) {
		t.Fatalf("list did not include webhook %q: %v", id, list.NativeIDs)
	}

	deleteResourceNow(t, p, cfg, WebhookResourceType, id)
}

func TestIntegration_ServiceLifecycle(t *testing.T) {
	p := livePlugin(t)
	cfg := liveTargetConfig(t)
	// Tailscale requires service names to be prefixed with "svc:".
	name := "svc:" + liveName("service")

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ServiceResourceType,
		Properties:   rawJSON(t, map[string]any{"name": name, "comment": liveName("service"), "ports": []string{"443"}}),
		TargetConfig: cfg,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	id := create.ProgressResult.NativeID
	mustDeleteResource(t, p, cfg, ServiceResourceType, id)

	update, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      ServiceResourceType,
		NativeID:          id,
		DesiredProperties: rawJSON(t, map[string]any{"name": name, "comment": liveName("service-updated"), "ports": []string{"443", "80"}}),
		TargetConfig:      cfg,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireSuccess(t, update.ProgressResult)

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ServiceResourceType, NativeID: id, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != "" {
		t.Fatalf("read error code: %s", read.ErrorCode)
	}
	var props ServiceProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if len(props.Ports) != 2 {
		t.Fatalf("ports: want 2 got %d (%v)", len(props.Ports), props.Ports)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: ServiceResourceType, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !contains(list.NativeIDs, id) {
		t.Fatalf("list did not include service %q: %v", id, list.NativeIDs)
	}

	deleteResourceNow(t, p, cfg, ServiceResourceType, id)
}

func livePlugin(t *testing.T) *Plugin {
	t.Helper()
	p := &Plugin{}
	if _, _, err := p.getClient(liveTargetConfig(t)); err != nil {
		t.Fatalf("get client: %v", err)
	}
	return p
}

func liveTargetConfig(t *testing.T) json.RawMessage {
	t.Helper()
	return rawJSON(t, liveTargetConfigMap())
}

// liveTargetConfigMap builds the tailnet target config from environment
// variables. Shared by liveTargetConfig (test-scoped) and the pre/post-suite
// sweep (which runs from TestMain without a *testing.T).
func liveTargetConfigMap() map[string]any {
	cfg := map[string]any{"tailnet": envDefault("TAILSCALE_TAILNET", "-")}
	if apiKey := os.Getenv("TAILSCALE_API_KEY"); apiKey != "" {
		cfg["apiKey"] = apiKey
	} else {
		cfg["oauthClientID"] = os.Getenv("TAILSCALE_OAUTH_CLIENT_ID")
		cfg["oauthClientSecret"] = os.Getenv("TAILSCALE_OAUTH_CLIENT_SECRET")
		cfg["oauthScopes"] = splitScopes(os.Getenv("TAILSCALE_OAUTH_SCOPES"))
	}
	if baseURL := os.Getenv("TAILSCALE_BASE_URL"); baseURL != "" {
		cfg["baseUrl"] = baseURL
	}
	return cfg
}

func hasLiveCredentials() bool {
	return os.Getenv("TAILSCALE_API_KEY") != "" ||
		(os.Getenv("TAILSCALE_OAUTH_CLIENT_ID") != "" && os.Getenv("TAILSCALE_OAUTH_CLIENT_SECRET") != "")
}

func liveName(kind string) string {
	return integrationPrefix + integrationRunID + "-" + kind
}

func splitScopes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			scopes = append(scopes, part)
		}
	}
	return scopes
}

func mustDeleteResource(t *testing.T, p *Plugin, cfg json.RawMessage, resourceType, id string) {
	t.Helper()
	t.Cleanup(func() {
		deleteResourceNow(t, p, cfg, resourceType, id)
	})
}

func deleteResourceNow(t *testing.T, p *Plugin, cfg json.RawMessage, resourceType, id string) {
	t.Helper()
	res, err := p.Delete(context.Background(), &resource.DeleteRequest{ResourceType: resourceType, NativeID: id, TargetConfig: cfg})
	if err != nil {
		t.Fatalf("delete %s %s: %v", resourceType, id, err)
	}
	if res.ProgressResult.OperationStatus == resource.OperationStatusSuccess ||
		res.ProgressResult.ErrorCode == resource.OperationErrorCodeNotFound {
		return
	}
	t.Fatalf("delete %s %s failed: code=%s message=%s", resourceType, id, res.ProgressResult.ErrorCode, res.ProgressResult.StatusMessage)
}

func sweepIntegration(phase string) {
	p := &Plugin{}
	cfg := liveTargetConfigForSweep()
	client, _, err := p.getClient(cfg)
	if err != nil {
		fmt.Printf("integration sweep (%s): get client: %v\n", phase, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	keys, err := client.Keys().List(ctx, true)
	if err != nil {
		fmt.Printf("integration sweep (%s): list keys: %v\n", phase, err)
	} else {
		for _, key := range keys {
			if (key.KeyType == "auth" || key.KeyType == "client" || key.KeyType == "federated") && strings.HasPrefix(key.Description, integrationPrefix) {
				if err := client.Keys().Delete(ctx, key.ID); err != nil {
					fmt.Printf("integration sweep (%s): delete key %s: %v\n", phase, key.ID, err)
				}
			}
		}
	}

	webhooks, err := client.Webhooks().List(ctx)
	if err != nil {
		fmt.Printf("integration sweep (%s): list webhooks: %v\n", phase, err)
	} else {
		for _, wh := range webhooks {
			if strings.Contains(wh.EndpointURL, integrationPrefix) {
				if err := client.Webhooks().Delete(ctx, wh.EndpointID); err != nil {
					fmt.Printf("integration sweep (%s): delete webhook %s: %v\n", phase, wh.EndpointID, err)
				}
			}
		}
	}

	services, err := client.Services().List(ctx)
	if err != nil {
		fmt.Printf("integration sweep (%s): list services: %v\n", phase, err)
	} else {
		for _, svc := range services {
			// Service names are "svc:"-prefixed, so match the test prefix as a substring.
			if strings.Contains(svc.Name, integrationPrefix) || strings.HasPrefix(svc.Comment, integrationPrefix) {
				if err := client.Services().Delete(ctx, svc.Name); err != nil {
					fmt.Printf("integration sweep (%s): delete service %s: %v\n", phase, svc.Name, err)
				}
			}
		}
	}

	integrations, err := client.DevicePosture().ListIntegrations(ctx)
	if err != nil {
		fmt.Printf("integration sweep (%s): list posture integrations: %v\n", phase, err)
	} else {
		for _, intg := range integrations {
			if strings.Contains(intg.CloudID, integrationPrefix) || strings.Contains(intg.ClientID, integrationPrefix) {
				if err := client.DevicePosture().DeleteIntegration(ctx, intg.ID); err != nil {
					fmt.Printf("integration sweep (%s): delete posture integration %s: %v\n", phase, intg.ID, err)
				}
			}
		}
	}
}

func liveTargetConfigForSweep() json.RawMessage {
	b, _ := json.Marshal(liveTargetConfigMap())
	return b
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func contains[T comparable](items []T, want T) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
