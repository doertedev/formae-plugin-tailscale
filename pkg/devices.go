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
	register(DeviceAuthorizationResourceType, deviceAuthorizationHandler{})
	register(DeviceKeyResourceType, deviceKeyHandler{})
	register(DeviceSubnetRoutesResourceType, deviceSubnetRoutesHandler{})
	register(DeviceTagsResourceType, deviceTagsHandler{})
	register(DeviceResourceType, deviceHandler{})
}

// notSupported returns a failure result indicating the operation is not
// supported for the resource (used by read-only/inventory resources).
func notSupported(op resource.Operation, resourceType, nativeID string) *resource.ProgressResult {
	return fail(op, nativeID, resourceType+" does not support "+string(op), resource.OperationErrorCodeInvalidRequest)
}

// deviceIDMismatch fails an update when the caller supplies a deviceId that
// differs from the native id. The native id is the authoritative device handle;
// a mismatch signals a user error rather than a silent redirect.
func deviceIDMismatch(req *resource.UpdateRequest, propsDeviceID string) *resource.UpdateResult {
	if propsDeviceID != "" && propsDeviceID != req.NativeID {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, fmt.Sprintf("deviceId %q does not match nativeID %q", propsDeviceID, req.NativeID), resource.OperationErrorCodeInvalidRequest)}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Device authorization
// ---------------------------------------------------------------------------

type DeviceAuthorizationProperties struct {
	DeviceID   string `json:"deviceId"`
	Authorized bool   `json:"authorized"`
}

type deviceAuthorizationHandler struct{}

