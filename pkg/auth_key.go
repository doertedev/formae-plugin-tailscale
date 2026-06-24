// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

const AuthKeyResourceType = "TAILSCALE::IAM::AuthKey"

func init() { register(AuthKeyResourceType, authKeyHandler{}) }

type AuthKeyProperties struct {
	Reusable      bool     `json:"reusable"`
	Ephemeral     bool     `json:"ephemeral"`
	Tags          []string `json:"tags,omitempty"`
	Preauthorized bool     `json:"preauthorized"`
	ExpirySeconds int64    `json:"expirySeconds,omitempty"`
	Description   string   `json:"description,omitempty"`
	Key           string   `json:"key,omitempty"`
	ID            string   `json:"id,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
	ExpiresAt     string   `json:"expiresAt,omitempty"`
	Invalid       bool     `json:"invalid,omitempty"`
	UserID        string   `json:"userId,omitempty"`
}

type authKeyHandler struct{}

func parseAuthKeyProperties(data json.RawMessage) (*AuthKeyProperties, error) {
	var props AuthKeyProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid auth key properties: %w", err)
	}
	if len(props.Tags) == 0 && props.Preauthorized {
		return nil, errors.New("preauthorized auth keys must include at least one tag")
	}
	return &props, nil
}

func (authKeyHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseAuthKeyProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	var create ts.CreateKeyRequest
	create.Capabilities.Devices.Create.Reusable = props.Reusable
	create.Capabilities.Devices.Create.Ephemeral = props.Ephemeral
	create.Capabilities.Devices.Create.Tags = props.Tags
	create.Capabilities.Devices.Create.Preauthorized = props.Preauthorized
	create.ExpirySeconds = props.ExpirySeconds
	create.Description = props.Description
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, AuthKeyResourceType, "create", "Keys.CreateAuthKey", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().CreateAuthKey(ctx, create)
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	props = authKeyFrom(key)
	// The raw key material is emitted once on create and never re-read; surface
	// it only here so it does not leak into read/list state.
	props.Key = key.Key
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.ID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (authKeyHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, AuthKeyResourceType, "read", "Keys.Get", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if key == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	if keyIsRevoked(key) {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	props := authKeyFrom(key)
	b, _ := json.Marshal(props)
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (authKeyHandler) update(_ context.Context, _ tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "auth keys are immutable; change a createOnly field to replace the resource", resource.OperationErrorCodeNotUpdatable)}, nil
}

func (authKeyHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, AuthKeyResourceType, "delete", "Keys.Delete", func(ctx context.Context) error {
		return c.Keys().Delete(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (authKeyHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	keys, err := traceAPICall(apiCtx, AuthKeyResourceType, "list", "Keys.List", func(ctx context.Context) ([]ts.Key, error) {
		return c.Keys().List(ctx, true)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.KeyType == "" || key.KeyType == "auth" {
			ids = append(ids, key.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}

func authKeyFrom(key *ts.Key) *AuthKeyProperties {
	if key == nil {
		return nil
	}
	props := &AuthKeyProperties{
		Reusable:      key.Capabilities.Devices.Create.Reusable,
		Ephemeral:     key.Capabilities.Devices.Create.Ephemeral,
		Tags:          sortedStrings(key.Capabilities.Devices.Create.Tags),
		Preauthorized: key.Capabilities.Devices.Create.Preauthorized,
		Description:   key.Description,
		ID:            key.ID,
		CreatedAt:     formatTime(key.Created),
		ExpiresAt:     formatTime(key.Expires),
		Invalid:       key.Invalid,
		UserID:        key.UserID,
		// Key intentionally omitted: surfaced only on create.
	}
	if key.ExpirySeconds != nil {
		props.ExpirySeconds = int64((*key.ExpirySeconds) / time.Second)
	}
	return props
}

// keyIsRevoked reports whether a key has been administratively revoked/deleted.
// The Tailscale API removes revoked keys from List immediately but Get keeps
// returning them (with a non-zero Revoked timestamp) for a while, so callers
// must treat a revoked key as gone for drift and out-of-band-removal detection.
// An expired key is Invalid but has a zero Revoked time, so it is not treated as
// removed by this check.
func keyIsRevoked(k *ts.Key) bool {
	return k != nil && !k.Revoked.IsZero()
}
