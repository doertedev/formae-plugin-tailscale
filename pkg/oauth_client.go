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

const OAuthClientResourceType = "TAILSCALE::IAM::OAuthClient"

func init() { register(OAuthClientResourceType, oauthClientHandler{}) }

type OAuthClientProperties struct {
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes"`
	Tags        []string `json:"tags,omitempty"`
	Key         string   `json:"key,omitempty"`
	ID          string   `json:"id,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	UserID      string   `json:"userId,omitempty"`
}

type oauthClientHandler struct{}

func parseOAuthClientProperties(data json.RawMessage) (*OAuthClientProperties, error) {
	var props OAuthClientProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid oauth client properties: %w", err)
	}
	if len(props.Scopes) == 0 {
		return nil, errors.New("oauth client properties missing 'scopes'")
	}
	return &props, nil
}

func (oauthClientHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseOAuthClientProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, OAuthClientResourceType, "create", "Keys.CreateOAuthClient", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().CreateOAuthClient(ctx, ts.CreateOAuthClientRequest{
			Description: props.Description,
			Scopes:      props.Scopes,
			Tags:        props.Tags,
		})
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	props = oauthClientFrom(key)
	// The client secret is emitted once on create and never re-read; surface it
	// only here so it does not leak into read/list state.
	props.Key = key.Key
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.ID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (oauthClientHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, OAuthClientResourceType, "read", "Keys.Get", func(ctx context.Context) (*ts.Key, error) {
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
	props := oauthClientFrom(key)
	b, _ := json.Marshal(props)
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (oauthClientHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseOAuthClientProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, OAuthClientResourceType, "update", "Keys.SetOAuthClient", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().SetOAuthClient(ctx, req.NativeID, ts.SetOAuthClientRequest{
			Description: props.Description,
			Scopes:      props.Scopes,
			Tags:        props.Tags,
		})
	})
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props = oauthClientFrom(key)
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (oauthClientHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, OAuthClientResourceType, "delete", "Keys.Delete", func(ctx context.Context) error {
		return c.Keys().Delete(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (oauthClientHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	keys, err := traceAPICall(apiCtx, OAuthClientResourceType, "list", "Keys.List", func(ctx context.Context) ([]ts.Key, error) {
		return c.Keys().List(ctx, true)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.KeyType == "client" {
			ids = append(ids, key.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}

func oauthClientFrom(key *ts.Key) *OAuthClientProperties {
	if key == nil {
		return nil
	}
	return &OAuthClientProperties{
		Description: key.Description,
		Scopes:      sortedStrings(key.Scopes),
		Tags:        sortedStrings(key.Tags),
		ID:          key.ID,
		CreatedAt:   formatTime(key.Created),
		UpdatedAt:   formatTime(key.Updated),
		UserID:      key.UserID,
		// Key intentionally omitted: surfaced only on create.
	}
}
