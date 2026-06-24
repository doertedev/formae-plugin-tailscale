// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/platform-engineering-labs/formae/pkg/model"
	"github.com/platform-engineering-labs/formae/pkg/plugin"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

type resourceHandler interface {
	create(context.Context, tailscaleAPI, *resource.CreateRequest) (*resource.CreateResult, error)
	read(context.Context, tailscaleAPI, *resource.ReadRequest) (*resource.ReadResult, error)
	update(context.Context, tailscaleAPI, *resource.UpdateRequest) (*resource.UpdateResult, error)
	delete(context.Context, tailscaleAPI, *resource.DeleteRequest) (*resource.DeleteResult, error)
	list(context.Context, tailscaleAPI, *resource.ListRequest) (*resource.ListResult, error)
}

var handlers = map[string]resourceHandler{}

func register(resourceType string, h resourceHandler) {
	if _, dup := handlers[resourceType]; dup {
		panic("formae-plugin-tailscale: duplicate handler for " + resourceType)
	}
	handlers[resourceType] = h
}

type keysAPI interface {
	CreateAuthKey(context.Context, ts.CreateKeyRequest) (*ts.Key, error)
	CreateOAuthClient(context.Context, ts.CreateOAuthClientRequest) (*ts.Key, error)
	SetOAuthClient(context.Context, string, ts.SetOAuthClientRequest) (*ts.Key, error)
	CreateFederatedIdentity(context.Context, ts.CreateFederatedIdentityRequest) (*ts.Key, error)
	SetFederatedIdentity(context.Context, string, ts.SetFederatedIdentityRequest) (*ts.Key, error)
	Get(context.Context, string) (*ts.Key, error)
	List(context.Context, bool) ([]ts.Key, error)
	Delete(context.Context, string) error
}

type webhooksAPI interface {
	Create(context.Context, ts.CreateWebhookRequest) (*ts.Webhook, error)
	Get(context.Context, string) (*ts.Webhook, error)
	List(context.Context) ([]ts.Webhook, error)
	Update(context.Context, string, []ts.WebhookSubscriptionType) (*ts.Webhook, error)
	Delete(context.Context, string) error
}

type policyFileAPI interface {
	Get(context.Context) (*ts.ACL, error)
	Raw(context.Context) (*ts.RawACL, error)
	Set(context.Context, any, string) error
	Validate(context.Context, any) error
}

type tailnetSettingsAPI interface {
	Get(context.Context) (*ts.TailnetSettings, error)
	Update(context.Context, ts.UpdateTailnetSettingsRequest) error
}

type contactsAPI interface {
	Get(context.Context) (*ts.Contacts, error)
	Update(context.Context, ts.ContactType, ts.UpdateContactRequest) error
}

type dnsAPI interface {
	Configuration(context.Context) (*ts.DNSConfiguration, error)
	SetConfiguration(context.Context, ts.DNSConfiguration) error
	Nameservers(context.Context) ([]string, error)
	SetNameservers(context.Context, []string) error
	Preferences(context.Context) (*ts.DNSPreferences, error)
	SetPreferences(context.Context, ts.DNSPreferences) error
	SearchPaths(context.Context) ([]string, error)
	SetSearchPaths(context.Context, []string) error
	SplitDNS(context.Context) (ts.SplitDNSResponse, error)
	SetSplitDNS(context.Context, ts.SplitDNSRequest) error
}

type devicesAPI interface {
	List(context.Context, ...ts.ListDevicesOptions) ([]ts.Device, error)
	Get(context.Context, string) (*ts.Device, error)
	GetWithAllFields(context.Context, string) (*ts.Device, error)
	SetAuthorized(context.Context, string, bool) error
	SetKey(context.Context, string, ts.DeviceKey) error
	SetSubnetRoutes(context.Context, string, []string) error
	SubnetRoutes(context.Context, string) (*ts.DeviceRoutes, error)
	SetTags(context.Context, string, []string) error
	Delete(context.Context, string) error
}

