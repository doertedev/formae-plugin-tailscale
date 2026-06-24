// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func init() { register(UserResourceType, userHandler{}) }

// UserProperties models a tailnet user. The user resource is read-only: users
// are provisioned via SSO/identity and surfaced here for inventory and single
// lookup workflows.
type UserProperties struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName,omitempty"`
	LoginName          string `json:"loginName,omitempty"`
	ProfilePicURL      string `json:"profilePicUrl,omitempty"`
	TailnetID          string `json:"tailnetId,omitempty"`
	Type               string `json:"type,omitempty"`
	Role               string `json:"role,omitempty"`
	Status             string `json:"status,omitempty"`
	DeviceCount        int    `json:"deviceCount,omitempty"`
	CurrentlyConnected bool   `json:"currentlyConnected"`
	CreatedAt          string `json:"createdAt,omitempty"`
	LastSeen           string `json:"lastSeen,omitempty"`
}

type userHandler struct{}

func userFrom(u *ts.User) *UserProperties {
	if u == nil {
		return nil
	}
	return &UserProperties{
		ID:                 u.ID,
		DisplayName:        u.DisplayName,
		LoginName:          u.LoginName,
		ProfilePicURL:      u.ProfilePicURL,
		TailnetID:          u.TailnetID,
		Type:               string(u.Type),
		Role:               string(u.Role),
		Status:             string(u.Status),
		DeviceCount:        u.DeviceCount,
		CurrentlyConnected: u.CurrentlyConnected,
		CreatedAt:          formatTime(u.Created),
		LastSeen:           formatTime(u.LastSeen),
	}
}

// create/update/delete are not supported: users are managed by the identity
// provider and surfaced here for read-only inventory and lookup.
func (userHandler) create(_ context.Context, _ tailscaleAPI, _ *resource.CreateRequest) (*resource.CreateResult, error) {
	return &resource.CreateResult{ProgressResult: notSupported(resource.OperationCreate, UserResourceType, "")}, nil
}

func (userHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	u, err := traceAPICall(apiCtx, UserResourceType, "read", "Users.Get", func(ctx context.Context) (*ts.User, error) {
		return c.Users().Get(ctx, req.NativeID)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if u == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(userFrom(u))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (userHandler) update(_ context.Context, _ tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	return &resource.UpdateResult{ProgressResult: notSupported(resource.OperationUpdate, UserResourceType, req.NativeID)}, nil
}

func (userHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: notSupported(resource.OperationDelete, UserResourceType, req.NativeID)}, nil
}

func (userHandler) list(ctx context.Context, c tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	users, err := traceAPICall(apiCtx, UserResourceType, "list", "Users.List", func(ctx context.Context) ([]ts.User, error) {
		return c.Users().List(ctx, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
