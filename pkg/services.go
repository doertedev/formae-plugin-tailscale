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

func init() { register(ServiceResourceType, serviceHandler{}) }

// ServiceProperties models a Tailscale service (virtual IP service). The name is
// the immutable identifier; the remaining fields are the managed configuration.
type ServiceProperties struct {
	Name        string            `json:"name"`
	Comment     string            `json:"comment,omitempty"`
	Addrs       []string          `json:"addrs,omitempty"`
	Ports       []string          `json:"ports,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type serviceHandler struct{}

func serviceFrom(s *ts.Service) *ServiceProperties {
	if s == nil {
		return nil
	}
	return &ServiceProperties{
		Name:        s.Name,
		Comment:     s.Comment,
		Addrs:       sortedStrings(s.Addrs),
		Ports:       sortedStrings(s.Ports),
		Tags:        sortedStrings(s.Tags),
		Annotations: s.Annotations,
	}
}

func (serviceHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props ServiceProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid service properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.Name == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "service properties missing 'name'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, ServiceResourceType, "create", "Services.CreateOrUpdate", func(ctx context.Context) error {
		return c.Services().CreateOrUpdate(ctx, ts.Service{
			Name: props.Name, Comment: props.Comment, Addrs: props.Addrs,
			Ports: props.Ports, Tags: props.Tags, Annotations: props.Annotations,
		})
	}); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, props.Name, err.Error(), mapTailscaleError(err))}, nil
	}
	// Read back the canonical state so auto-allocated fields (notably addrs) are
	// captured for the caller and for subsequent drift detection.
	if current := serviceRead(apiCtx, c, props.Name); current != nil {
		props = *serviceFrom(current)
	}
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.Name)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

// serviceRead fetches a single service, returning nil on any error so callers
// can treat the post-write read-back as best-effort. ctx is expected to already
// be the bounded API context from the caller (apiContext); it is not re-derived
// here to avoid nesting two independent timeouts.
func serviceRead(ctx context.Context, c tailscaleAPI, name string) *ts.Service {
	svc, err := traceAPICall(ctx, ServiceResourceType, "read", "Services.Get", func(ctx context.Context) (*ts.Service, error) {
		return c.Services().Get(ctx, name)
	})
	if err != nil || svc == nil {
		return nil
	}
	return svc
}

func (serviceHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	svc, err := traceAPICall(apiCtx, ServiceResourceType, "read", "Services.Get", func(ctx context.Context) (*ts.Service, error) {
		return c.Services().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if svc == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(serviceFrom(svc))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (serviceHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props ServiceProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid service properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	name := req.NativeID
	if props.Name != "" && props.Name != name {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, fmt.Sprintf("service name is immutable: want %q got %q", name, props.Name), resource.OperationErrorCodeNotUpdatable)}, nil
	}
	props.Name = name
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	// Tailscale services must carry exactly two addresses. On create the API
	// auto-allocates them; on update, if the caller did not specify addrs, we
	// preserve the currently-allocated addresses so the PUT stays valid.
	if len(props.Addrs) == 0 {
		if current := serviceRead(apiCtx, c, name); current != nil {
			props.Addrs = current.Addrs
		}
	}
	if err := traceAPICallNoResult(apiCtx, ServiceResourceType, "update", "Services.CreateOrUpdate", func(ctx context.Context) error {
		return c.Services().CreateOrUpdate(ctx, ts.Service{
			Name: props.Name, Comment: props.Comment, Addrs: props.Addrs,
			Ports: props.Ports, Tags: props.Tags, Annotations: props.Annotations,
		})
	}); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	if current := serviceRead(apiCtx, c, name); current != nil {
		props = *serviceFrom(current)
	}
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (serviceHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, ServiceResourceType, "delete", "Services.Delete", func(ctx context.Context) error {
		return c.Services().Delete(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (serviceHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	svcs, err := traceAPICall(apiCtx, ServiceResourceType, "list", "Services.List", func(ctx context.Context) ([]ts.Service, error) {
		return c.Services().List(ctx)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(svcs))
	for _, s := range svcs {
		ids = append(ids, s.Name)
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