type servicesAPI interface {
	List(context.Context) ([]ts.Service, error)
	Get(context.Context, string) (*ts.Service, error)
	CreateOrUpdate(context.Context, ts.Service) error
	Delete(context.Context, string) error
}

type loggingAPI interface {
	LogstreamConfiguration(context.Context, ts.LogType) (*ts.LogstreamConfiguration, error)
	SetLogstreamConfiguration(context.Context, ts.LogType, ts.SetLogstreamConfigurationRequest) error
	DeleteLogstreamConfiguration(context.Context, ts.LogType) error
	CreateOrGetAwsExternalId(context.Context, bool) (*ts.AWSExternalID, error)
}

type devicePostureAPI interface {
	ListIntegrations(context.Context) ([]ts.PostureIntegration, error)
	CreateIntegration(context.Context, ts.CreatePostureIntegrationRequest) (*ts.PostureIntegration, error)
	UpdateIntegration(context.Context, string, ts.UpdatePostureIntegrationRequest) (*ts.PostureIntegration, error)
	DeleteIntegration(context.Context, string) error
	GetIntegration(context.Context, string) (*ts.PostureIntegration, error)
}

type usersAPI interface {
	List(context.Context, *ts.UserType, *ts.UserRole) ([]ts.User, error)
	Get(context.Context, string) (*ts.User, error)
}

type tailscaleAPI interface {
	Keys() keysAPI
	Webhooks() webhooksAPI
	PolicyFile() policyFileAPI
	TailnetSettings() tailnetSettingsAPI
	Contacts() contactsAPI
	DNS() dnsAPI
	Devices() devicesAPI
	Services() servicesAPI
	Logging() loggingAPI
	DevicePosture() devicePostureAPI
	Users() usersAPI
}

type productionClient struct{ c *ts.Client }

func (p productionClient) Keys() keysAPI                       { return p.c.Keys() }
func (p productionClient) Webhooks() webhooksAPI               { return p.c.Webhooks() }
func (p productionClient) PolicyFile() policyFileAPI           { return p.c.PolicyFile() }
func (p productionClient) TailnetSettings() tailnetSettingsAPI { return p.c.TailnetSettings() }
func (p productionClient) Contacts() contactsAPI               { return p.c.Contacts() }
func (p productionClient) DNS() dnsAPI                         { return p.c.DNS() }
func (p productionClient) Devices() devicesAPI                 { return p.c.Devices() }
func (p productionClient) Services() servicesAPI               { return p.c.Services() }
func (p productionClient) Logging() loggingAPI                 { return p.c.Logging() }
func (p productionClient) DevicePosture() devicePostureAPI     { return p.c.DevicePosture() }
func (p productionClient) Users() usersAPI                     { return p.c.Users() }

var _ tailscaleAPI = productionClient{}

// singletonNativeID is the stable identifier used for tailnet-scoped singleton
// resources (ACL, tailnet settings, contacts, DNS settings). There is exactly
// one of each per tailnet, and a plugin client is bound to a single tailnet, so
// a fixed sentinel is sufficient and unambiguous within a resource type.
const singletonNativeID = "tailnet"

// singletonList is the shared list result for tailnet-scoped singleton
// resources: exactly one instance exists per tailnet, so discovery always
// reports the fixed sentinel native id.
func singletonList() *resource.ListResult {
	return &resource.ListResult{NativeIDs: []string{singletonNativeID}}
}

type Plugin struct {
	mu     sync.Mutex
	client tailscaleAPI
	cached *cachedClient
}

type cachedClient struct {
	key string
	api tailscaleAPI
}

var _ plugin.ResourcePlugin = &Plugin{}

func newPluginWithClient(c tailscaleAPI) *Plugin { return &Plugin{client: c} }

