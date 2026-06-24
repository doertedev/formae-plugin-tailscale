// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() { register(FederatedIdentityResourceType, federatedIdentityHandler{}) }

// FederatedIdentityProperties models a Tailscale federated identity key. It is
// the federated counterpart to an OAuth client, keyed by an IdP trust
// (audience/issuer/subject) rather than a shared client secret.
type FederatedIdentityProperties struct {
	Description      string            `json:"description,omitempty"`
	Scopes           []string          `json:"scopes"`
	Tags             []string          `json:"tags,omitempty"`
	Audience         string            `json:"audience,omitempty"`
	Issuer           string            `json:"issuer,omitempty"`
	Subject          string            `json:"subject,omitempty"`
	CustomClaimRules map[string]string `json:"customClaimRules,omitempty"`
	Key              string            `json:"key,omitempty"`
	ID               string            `json:"id,omitempty"`
	CreatedAt        string            `json:"createdAt,omitempty"`
	UpdatedAt        string            `json:"updatedAt,omitempty"`
	UserID           string            `json:"userId,omitempty"`
}

type federatedIdentityHandler struct{}

func federatedIdentityFrom(key *ts.Key) *FederatedIdentityProperties {
	if key == nil {
		return nil
	}
	return &FederatedIdentityProperties{
		Description:      key.Description,
		Scopes:           sortedStrings(key.Scopes),
		Tags:             sortedStrings(key.Tags),
		Audience:         key.Audience,
		Issuer:           key.Issuer,
		Subject:          key.Subject,
		CustomClaimRules: key.CustomClaimRules,
		ID:               key.ID,
		CreatedAt:        formatTime(key.Created),
		UpdatedAt:        formatTime(key.Updated),
		UserID:           key.UserID,
		// Key intentionally omitted: surfaced only on create.
	}
}

func parseFederatedIdentityProperties(data json.RawMessage) (*FederatedIdentityProperties, error) {
	var props FederatedIdentityProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, err
	}
	if len(props.Scopes) == 0 {
		return nil, errors.New("federated identity properties missing 'scopes'")
	}
	return &props, nil
}

func (federatedIdentityHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseFederatedIdentityProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid federated identity properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, FederatedIdentityResourceType, "create", "Keys.CreateFederatedIdentity", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().CreateFederatedIdentity(ctx, ts.CreateFederatedIdentityRequest{
			Scopes:           props.Scopes,
			Tags:             props.Tags,
			Audience:         props.Audience,
			Issuer:           props.Issuer,
			Subject:          props.Subject,
			CustomClaimRules: props.CustomClaimRules,
			Description:      props.Description,
		})
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	props = federatedIdentityFrom(key)
	// The federated key material is emitted once on create and never re-read;
	// surface it only here so it does not leak into read/list/update state.
	props.Key = key.Key
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.ID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (federatedIdentityHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, FederatedIdentityResourceType, "read", "Keys.Get", func(ctx context.Context) (*ts.Key, error) {
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
	b, _ := json.Marshal(federatedIdentityFrom(key))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (federatedIdentityHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseFederatedIdentityProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid federated identity properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	key, err := traceAPICall(apiCtx, FederatedIdentityResourceType, "update", "Keys.SetFederatedIdentity", func(ctx context.Context) (*ts.Key, error) {
		return c.Keys().SetFederatedIdentity(ctx, req.NativeID, ts.SetFederatedIdentityRequest{
			Scopes:           props.Scopes,
			Tags:             props.Tags,
			Audience:         props.Audience,
			Issuer:           props.Issuer,
			Subject:          props.Subject,
			CustomClaimRules: props.CustomClaimRules,
			Description:      props.Description,
		})
	})
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	props = federatedIdentityFrom(key)
	pr := progress(resource.OperationUpdate, resource.OperationStatusSuccess, req.NativeID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (federatedIdentityHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, FederatedIdentityResourceType, "delete", "Keys.Delete", func(ctx context.Context) error {
		return c.Keys().Delete(ctx, req.NativeID)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (federatedIdentityHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	keys, err := traceAPICall(apiCtx, FederatedIdentityResourceType, "list", "Keys.List", func(ctx context.Context) ([]ts.Key, error) {
		return c.Keys().List(ctx, true)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.KeyType == "federated" {
			ids = append(ids, key.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
