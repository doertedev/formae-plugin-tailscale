// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package testcleanup

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	ts "tailscale.com/client/tailscale/v2"
)

// Main runs the Tailscale test-resource cleanup command and returns a process
// exit code for the small scripts/ci runner.
func Main() int {
	cfg, err := configFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: %v\n", err)
		return 1
	}

	prefixes := cleanupPrefixes()
	fmt.Printf("tailscale cleanup: prefixes=%s tailnet=%q\n", strings.Join(prefixes, ","), cfg.tailnet)

	client, err := cfg.client()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutFromEnv("TAILSCALE_CLEANUP_TIMEOUT", 30*time.Second))
	defer cancel()

	var failed bool
	if err := cleanupKeys(ctx, client, prefixes); err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: keys: %v\n", err)
		failed = true
	}
	if err := cleanupWebhooks(ctx, client, prefixes); err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: webhooks: %v\n", err)
		failed = true
	}
	if err := cleanupServices(ctx, client, prefixes); err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: services: %v\n", err)
		failed = true
	}
	if err := cleanupPostureIntegrations(ctx, client, prefixes); err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: posture integrations: %v\n", err)
		failed = true
	}
	if failed {
		return 1
	}
	fmt.Println("tailscale cleanup: complete")
	return 0
}

type cleanupConfig struct {
	apiKey            string
	oauthClientID     string
	oauthClientSecret string
	oauthScopes       []string
	tailnet           string
	baseURL           string
}

func configFromEnv() (cleanupConfig, error) {
	cfg := cleanupConfig{
		apiKey:            os.Getenv("TAILSCALE_API_KEY"),
		oauthClientID:     os.Getenv("TAILSCALE_OAUTH_CLIENT_ID"),
		oauthClientSecret: os.Getenv("TAILSCALE_OAUTH_CLIENT_SECRET"),
		oauthScopes:       splitList(os.Getenv("TAILSCALE_OAUTH_SCOPES")),
		tailnet:           envDefault("TAILSCALE_TAILNET", "-"),
		baseURL:           os.Getenv("TAILSCALE_BASE_URL"),
	}
	if cfg.apiKey == "" && (cfg.oauthClientID == "" || cfg.oauthClientSecret == "") {
		return cfg, fmt.Errorf("missing credentials: set TAILSCALE_API_KEY or TAILSCALE_OAUTH_CLIENT_ID/TAILSCALE_OAUTH_CLIENT_SECRET")
	}
	return cfg, nil
}

func (c cleanupConfig) client() (*ts.Client, error) {
	var baseURL *url.URL
	if c.baseURL != "" {
		parsed, err := url.Parse(c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid TAILSCALE_BASE_URL: %w", err)
		}
		baseURL = parsed
	}
	client := &ts.Client{
		BaseURL:   baseURL,
		UserAgent: "formae-plugin-tailscale-cleanup/0.1.0",
		APIKey:    c.apiKey,
		Tailnet:   c.tailnet,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
	if c.oauthClientID != "" && c.oauthClientSecret != "" {
		client.APIKey = ""
		client.Auth = &ts.OAuth{
			ClientID:     c.oauthClientID,
			ClientSecret: c.oauthClientSecret,
			Scopes:       c.oauthScopes,
		}
	}
	return client, nil
}

func cleanupKeys(ctx context.Context, client *ts.Client, prefixes []string) error {
	keys, err := client.Keys().List(ctx, true)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, key := range keys {
		// auth keys, OAuth clients, and federated identities are all "keys" in the
		// API and are swept when their description matches a test prefix.
		if key.KeyType != "auth" && key.KeyType != "client" && key.KeyType != "federated" {
			continue
		}
		if !hasAnyPrefix(key.Description, prefixes) {
			continue
		}
		fmt.Printf("tailscale cleanup: deleting %s key %s (%q)\n", key.KeyType, key.ID, key.Description)
		if err := client.Keys().Delete(ctx, key.ID); err != nil {
			fmt.Fprintf(os.Stderr, "tailscale cleanup: delete key %s: %v\n", key.ID, err)
		}
	}
	return nil
}

func cleanupWebhooks(ctx context.Context, client *ts.Client, prefixes []string) error {
	webhooks, err := client.Webhooks().List(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, wh := range webhooks {
		if !containsAny(wh.EndpointURL, prefixes) {
			continue
		}
		fmt.Printf("tailscale cleanup: deleting webhook %s (%q)\n", wh.EndpointID, wh.EndpointURL)
		if err := client.Webhooks().Delete(ctx, wh.EndpointID); err != nil {
			fmt.Fprintf(os.Stderr, "tailscale cleanup: delete webhook %s: %v\n", wh.EndpointID, err)
		}
	}
	return nil
}

func cleanupServices(ctx context.Context, client *ts.Client, prefixes []string) error {
	services, err := client.Services().List(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, svc := range services {
		// Services are swept when the service name or comment carries a test prefix.
		// Name matching is substring-based because Tailscale prefixes names with "svc:".
		if !containsAny(svc.Name, prefixes) && !hasAnyPrefix(svc.Comment, prefixes) {
			continue
		}
		fmt.Printf("tailscale cleanup: deleting service %s (%q)\n", svc.Name, svc.Comment)
		if err := client.Services().Delete(ctx, svc.Name); err != nil {
			fmt.Fprintf(os.Stderr, "tailscale cleanup: delete service %s: %v\n", svc.Name, err)
		}
	}
	return nil
}

func cleanupPostureIntegrations(ctx context.Context, client *ts.Client, prefixes []string) error {
	integrations, err := client.DevicePosture().ListIntegrations(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, intg := range integrations {
		// Posture integrations lack a free-form description, so the test prefix is
		// expected in the cloudId (or clientId) by convention.
		if !containsAny(intg.CloudID, prefixes) && !containsAny(intg.ClientID, prefixes) {
			continue
		}
		fmt.Printf("tailscale cleanup: deleting posture integration %s (%s)\n", intg.ID, intg.CloudID)
		if err := client.DevicePosture().DeleteIntegration(ctx, intg.ID); err != nil {
			fmt.Fprintf(os.Stderr, "tailscale cleanup: delete posture integration %s: %v\n", intg.ID, err)
		}
	}
	return nil
}

func cleanupPrefixes() []string {
	raw := os.Getenv("TAILSCALE_CLEANUP_PREFIXES")
	if raw == "" {
		raw = os.Getenv("TEST_PREFIX")
	}
	if raw == "" {
		raw = "formae-test-"
	}
	return splitList(raw)
}

func splitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func timeoutFromEnv(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tailscale cleanup: ignoring invalid %s=%q: %v\n", name, raw, err)
		return fallback
	}
	return d
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