type targetConfig struct {
	APIKey            string   `json:"apiKey"`
	LegacyAPIKey      string   `json:"api_key"`
	OAuthClientID     string   `json:"oauthClientID"`
	LegacyOAuthID     string   `json:"oauth_client_id"`
	OAuthClientSecret string   `json:"oauthClientSecret"`
	LegacyOAuthSecret string   `json:"oauth_client_secret"`
	OAuthScopes       []string `json:"oauthScopes"`
	LegacyScopes      []string `json:"scopes"`
	Tailnet           string   `json:"tailnet"`
	BaseURL           string   `json:"baseUrl"`
	LegacyBaseURL     string   `json:"base_url"`
	APITimeoutSeconds int      `json:"apiTimeoutSeconds"`
}

func resolveTargetConfig(raw json.RawMessage) (targetConfig, error) {
	var cfg targetConfig
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("invalid tailscale target config JSON: %w", err)
		}
	}
	if cfg.APIKey == "" {
		cfg.APIKey = cfg.LegacyAPIKey
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("TAILSCALE_API_KEY")
	}
	if cfg.OAuthClientID == "" {
		cfg.OAuthClientID = cfg.LegacyOAuthID
	}
	if cfg.OAuthClientID == "" {
		cfg.OAuthClientID = os.Getenv("TAILSCALE_OAUTH_CLIENT_ID")
	}
	if cfg.OAuthClientSecret == "" {
		cfg.OAuthClientSecret = cfg.LegacyOAuthSecret
	}
	if cfg.OAuthClientSecret == "" {
		cfg.OAuthClientSecret = os.Getenv("TAILSCALE_OAUTH_CLIENT_SECRET")
	}
	if cfg.Tailnet == "" {
		cfg.Tailnet = os.Getenv("TAILSCALE_TAILNET")
	}
	if cfg.Tailnet == "" {
		cfg.Tailnet = "-"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.LegacyBaseURL
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("TAILSCALE_BASE_URL")
	}
	if len(cfg.OAuthScopes) == 0 {
		cfg.OAuthScopes = cfg.LegacyScopes
	}
	if cfg.APITimeoutSeconds <= 0 {
		if raw := os.Getenv("TAILSCALE_API_TIMEOUT_SECONDS"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				cfg.APITimeoutSeconds = v
			}
		}
	}
	if cfg.APIKey == "" && (cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "") {
		return cfg, errors.New("tailscale credentials not configured: set apiKey in the target config or TAILSCALE_API_KEY, or set OAuth client credentials")
	}
	return cfg, nil
}

func (p *Plugin) getClient(raw json.RawMessage) (tailscaleAPI, time.Duration, error) {
	if p.client != nil {
		tracef("client using injected test client")
		return p.client, defaultAPITimeout, nil
	}
	cfg, err := resolveTargetConfig(raw)
	if err != nil {
		return nil, 0, err
	}
	timeout := apiTimeoutFor(cfg)
	cacheKey := strings.Join([]string{cfg.APIKey, cfg.OAuthClientID, cfg.OAuthClientSecret, cfg.Tailnet, cfg.BaseURL, strings.Join(cfg.OAuthScopes, ","), timeout.String()}, "\x00")
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != nil && p.cached.key == cacheKey {
		tracef("client cache hit auth=%s tailnet=%q baseUrlSet=%t", authMode(cfg), cfg.Tailnet, cfg.BaseURL != "")
		return p.cached.api, timeout, nil
	}
	tracef("client constructing auth=%s tailnet=%q baseUrlSet=%t scopes=%d", authMode(cfg), cfg.Tailnet, cfg.BaseURL != "", len(cfg.OAuthScopes))
	var baseURL *url.URL
	if cfg.BaseURL != "" {
		parsed, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid tailscale baseUrl: %w", err)
		}
		baseURL = parsed
	}
	client := &ts.Client{
		BaseURL:   baseURL,
		UserAgent: "formae-plugin-tailscale/0.1.0",
		APIKey:    cfg.APIKey,
		Tailnet:   cfg.Tailnet,
		HTTP:      &http.Client{Timeout: timeout},
	}
	if cfg.OAuthClientID != "" && cfg.OAuthClientSecret != "" {
		client.APIKey = ""
		client.Auth = &ts.OAuth{
			ClientID:     cfg.OAuthClientID,
			ClientSecret: cfg.OAuthClientSecret,
			Scopes:       cfg.OAuthScopes,
		}
	}
	api := productionClient{c: client}
	p.cached = &cachedClient{key: cacheKey, api: api}
	return api, timeout, nil
}

