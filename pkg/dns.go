// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() {
	register(DNSConfigurationResourceType, dnsConfigurationHandler{})
	register(DNSNameserversResourceType, dnsNameserversHandler{})
	register(DNSPreferencesResourceType, dnsPreferencesHandler{})
	register(DNSSearchPathsResourceType, dnsSearchPathsHandler{})
	register(DNSSplitNameserversResourceType, dnsSplitNameserversHandler{})
}

// ---------------------------------------------------------------------------
// DNS configuration (complete tailnet DNS, alpha endpoint)
// ---------------------------------------------------------------------------

type DNSResolverProperties struct {
	Address         string `json:"address,omitempty"`
	UseWithExitNode *bool  `json:"useWithExitNode,omitempty"`
}

type DNSConfigurationPreferencesProperties struct {
	OverrideLocalDNS *bool `json:"overrideLocalDNS,omitempty"`
	MagicDNS         *bool `json:"magicDNS,omitempty"`
}

// DNSConfigurationProperties models a partial tailnet DNS configuration. Each
// top-level section is optional: a nil slice/map (or nil *Preferences) means
// "leave the existing tailnet state for this section unchanged", and apply
// merges desired sections over the live configuration before writing. Within a
// provided section, nested boolean pointers follow the same rule.
type DNSConfigurationProperties struct {
	Tailnet     string                                 `json:"tailnet,omitempty"`
	Nameservers []DNSResolverProperties                `json:"nameservers,omitempty"`
	SplitDNS    map[string][]DNSResolverProperties     `json:"splitDNS,omitempty"`
	SearchPaths []string                               `json:"searchPaths,omitempty"`
	Preferences *DNSConfigurationPreferencesProperties `json:"preferences,omitempty"`
}

type dnsConfigurationHandler struct{}

func resolverFromAPI(r ts.DNSConfigurationResolver) DNSResolverProperties {
	out := DNSResolverProperties{Address: r.Address, UseWithExitNode: &r.UseWithExitNode}
	return out
}

func resolverToAPI(r DNSResolverProperties) ts.DNSConfigurationResolver {
	out := ts.DNSConfigurationResolver{Address: r.Address}
	if r.UseWithExitNode != nil {
		out.UseWithExitNode = *r.UseWithExitNode
	}
	return out
}

func dnsConfigurationFrom(c *ts.DNSConfiguration, nativeID string) *DNSConfigurationProperties {
	if c == nil {
		return nil
	}
	props := &DNSConfigurationProperties{Tailnet: nativeID, SearchPaths: sortedStrings(c.SearchPaths)}
	props.Nameservers = make([]DNSResolverProperties, 0, len(c.Nameservers))
	for _, r := range c.Nameservers {
		props.Nameservers = append(props.Nameservers, resolverFromAPI(r))
	}
	if len(c.SplitDNS) > 0 {
		props.SplitDNS = make(map[string][]DNSResolverProperties, len(c.SplitDNS))
		for domain, servers := range c.SplitDNS {
			mapped := make([]DNSResolverProperties, 0, len(servers))
			for _, s := range servers {
				mapped = append(mapped, resolverFromAPI(s))
			}
			props.SplitDNS[domain] = mapped
		}
	}
	prefs := c.Preferences
	props.Preferences = &DNSConfigurationPreferencesProperties{OverrideLocalDNS: &prefs.OverrideLocalDNS, MagicDNS: &prefs.MagicDNS}
	return props
}

// mergeDNSConfiguration overlays the desired (provided) sections of props onto
// the live configuration. Omitted sections (nil slice/map/pointer) are carried
// over from current so SetConfiguration's full-replace POST does not wipe them.
func mergeDNSConfiguration(current *ts.DNSConfiguration, props *DNSConfigurationProperties) ts.DNSConfiguration {
	cfg := ts.DNSConfiguration{}
	if current != nil {
		cfg = *current
	}
	if props.Nameservers != nil {
		cfg.Nameservers = make([]ts.DNSConfigurationResolver, 0, len(props.Nameservers))
		for _, r := range props.Nameservers {
			cfg.Nameservers = append(cfg.Nameservers, resolverToAPI(r))
		}
	}
	if props.SplitDNS != nil {
		cfg.SplitDNS = make(map[string][]ts.DNSConfigurationResolver, len(props.SplitDNS))
		for domain, servers := range props.SplitDNS {
			mapped := make([]ts.DNSConfigurationResolver, 0, len(servers))
			for _, s := range servers {
				mapped = append(mapped, resolverToAPI(s))
			}
			cfg.SplitDNS[domain] = mapped
		}
	}
	if props.SearchPaths != nil {
		cfg.SearchPaths = sortedStrings(props.SearchPaths)
	}
	if props.Preferences != nil {
		if props.Preferences.OverrideLocalDNS != nil {
			cfg.Preferences.OverrideLocalDNS = *props.Preferences.OverrideLocalDNS
		}
		if props.Preferences.MagicDNS != nil {
			cfg.Preferences.MagicDNS = *props.Preferences.MagicDNS
		}
	}
	return cfg
}

