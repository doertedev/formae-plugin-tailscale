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

func init() { register(ContactsResourceType, contactsHandler{}) }

// ContactsProperties models the three tailnet contact channels. Email fields
// are written when non-empty; the remaining fields are read-only state returned
// by the API (e.g. pending verification flags).
type ContactsProperties struct {
	Tailnet string `json:"tailnet,omitempty"`

	AccountEmail  string `json:"accountEmail,omitempty"`
	SupportEmail  string `json:"supportEmail,omitempty"`
	SecurityEmail string `json:"securityEmail,omitempty"`

	AccountNeedsVerification  bool   `json:"accountNeedsVerification,omitempty"`
	AccountFallbackEmail      string `json:"accountFallbackEmail,omitempty"`
	SupportNeedsVerification  bool   `json:"supportNeedsVerification,omitempty"`
	SupportFallbackEmail      string `json:"supportFallbackEmail,omitempty"`
	SecurityNeedsVerification bool   `json:"securityNeedsVerification,omitempty"`
	SecurityFallbackEmail     string `json:"securityFallbackEmail,omitempty"`
}

type contactsHandler struct{}

func parseContactsProperties(data json.RawMessage) (*ContactsProperties, error) {
	var props ContactsProperties
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("invalid contacts properties: %w", err)
	}
	return &props, nil
}

func contactsFrom(c *ts.Contacts, nativeID string) *ContactsProperties {
	if c == nil {
		return nil
	}
	return &ContactsProperties{
		Tailnet:                   nativeID,
		AccountEmail:              c.Account.Email,
		AccountNeedsVerification:  c.Account.NeedsVerification,
		AccountFallbackEmail:      c.Account.FallbackEmail,
		SupportEmail:              c.Support.Email,
		SupportNeedsVerification:  c.Support.NeedsVerification,
		SupportFallbackEmail:      c.Support.FallbackEmail,
		SecurityEmail:             c.Security.Email,
		SecurityNeedsVerification: c.Security.NeedsVerification,
		SecurityFallbackEmail:     c.Security.FallbackEmail,
	}
}

// applyContacts upserts each contact email that is set on the properties. The
// account contact frequently mirrors the tailnet owner and may reject updates;
// such failures surface as a normal operation failure.
//
// Emails with an empty value are skipped, so a contact can only be set or
// overwritten, never cleared. This is intentional: the Tailscale contacts API
// treats a missing email as "leave unchanged" rather than "delete", and
// clearing the account contact is rejected outright. To change a contact,
// supply the new email; to relinquish management, remove the resource (delete
// is a no-op success since contacts are an always-present singleton).
func applyContacts(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *ContactsProperties) (*resource.ProgressResult, error) {
	updates := []struct {
		contactType ts.ContactType
		email       string
	}{
		{ts.ContactAccount, props.AccountEmail},
		{ts.ContactSupport, props.SupportEmail},
		{ts.ContactSecurity, props.SecurityEmail},
	}
	for _, u := range updates {
		if u.email == "" {
			continue
		}
		email := u.email
		if err := traceAPICallNoResult(ctx, ContactsResourceType, opLabel(op), "Contacts.Update", func(ctx context.Context) error {
			return c.Contacts().Update(ctx, u.contactType, ts.UpdateContactRequest{Email: &email})
		}); err != nil {
			return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
		}
	}
	props.Tailnet = nativeID
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(props)
	return pr, nil
}

func (contactsHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	props, err := parseContactsProperties(req.Properties)
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := applyContacts(apiCtx, c, resource.OperationCreate, singletonNativeID, props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (contactsHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	contacts, err := traceAPICall(apiCtx, ContactsResourceType, "read", "Contacts.Get", func(ctx context.Context) (*ts.Contacts, error) {
		return c.Contacts().Get(ctx)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if contacts == nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(contactsFrom(contacts, req.NativeID))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (contactsHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	props, err := parseContactsProperties(req.DesiredProperties)
	if err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := applyContacts(apiCtx, c, resource.OperationUpdate, req.NativeID, props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

// delete is a no-op success: contacts are an always-present singleton and the
// account contact cannot be removed. Destroying the managed resource simply
// relinquishes management.
func (contactsHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (contactsHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return singletonList(), nil
}