func (h deviceAuthorizationHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DeviceAuthorizationProperties) (*resource.ProgressResult, error) {
	if err := traceAPICallNoResult(ctx, DeviceAuthorizationResourceType, opLabel(op), "Devices.SetAuthorized", func(ctx context.Context) error {
		return c.Devices().SetAuthorized(ctx, nativeID, props.Authorized)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h deviceAuthorizationHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DeviceAuthorizationProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid device authorization properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.DeviceID == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "device authorization properties missing 'deviceId'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, props.DeviceID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (h deviceAuthorizationHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	dev, err := traceAPICall(apiCtx, DeviceAuthorizationResourceType, "read", "Devices.Get", func(ctx context.Context) (*ts.Device, error) {
		return c.Devices().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if dev == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(DeviceAuthorizationProperties{DeviceID: dev.NodeID, Authorized: dev.Authorized})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h deviceAuthorizationHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DeviceAuthorizationProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid device authorization properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if res := deviceIDMismatch(req, props.DeviceID); res != nil {
		return res, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete relinquishes management without mutating the authorization state, since
// a device's baseline authorization depends on tailnet-wide approval settings.
func (deviceAuthorizationHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (deviceAuthorizationHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: []string{}}, nil
}

// ---------------------------------------------------------------------------
// Device key (key expiry)
// ---------------------------------------------------------------------------

type DeviceKeyProperties struct {
	DeviceID          string `json:"deviceId"`
	KeyExpiryDisabled bool   `json:"keyExpiryDisabled"`
}

type deviceKeyHandler struct{}

func (h deviceKeyHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DeviceKeyProperties) (*resource.ProgressResult, error) {
	if err := traceAPICallNoResult(ctx, DeviceKeyResourceType, opLabel(op), "Devices.SetKey", func(ctx context.Context) error {
		return c.Devices().SetKey(ctx, nativeID, ts.DeviceKey{KeyExpiryDisabled: props.KeyExpiryDisabled})
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h deviceKeyHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DeviceKeyProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid device key properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.DeviceID == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "device key properties missing 'deviceId'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, props.DeviceID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (h deviceKeyHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	dev, err := traceAPICall(apiCtx, DeviceKeyResourceType, "read", "Devices.Get", func(ctx context.Context) (*ts.Device, error) {
		return c.Devices().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if dev == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(DeviceKeyProperties{DeviceID: dev.NodeID, KeyExpiryDisabled: dev.KeyExpiryDisabled})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h deviceKeyHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DeviceKeyProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid device key properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if res := deviceIDMismatch(req, props.DeviceID); res != nil {
		return res, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (deviceKeyHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (deviceKeyHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: []string{}}, nil
}

// ---------------------------------------------------------------------------
// Device subnet routes
// ---------------------------------------------------------------------------

type DeviceSubnetRoutesProperties struct {
	DeviceID         string   `json:"deviceId"`
	Routes           []string `json:"routes"`
	AdvertisedRoutes []string `json:"advertisedRoutes,omitempty"`
}

type deviceSubnetRoutesHandler struct{}

func (h deviceSubnetRoutesHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DeviceSubnetRoutesProperties) (*resource.ProgressResult, error) {
	routes := props.Routes
	if routes == nil {
		routes = []string{}
	}
	if err := traceAPICallNoResult(ctx, DeviceSubnetRoutesResourceType, opLabel(op), "Devices.SetSubnetRoutes", func(ctx context.Context) error {
		return c.Devices().SetSubnetRoutes(ctx, nativeID, routes)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h deviceSubnetRoutesHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DeviceSubnetRoutesProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid device subnet routes properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.DeviceID == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "device subnet routes properties missing 'deviceId'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, props.DeviceID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (deviceSubnetRoutesHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	routes, err := traceAPICall(apiCtx, DeviceSubnetRoutesResourceType, "read", "Devices.SubnetRoutes", func(ctx context.Context) (*ts.DeviceRoutes, error) {
		return c.Devices().SubnetRoutes(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if routes == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(DeviceSubnetRoutesProperties{DeviceID: req.NativeID, Routes: sortedStrings(routes.Enabled), AdvertisedRoutes: sortedStrings(routes.Advertised)})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h deviceSubnetRoutesHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DeviceSubnetRoutesProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid device subnet routes properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if res := deviceIDMismatch(req, props.DeviceID); res != nil {
		return res, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete clears the enabled subnet routes that this resource managed. Advertised
// routes are left untouched since they are controlled by the device itself.
func (h deviceSubnetRoutesHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DeviceSubnetRoutesResourceType, "delete", "Devices.SetSubnetRoutes", func(ctx context.Context) error {
		return c.Devices().SetSubnetRoutes(ctx, req.NativeID, []string{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (deviceSubnetRoutesHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: []string{}}, nil
}

// ---------------------------------------------------------------------------
// Device tags
// ---------------------------------------------------------------------------

type DeviceTagsProperties struct {
	DeviceID string   `json:"deviceId"`
	Tags     []string `json:"tags"`
}

type deviceTagsHandler struct{}

func (h deviceTagsHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *DeviceTagsProperties) (*resource.ProgressResult, error) {
	tags := props.Tags
	if tags == nil {
		tags = []string{}
	}
	if err := traceAPICallNoResult(ctx, DeviceTagsResourceType, opLabel(op), "Devices.SetTags", func(ctx context.Context) error {
		return c.Devices().SetTags(ctx, nativeID, tags)
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (h deviceTagsHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props DeviceTagsProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid device tags properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if props.DeviceID == "" {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "device tags properties missing 'deviceId'", resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, props.DeviceID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (h deviceTagsHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	dev, err := traceAPICall(apiCtx, DeviceTagsResourceType, "read", "Devices.Get", func(ctx context.Context) (*ts.Device, error) {
		return c.Devices().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if dev == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(DeviceTagsProperties{DeviceID: dev.NodeID, Tags: sortedStrings(dev.Tags)})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h deviceTagsHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props DeviceTagsProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid device tags properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	if res := deviceIDMismatch(req, props.DeviceID); res != nil {
		return res, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete clears the tags applied by this resource, mirroring the upstream
// provider behavior on destroy.
func (h deviceTagsHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, DeviceTagsResourceType, "delete", "Devices.SetTags", func(ctx context.Context) error {
		return c.Devices().SetTags(ctx, req.NativeID, []string{})
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (deviceTagsHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: []string{}}, nil
}

// ---------------------------------------------------------------------------
// Device inventory (read-only discovery / single device lookup)
// ---------------------------------------------------------------------------

type DeviceProperties struct {
	NodeID                    string   `json:"nodeId"`
	ID                        string   `json:"id,omitempty"`
	Name                      string   `json:"name,omitempty"`
	Hostname                  string   `json:"hostname,omitempty"`
	Authorized                bool     `json:"authorized"`
	User                      string   `json:"user,omitempty"`
	Tags                      []string `json:"tags,omitempty"`
	Addresses                 []string `json:"addresses,omitempty"`
	ClientVersion             string   `json:"clientVersion,omitempty"`
	OS                        string   `json:"os,omitempty"`
	KeyExpiryDisabled         bool     `json:"keyExpiryDisabled"`
	BlocksIncomingConnections bool     `json:"blocksIncomingConnections"`
	IsEphemeral               bool     `json:"isEphemeral"`
	IsExternal                bool     `json:"isExternal"`
	ConnectedToControl        bool     `json:"connectedToControl"`
	UpdateAvailable           bool     `json:"updateAvailable"`
	Created                   string   `json:"createdAt,omitempty"`
	Expires                   string   `json:"expiresAt,omitempty"`
	LastSeen                  string   `json:"lastSeen,omitempty"`
}

type deviceHandler struct{}

func deviceFrom(d *ts.Device) *DeviceProperties {
	if d == nil {
		return nil
	}
	props := &DeviceProperties{
		NodeID:                    d.NodeID,
		ID:                        d.ID,
		Name:                      d.Name,
		Hostname:                  d.Hostname,
		Authorized:                d.Authorized,
		User:                      d.User,
		Tags:                      sortedStrings(d.Tags),
		Addresses:                 sortedStrings(d.Addresses),
		ClientVersion:             d.ClientVersion,
		OS:                        d.OS,
		KeyExpiryDisabled:         d.KeyExpiryDisabled,
		BlocksIncomingConnections: d.BlocksIncomingConnections,
		IsEphemeral:               d.IsEphemeral,
		IsExternal:                d.IsExternal,
		ConnectedToControl:        d.ConnectedToControl,
		UpdateAvailable:           d.UpdateAvailable,
		Created:                   formatTime(d.Created.Time),
		Expires:                   formatTime(d.Expires.Time),
	}
	if d.LastSeen != nil {
		props.LastSeen = formatTime(d.LastSeen.Time)
	}
	return props
}

// create/update/delete are not supported: devices join the tailnet naturally and
// are surfaced here for inventory and single-lookup workflows only.
func (deviceHandler) create(_ context.Context, _ tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	return &resource.CreateResult{ProgressResult: notSupported(resource.OperationCreate, DeviceResourceType, "")}, nil
}

func (deviceHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	dev, err := traceAPICall(apiCtx, DeviceResourceType, "read", "Devices.Get", func(ctx context.Context) (*ts.Device, error) {
		return c.Devices().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if dev == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(deviceFrom(dev))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (deviceHandler) update(_ context.Context, _ tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	return &resource.UpdateResult{ProgressResult: notSupported(resource.OperationUpdate, DeviceResourceType, req.NativeID)}, nil
}

func (deviceHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: notSupported(resource.OperationDelete, DeviceResourceType, req.NativeID)}, nil
}

func (deviceHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	devices, err := traceAPICall(apiCtx, DeviceResourceType, "list", "Devices.List", func(ctx context.Context) ([]ts.Device, error) {
		return c.Devices().List(ctx)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		if d.NodeID != "" {
			ids = append(ids, d.NodeID)
		} else {
			ids = append(ids, d.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