func (dnsConfigurationHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DNSConfigurationProperties) (*resource.ProgressResult, error) {
	// SetConfiguration is a full-replace POST, so omitted sections would wipe
	// existing tailnet state. Read the live configuration first and merge the
	// desired sections over it: a partial resource then only updates the
	// sections it declares.
	current, err := traceAPICall(ctx, DNSConfigurationResourceType, opLabel(op), "DNS.Configuration", func(ctx context.Context) (*ts.DNSConfiguration, error) {
		return c.DNS().Configuration(ctx)
	})
	if err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	cfg := mergeDNSConfiguration(current, props)
	if err := traceAPICallNoResult(ctx, DNSConfigurationResourceType, opLabel(op), "DNS.SetConfiguration", func(ctx context.Context) error {
		return c.DNS().SetConfiguration(ctx, cfg)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	// Normalize provided set-like lists so persisted write state is byte-identical
	// to read state for the sections this resource manages. Omitted sections stay
	// absent; formae treats them as provider-defaulted via hasProviderDefault.
	props.Tailnet = nativeID
	props.SearchPaths = sortedStrings(props.SearchPaths)
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h dnsConfigurationHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DNSConfigurationProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", fmt.Errorf("invalid dns configuration properties: %w", err).Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, singletonNativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (h dnsConfigurationHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	cfg, err := traceAPICall(apiCtx, DNSConfigurationResourceType, "read", "DNS.Configuration", func(ctx context.Context) (*ts.DNSConfiguration, error) {
		return c.DNS().Configuration(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	b, _ := json.Marshal(dnsConfigurationFrom(cfg, req.NativeID))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h dnsConfigurationHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DNSConfigurationProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, fmt.Errorf("invalid dns configuration properties: %w", err).Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (dnsConfigurationHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSConfigurationResourceType, "delete", "DNS.SetConfiguration", func(ctx context.Context) error {
		return c.DNS().SetConfiguration(ctx, ts.DNSConfiguration{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (dnsConfigurationHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}

// ---------------------------------------------------------------------------
// DNS nameservers (global nameserver list)
// ---------------------------------------------------------------------------

type DNSNameserversProperties struct {
	Tailnet     string   `json:"tailnet,omitempty"`
	Nameservers []string `json:"nameservers"`
}

type dnsNameserversHandler struct{}

func (h dnsNameserversHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DNSNameserversProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid dns nameservers properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSNameserversResourceType, "create", "DNS.SetNameservers", func(ctx context.Context) error {
		return c.DNS().SetNameservers(ctx, props.Nameservers)
	}); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, singletonNativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props.Tailnet = singletonNativeID
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, singletonNativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (dnsNameserversHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	ns, err := traceAPICall(apiCtx, DNSNameserversResourceType, "read", "DNS.Nameservers", func(ctx context.Context) ([]string, error) {
		return c.DNS().Nameservers(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	b, _ := json.Marshal(DNSNameserversProperties{Tailnet: req.NativeID, Nameservers: sortedStrings(ns)})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h dnsNameserversHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DNSNameserversProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid dns nameservers properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSNameserversResourceType, "update", "DNS.SetNameservers", func(ctx context.Context) error {
		return c.DNS().SetNameservers(ctx, props.Nameservers)
	}); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props.Tailnet = req.NativeID
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (dnsNameserversHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSNameserversResourceType, "delete", "DNS.SetNameservers", func(ctx context.Context) error {
		return c.DNS().SetNameservers(ctx, []string{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (dnsNameserversHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}

// ---------------------------------------------------------------------------
// DNS preferences (MagicDNS toggle)
// ---------------------------------------------------------------------------

type DNSPreferencesProperties struct {
	Tailnet  string `json:"tailnet,omitempty"`
	MagicDNS bool   `json:"magicDNS"`
}

type dnsPreferencesHandler struct{}

func (h dnsPreferencesHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DNSPreferencesProperties) (*resource.ProgressResult, error) {
	if err := traceAPICallNoResult(ctx, DNSPreferencesResourceType, opLabel(op), "DNS.SetPreferences", func(ctx context.Context) error {
		return c.DNS().SetPreferences(ctx, ts.DNSPreferences{MagicDNS: props.MagicDNS})
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	props.Tailnet = nativeID
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h dnsPreferencesHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DNSPreferencesProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid dns preferences properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, singletonNativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (dnsPreferencesHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	prefs, err := traceAPICall(apiCtx, DNSPreferencesResourceType, "read", "DNS.Preferences", func(ctx context.Context) (*ts.DNSPreferences, error) {
		return c.DNS().Preferences(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	magic := false
	if prefs != nil {
		magic = prefs.MagicDNS
	}
	b, _ := json.Marshal(DNSPreferencesProperties{Tailnet: req.NativeID, MagicDNS: magic})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h dnsPreferencesHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DNSPreferencesProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid dns preferences properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (dnsPreferencesHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSPreferencesResourceType, "delete", "DNS.SetPreferences", func(ctx context.Context) error {
		return c.DNS().SetPreferences(ctx, ts.DNSPreferences{MagicDNS: false})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (dnsPreferencesHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}

// ---------------------------------------------------------------------------
// DNS search paths
// ---------------------------------------------------------------------------

type DNSSearchPathsProperties struct {
	Tailnet     string   `json:"tailnet,omitempty"`
	SearchPaths []string `json:"searchPaths"`
}

type dnsSearchPathsHandler struct{}

func (h dnsSearchPathsHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DNSSearchPathsProperties) (*resource.ProgressResult, error) {
	if err := traceAPICallNoResult(ctx, DNSSearchPathsResourceType, opLabel(op), "DNS.SetSearchPaths", func(ctx context.Context) error {
		return c.DNS().SetSearchPaths(ctx, props.SearchPaths)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	props.Tailnet = nativeID
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h dnsSearchPathsHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DNSSearchPathsProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid dns search paths properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, singletonNativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (dnsSearchPathsHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	paths, err := traceAPICall(apiCtx, DNSSearchPathsResourceType, "read", "DNS.SearchPaths", func(ctx context.Context) ([]string, error) {
		return c.DNS().SearchPaths(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	b, _ := json.Marshal(DNSSearchPathsProperties{Tailnet: req.NativeID, SearchPaths: sortedStrings(paths)})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h dnsSearchPathsHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DNSSearchPathsProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid dns search paths properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (dnsSearchPathsHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSSearchPathsResourceType, "delete", "DNS.SetSearchPaths", func(ctx context.Context) error {
		return c.DNS().SetSearchPaths(ctx, []string{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (dnsSearchPathsHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}

// ---------------------------------------------------------------------------
// DNS split nameservers (domain -> nameservers map)
// ---------------------------------------------------------------------------

type DNSSplitNameserversProperties struct {
	Tailnet  string              `json:"tailnet,omitempty"`
	SplitDNS map[string][]string `json:"splitDNS"`
}

// sortedSplitDNS returns a copy of resp with each domain's resolver list sorted
// for stable read output. Map keys are already emitted deterministically by
// encoding/json.
func sortedSplitDNS(resp ts.SplitDNSResponse) map[string][]string {
	if resp == nil {
		return nil
	}
	out := make(map[string][]string, len(resp))
	for domain, servers := range resp {
		out[domain] = sortedStrings(servers)
	}
	return out
}

type dnsSplitNameserversHandler struct{}

func (h dnsSplitNameserversHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DNSSplitNameserversProperties) (*resource.ProgressResult, error) {
	req := ts.SplitDNSRequest(props.SplitDNS)
	if req == nil {
		req = ts.SplitDNSRequest{}
	}
	if err := traceAPICallNoResult(ctx, DNSSplitNameserversResourceType, opLabel(op), "DNS.SetSplitDNS", func(ctx context.Context) error {
		return c.DNS().SetSplitDNS(ctx, req)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	props.Tailnet = nativeID
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h dnsSplitNameserversHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DNSSplitNameserversProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid dns split nameservers properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, singletonNativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (dnsSplitNameserversHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	resp, err := traceAPICall(apiCtx, DNSSplitNameserversResourceType, "read", "DNS.SplitDNS", func(ctx context.Context) (ts.SplitDNSResponse, error) {
		return c.DNS().SplitDNS(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	b, _ := json.Marshal(DNSSplitNameserversProperties{Tailnet: req.NativeID, SplitDNS: sortedSplitDNS(resp)})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h dnsSplitNameserversHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DNSSplitNameserversProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid dns split nameservers properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (dnsSplitNameserversHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DNSSplitNameserversResourceType, "delete", "DNS.SetSplitDNS", func(ctx context.Context) error {
		return c.DNS().SetSplitDNS(ctx, ts.SplitDNSRequest{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (dnsSplitNameserversHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}