func authMode(cfg targetConfig) string {
	if cfg.OAuthClientID != "" && cfg.OAuthClientSecret != "" {
		return "oauth"
	}
	if cfg.APIKey != "" {
		return "api-key"
	}
	return "none"
}

func (p *Plugin) RateLimit() model.RateLimitConfig {
	return model.RateLimitConfig{
		Scope:                            model.RateLimitScopeNamespace,
		MaxRequestsPerSecondForNamespace: 5,
	}
}

func (p *Plugin) DiscoveryFilters() []model.MatchFilter { return nil }

func (p *Plugin) LabelConfig() model.LabelConfig {
	return model.LabelConfig{
		DefaultQuery: "$.description",
		ResourceOverrides: map[string]string{
			AuthKeyResourceType:                "$.id",
			WebhookResourceType:                "$.endpointUrl",
			OAuthClientResourceType:            "$.description",
			FederatedIdentityResourceType:      "$.description",
			ACLResourceType:                    "$.tailnet",
			TailnetSettingsResourceType:        "$.tailnet",
			ContactsResourceType:               "$.tailnet",
			DNSConfigurationResourceType:       "$.tailnet",
			DNSNameserversResourceType:         "$.tailnet",
			DNSPreferencesResourceType:         "$.tailnet",
			DNSSearchPathsResourceType:         "$.tailnet",
			DNSSplitNameserversResourceType:    "$.tailnet",
			DeviceAuthorizationResourceType:    "$.deviceId",
			DeviceKeyResourceType:              "$.deviceId",
			DeviceSubnetRoutesResourceType:     "$.deviceId",
			DeviceTagsResourceType:             "$.deviceId",
			DeviceResourceType:                 "$.nodeId",
			ServiceResourceType:                "$.name",
			PostureIntegrationResourceType:     "$.cloudId",
			LogstreamConfigurationResourceType: "$.logType",
			AWSExternalIDResourceType:          "$.externalId",
			UserResourceType:                   "$.loginName",
		},
	}
}

func progress(op resource.Operation, status resource.OperationStatus, nativeID string) *resource.ProgressResult {
	return &resource.ProgressResult{Operation: op, OperationStatus: status, NativeID: nativeID}
}

func fail(op resource.Operation, nativeID, msg string, code resource.OperationErrorCode) *resource.ProgressResult {
	pr := progress(op, resource.OperationStatusFailure, nativeID)
	pr.ErrorCode = code
	pr.StatusMessage = msg
	return pr
}

// mapTailscaleError translates a Tailscale API error into a stable
// OperationErrorCode. err must be non-nil; all call sites guard on err != nil
// before invoking this. A non-nil error that is neither an APIError nor a
// context error falls through to ServiceInternalError.
func mapTailscaleError(err error) resource.OperationErrorCode {
	var apiErr ts.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusUnauthorized:
			return resource.OperationErrorCodeInvalidCredentials
		case http.StatusForbidden:
			return resource.OperationErrorCodeAccessDenied
		case http.StatusNotFound:
			return resource.OperationErrorCodeNotFound
		case http.StatusConflict:
			return resource.OperationErrorCodeResourceConflict
		case http.StatusTooManyRequests:
			return resource.OperationErrorCodeThrottling
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return resource.OperationErrorCodeInvalidRequest
		case http.StatusGatewayTimeout:
			return resource.OperationErrorCodeServiceTimeout
		default:
			if apiErr.Status >= 500 {
				return resource.OperationErrorCodeServiceInternalError
			}
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return resource.OperationErrorCodeServiceTimeout
	}
	return resource.OperationErrorCodeServiceInternalError
}

