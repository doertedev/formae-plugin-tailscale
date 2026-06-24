// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestContactsLifecycle(t *testing.T) {
	updated := map[string]string{}
	p := newPluginWithClient(fakeAPI{contacts: fakeContacts{
		update: func(_ context.Context, ct ts.ContactType, req ts.UpdateContactRequest) error {
			updated[string(ct)] = *req.Email
			return nil
		},
		get: func(context.Context) (*ts.Contacts, error) {
			return &ts.Contacts{
				Account:  ts.Contact{Email: "acct@example.com"},
				Support:  ts.Contact{Email: "sup@example.com", NeedsVerification: true},
				Security: ts.Contact{Email: "sec@example.com"},
			}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: ContactsResourceType,
		Properties: rawJSON(t, map[string]any{
			"supportEmail": "sup@example.com", "securityEmail": "sec@example.com",
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	if updated["support"] != "sup@example.com" || updated["security"] != "sec@example.com" {
		t.Fatalf("updates: %v", updated)
	}
	if _, ok := updated["account"]; ok {
		t.Fatal("empty account email should not trigger an update")
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: ContactsResourceType, NativeID: singletonNativeID})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props ContactsProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.AccountEmail != "acct@example.com" || !props.SupportNeedsVerification {
		t.Fatalf("read mapping: %+v", props)
	}
}

func TestContactsUpdateMapsNotFound(t *testing.T) {
	p := newPluginWithClient(fakeAPI{contacts: fakeContacts{
		update: func(context.Context, ts.ContactType, ts.UpdateContactRequest) error {
			return ts.APIError{Status: http.StatusNotFound}
		},
	}})
	res, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      ContactsResourceType,
		NativeID:          singletonNativeID,
		DesiredProperties: rawJSON(t, map[string]any{"accountEmail": "x@example.com"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireErrorCode(t, res.ProgressResult, resource.OperationErrorCodeNotFound)
}
