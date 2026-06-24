// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
	ts "tailscale.com/client/tailscale/v2"
)

func TestLogstreamConfigurationLifecycle(t *testing.T) {
	var sent ts.SetLogstreamConfigurationRequest
	p := newPluginWithClient(fakeAPI{logging: fakeLogging{
		setLogstreamConfiguration: func(_ context.Context, _ ts.LogType, r ts.SetLogstreamConfigurationRequest) error {
			sent = r
			return nil
		},
		logstreamConfiguration: func(_ context.Context, t ts.LogType) (*ts.LogstreamConfiguration, error) {
			return &ts.LogstreamConfiguration{DestinationType: ts.LogstreamSplunkEndpoint, URL: "https://splunk", User: "u", GCSCredentials: "super-secret-sa-json"}, nil
		},
		deleteLogstreamConfiguration: func(context.Context, ts.LogType) error { return nil },
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: LogstreamConfigurationResourceType,
		Properties: rawJSON(t, map[string]any{
			"logType": "network", "destinationType": "splunk", "url": "https://splunk", "token": "secret",
		}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	requireNativeID(t, create.ProgressResult, "network")
	if sent.Token != "secret" {
		t.Fatal("token not forwarded")
	}

	// The progress result is persisted as state by the engine, so write-only
	// credentials must not be echoed back (the API never returns them on read).
	var created LogstreamConfigurationProperties
	decodeJSON(t, create.ProgressResult.ResourceProperties, &created)
	if created.Token != "" {
		t.Fatalf("token must not appear in create state: %+v", created)
	}
	if created.S3SecretAccessKey != "" || created.GCSCredentials != "" {
		t.Fatalf("write-only credentials must not appear in create state: %+v", created)
	}
	if created.URL != "https://splunk" {
		t.Fatalf("non-secret field dropped from create state: %+v", created)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{ResourceType: LogstreamConfigurationResourceType, NativeID: "network"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var props LogstreamConfigurationProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.Token != "" {
		t.Fatalf("token must not be returned on read: %+v", props)
	}
	if props.GCSCredentials != "" {
		t.Fatalf("gcsCredentials must not be returned on read: %+v", props)
	}
	if props.DestinationType != "splunk" {
		t.Fatalf("read mapping: %+v", props)
	}

	list, err := p.List(context.Background(), &resource.ListRequest{ResourceType: LogstreamConfigurationResourceType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.NativeIDs) != 2 {
		t.Fatalf("list log types: %v", list.NativeIDs)
	}
}

func TestAWSExternalIDCreate(t *testing.T) {
	var reusable bool
	p := newPluginWithClient(fakeAPI{logging: fakeLogging{
		createOrGetAwsExternalId: func(_ context.Context, r bool) (*ts.AWSExternalID, error) {
			reusable = r
			return &ts.AWSExternalID{ExternalID: "ext-1", TailscaleAWSAccountID: "123456789012"}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: AWSExternalIDResourceType,
		Properties:   rawJSON(t, map[string]any{"reusable": true}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	requireNativeID(t, create.ProgressResult, "ext-1")
	if !reusable {
		t.Fatal("reusable flag not forwarded")
	}

	upd, err := p.Update(context.Background(), &resource.UpdateRequest{
		ResourceType:      AWSExternalIDResourceType,
		NativeID:          "ext-1",
		DesiredProperties: rawJSON(t, map[string]any{"reusable": false}),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	requireErrorCode(t, upd.ProgressResult, resource.OperationErrorCodeNotUpdatable)
}

// TestAWSExternalIDNonReusableReadIsNotFound verifies that a resource created
// with reusable=false does not read back as a different (reusable) ID or report
// reusable=true. Because the API can only resolve the shared reusable ID, a
// non-reusable native ID no longer matches and read reports NotFound.
func TestAWSExternalIDNonReusableReadIsNotFound(t *testing.T) {
	var reusableSeen []bool
	p := newPluginWithClient(fakeAPI{logging: fakeLogging{
		createOrGetAwsExternalId: func(_ context.Context, r bool) (*ts.AWSExternalID, error) {
			reusableSeen = append(reusableSeen, r)
			// The API returns distinct IDs: a shared reusable ID and a unique
			// non-reusable ID per call with reusable=false.
			if r {
				return &ts.AWSExternalID{ExternalID: "reusable-id", TailscaleAWSAccountID: "123456789012"}, nil
			}
			return &ts.AWSExternalID{ExternalID: "unique-id", TailscaleAWSAccountID: "123456789012"}, nil
		},
	}})

	create, err := p.Create(context.Background(), &resource.CreateRequest{
		ResourceType: AWSExternalIDResourceType,
		Properties:   rawJSON(t, map[string]any{"reusable": false}),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requireSuccess(t, create.ProgressResult)
	requireNativeID(t, create.ProgressResult, "unique-id")
	if len(reusableSeen) != 1 || reusableSeen[0] {
		t.Fatalf("create should forward reusable=false, got %v", reusableSeen)
	}

	read, err := p.Read(context.Background(), &resource.ReadRequest{
		ResourceType: AWSExternalIDResourceType,
		NativeID:     "unique-id",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("non-reusable ID should read as NotFound, got code=%q properties=%q", read.ErrorCode, read.Properties)
	}
}

// TestAWSExternalIDReusableReadIsStable verifies that a resource created with
// reusable=true reads back the same ID stably.
func TestAWSExternalIDReusableReadIsStable(t *testing.T) {
	p := newPluginWithClient(fakeAPI{logging: fakeLogging{
		createOrGetAwsExternalId: func(_ context.Context, r bool) (*ts.AWSExternalID, error) {
			return &ts.AWSExternalID{ExternalID: "reusable-id", TailscaleAWSAccountID: "123456789012"}, nil
		},
	}})

	read, err := p.Read(context.Background(), &resource.ReadRequest{
		ResourceType: AWSExternalIDResourceType,
		NativeID:     "reusable-id",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.ErrorCode != "" {
		t.Fatalf("reusable ID read should succeed, got code=%q", read.ErrorCode)
	}
	var props AWSExternalIDProperties
	decodeJSON(t, []byte(read.Properties), &props)
	if props.ExternalID != "reusable-id" || !props.Reusable {
		t.Fatalf("reusable ID read mapping: %+v", props)
	}
}