// defaultAPITimeout is the per-call Tailscale API timeout used when a target
// does not set apiTimeoutSeconds. It is generous enough for most CRUD calls;
// large tailnets may need a larger value via the config field.
const defaultAPITimeout = 15 * time.Second

// apiTimeoutFor resolves the per-call API timeout for a target config. It is
// used per request so that concurrent operations against multiple targets each
// observe their own configured deadline instead of sharing one global value.
func apiTimeoutFor(cfg targetConfig) time.Duration {
	if cfg.APITimeoutSeconds > 0 {
		return time.Duration(cfg.APITimeoutSeconds) * time.Second
	}
	return defaultAPITimeout
}

// apiTimeoutKey is the context value used to carry the per-target API timeout
// from the plugin entry points down to apiContext. Keeping it on the context
// (rather than a package-level variable) means each request derives its own
// bounded deadline even when one plugin instance serves several targets.
type apiTimeoutKey struct{}

func contextWithAPITimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, apiTimeoutKey{}, d)
}

func apiContext(ctx context.Context) (context.Context, context.CancelFunc) {
	d := defaultAPITimeout
	if v, ok := ctx.Value(apiTimeoutKey{}).(time.Duration); ok && v > 0 {
		d = v
	}
	return context.WithTimeout(ctx, d)
}

// marshalProperties serializes a properties struct for a progress result. Errors
// are ignored because the inputs are plain structs under plugin control.
func marshalProperties(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// formatTime renders a time.Time as RFC3339, returning "" for the zero value so
// omitted timestamps do not surface as misleading epoch values.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// opLabel returns a lowercase label for a resource operation, used in trace
// output so log lines read naturally (e.g. "api begin op=create ...").
func opLabel(op resource.Operation) string {
	switch op {
	case resource.OperationCreate:
		return "create"
	case resource.OperationUpdate:
		return "update"
	case resource.OperationDelete:
		return "delete"
	case resource.OperationCheckStatus:
		return "status"
	default:
		return "op"
	}
}

// sortedStrings returns a sorted copy of in. It is used to normalize set-like
// collections (tags, scopes, routes, subscriptions, DNS lists) on read so that
// repeated reads of the same state are byte-identical and not brittle to API
// reordering. The formae engine and conformance harness compare scalar arrays
// order-insensitively, so this is purely a stability improvement.
func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func (p *Plugin) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	done := traceOperation("create", req.ResourceType, "")
	defer done()
	client, timeout, err := p.getClient(req.TargetConfig)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInternalFailure)}, nil
	}
	ctx = contextWithAPITimeout(ctx, timeout)
	h, ok := handlers[req.ResourceType]
	if !ok {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "unsupported resource type: "+req.ResourceType, resource.OperationErrorCodeInvalidRequest)}, nil
	}
	return h.create(ctx, client, req)
}

func (p *Plugin) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	done := traceOperation("read", req.ResourceType, req.NativeID)
	defer done()
	client, timeout, err := p.getClient(req.TargetConfig)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInternalFailure}, nil
	}
	ctx = contextWithAPITimeout(ctx, timeout)
	h, ok := handlers[req.ResourceType]
	if !ok {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	return h.read(ctx, client, req)
}

func (p *Plugin) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	done := traceOperation("update", req.ResourceType, req.NativeID)
	defer done()
	client, timeout, err := p.getClient(req.TargetConfig)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInternalFailure)}, nil
	}
	ctx = contextWithAPITimeout(ctx, timeout)
	h, ok := handlers[req.ResourceType]
	if !ok {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "unsupported resource type: "+req.ResourceType, resource.OperationErrorCodeInvalidRequest)}, nil
	}
	return h.update(ctx, client, req)
}

func (p *Plugin) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	done := traceOperation("delete", req.ResourceType, req.NativeID)
	defer done()
	client, timeout, err := p.getClient(req.TargetConfig)
	if err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), resource.OperationErrorCodeInternalFailure)}, nil
	}
	ctx = contextWithAPITimeout(ctx, timeout)
	h, ok := handlers[req.ResourceType]
	if !ok {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, "unsupported resource type: "+req.ResourceType, resource.OperationErrorCodeInvalidRequest)}, nil
	}
	return h.delete(ctx, client, req)
}

// Status is a no-op success. Every Tailscale API call this plugin makes is
// synchronous (create/update/delete complete before returning), so there are
// no async resources to poll. The engine still calls Status between operations;
// reporting success here signals "ready".
func (p *Plugin) Status(_ context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	return &resource.StatusResult{ProgressResult: progress(resource.OperationCheckStatus, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (p *Plugin) List(ctx context.Context, req *resource.ListRequest) (*resource.ListResult, error) {
	done := traceOperation("list", req.ResourceType, "")
	defer done()
	client, timeout, err := p.getClient(req.TargetConfig)
	if err != nil {
		log.Printf("formae-plugin-tailscale: list %q skipped: %v", req.ResourceType, err)
		// Return an empty result rather than a Go error so the engine treats a
		// client-config failure the same way it treats an unsupported resource
		// type below — consistent with how Create/Read/Update/Delete surface
		// config failures as structured results.
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	ctx = contextWithAPITimeout(ctx, timeout)
	h, ok := handlers[req.ResourceType]
	if !ok {
		log.Printf("formae-plugin-tailscale: list %q skipped: unsupported resource type", req.ResourceType)
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	return h.list(ctx, client, req)
}

// traceEnabledCached avoids re-reading TAILSCALE_PLUGIN_DEBUG / FORMAE_PLUGIN_DEBUG
// on every trace call (tracef is on the hot path of every API call). The env is
// only consulted once at first use.
var traceEnabledCached = sync.OnceValue(func() bool {
	return os.Getenv("TAILSCALE_PLUGIN_DEBUG") != "" || os.Getenv("FORMAE_PLUGIN_DEBUG") != ""
})

func traceEnabled() bool {
	return traceEnabledCached()
}

func tracef(format string, args ...any) {
	if !traceEnabled() {
		return
	}
	log.Printf("formae-plugin-tailscale: "+format, args...)
}

func traceOperation(op, resourceType, nativeID string) func() {
	if !traceEnabled() {
		return func() {}
	}
	start := time.Now()
	if nativeID == "" {
		tracef("operation begin op=%s resourceType=%q", op, resourceType)
	} else {
		tracef("operation begin op=%s resourceType=%q nativeID=%q", op, resourceType, nativeID)
	}
	return func() {
		tracef("operation end op=%s resourceType=%q nativeID=%q elapsed=%s", op, resourceType, nativeID, time.Since(start).Round(time.Millisecond))
	}
}

func traceAPICall[T any](ctx context.Context, resourceType, op, call string, fn func(context.Context) (T, error)) (T, error) {
	if !traceEnabled() {
		return fn(ctx)
	}
	start := time.Now()
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		tracef("api begin op=%s resourceType=%q call=%s deadlineIn=%s", op, resourceType, call, time.Until(deadline).Round(time.Millisecond))
	} else {
		tracef("api begin op=%s resourceType=%q call=%s deadline=none", op, resourceType, call)
	}
	result, err := fn(ctx)
	if err != nil {
		tracef("api end op=%s resourceType=%q call=%s elapsed=%s error=%v", op, resourceType, call, time.Since(start).Round(time.Millisecond), err)
		return result, err
	}
	tracef("api end op=%s resourceType=%q call=%s elapsed=%s", op, resourceType, call, time.Since(start).Round(time.Millisecond))
	return result, nil
}

func traceAPICallNoResult(ctx context.Context, resourceType, op, call string, fn func(context.Context) error) error {
	_, err := traceAPICall(ctx, resourceType, op, call, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}
