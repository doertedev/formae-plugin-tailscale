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

func init() {
	register(LogstreamConfigurationResourceType, logstreamConfigurationHandler{})
	register(AWSExternalIDResourceType, awsExternalIDHandler{})
}

// ---------------------------------------------------------------------------
// Logstream configuration
// ---------------------------------------------------------------------------

// LogstreamConfigurationProperties models a log streaming endpoint scoped to a
// log type. The Token is write-only: accepted on create/update but never
// returned by the API, so it is omitted from read state.
type LogstreamConfigurationProperties struct {
	LogType              string   `json:"logType"`
	DestinationType      string   `json:"destinationType,omitempty"`
	URL                  string   `json:"url,omitempty"`
	User                 string   `json:"user,omitempty"`
	Token                string   `json:"token,omitempty"`
	UploadPeriodMinutes  int      `json:"uploadPeriodMinutes,omitempty"`
	CompressionFormat    string   `json:"compressionFormat,omitempty"`
	S3Bucket             string   `json:"s3Bucket,omitempty"`
	S3Region             string   `json:"s3Region,omitempty"`
	S3KeyPrefix          string   `json:"s3KeyPrefix,omitempty"`
	S3AuthenticationType string   `json:"s3AuthenticationType,omitempty"`
	S3AccessKeyID        string   `json:"s3AccessKeyId,omitempty"`
	S3SecretAccessKey    string   `json:"s3SecretAccessKey,omitempty"`
	S3RoleARN            string   `json:"s3RoleArn,omitempty"`
	S3ExternalID         string   `json:"s3ExternalId,omitempty"`
	GCSBucket            string   `json:"gcsBucket,omitempty"`
	GCSKeyPrefix         string   `json:"gcsKeyPrefix,omitempty"`
	GCSScopes            []string `json:"gcsScopes,omitempty"`
	GCSCredentials       string   `json:"gcsCredentials,omitempty"`
}

type logstreamConfigurationHandler struct{}

var supportedLogTypes = []string{string(ts.LogTypeConfig), string(ts.LogTypeNetwork)}

func logstreamFromNativeID(logType string) (ts.LogType, bool) {
	switch ts.LogType(logType) {
	case ts.LogTypeConfig, ts.LogTypeNetwork:
		return ts.LogType(logType), true
	}
	return "", false
}

func logstreamResponseFrom(c *ts.LogstreamConfiguration, nativeID string) *LogstreamConfigurationProperties {
	if c == nil {
		return nil
	}
	return &LogstreamConfigurationProperties{
		LogType:              nativeID,
		DestinationType:      string(c.DestinationType),
		URL:                  c.URL,
		User:                 c.User,
		UploadPeriodMinutes:  c.UploadPeriodMinutes,
		CompressionFormat:    string(c.CompressionFormat),
		S3Bucket:             c.S3Bucket,
		S3Region:             c.S3Region,
		S3KeyPrefix:          c.S3KeyPrefix,
		S3AuthenticationType: string(c.S3AuthenticationType),
		S3AccessKeyID:        c.S3AccessKeyID,
		S3RoleARN:            c.S3RoleARN,
		S3ExternalID:         c.S3ExternalID,
		GCSBucket:            c.GCSBucket,
		GCSKeyPrefix:         c.GCSKeyPrefix,
		GCSScopes:            sortedStrings(c.GCSScopes),
		// GCSCredentials intentionally omitted: surfaced only on write (it is a
		// service-account JSON key). S3SecretAccessKey is likewise write-only.
	}
}

func (logstreamConfigurationHandler) apply(ctx context.Context, c tailscaleAPI, op resource.Operation, nativeID string, props *LogstreamConfigurationProperties) (*resource.ProgressResult, error) {
	logType, ok := logstreamFromNativeID(nativeID)
	if !ok {
		return fail(op, nativeID, "unsupported logType "+nativeID+": want configuration|network", resource.OperationErrorCodeInvalidRequest), nil
	}
	if err := traceAPICallNoResult(ctx, LogstreamConfigurationResourceType, opLabel(op), "Logging.SetLogstreamConfiguration", func(ctx context.Context) error {
		return c.Logging().SetLogstreamConfiguration(ctx, logType, ts.SetLogstreamConfigurationRequest{
			DestinationType:      ts.LogstreamEndpointType(props.DestinationType),
			URL:                  props.URL,
			User:                 props.User,
			Token:                props.Token,
			UploadPeriodMinutes:  props.UploadPeriodMinutes,
			CompressionFormat:    ts.CompressionFormat(props.CompressionFormat),
			S3Bucket:             props.S3Bucket,
			S3Region:             props.S3Region,
			S3KeyPrefix:          props.S3KeyPrefix,
			S3AuthenticationType: ts.S3AuthenticationType(props.S3AuthenticationType),
			S3AccessKeyID:        props.S3AccessKeyID,
			S3SecretAccessKey:    props.S3SecretAccessKey,
			S3RoleARN:            props.S3RoleARN,
			S3ExternalID:         props.S3ExternalID,
			GCSBucket:            props.GCSBucket,
			GCSKeyPrefix:         props.GCSKeyPrefix,
			GCSScopes:            props.GCSScopes,
			GCSCredentials:       props.GCSCredentials,
		})
	}); err != nil {
		return fail(op, nativeID, err.Error(), mapTailscaleError(err)), nil
	}
	// Return a copy with write-only credentials stripped. The engine persists
	// ResourceProperties as current state; surfacing these fields would (a) leak
	// plaintext secrets and (b) cause perpetual drift, since the Tailscale API
	// never returns them on read. See logstreamResponseFrom.
	pr := progress(op, resource.OperationStatusSuccess, nativeID)
	pr.ResourceProperties = marshalProperties(logstreamWriteResult(props))
	return pr, nil
}

// logstreamWriteResult returns a shallow copy of props with write-only
// credentials removed, suitable for the progress result returned from a write.
func logstreamWriteResult(props *LogstreamConfigurationProperties) *LogstreamConfigurationProperties {
	out := *props
	out.Token = ""
	out.S3SecretAccessKey = ""
	out.GCSCredentials = ""
	return &out
}

func (h logstreamConfigurationHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props LogstreamConfigurationProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid logstream configuration properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	nativeID := props.LogType
	if nativeID == "" {
		nativeID = string(ts.LogTypeConfig)
	}
	props.LogType = nativeID
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationCreate, nativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.CreateResult{ProgressResult: pr}, nil
}

func (logstreamConfigurationHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	logType, ok := logstreamFromNativeID(req.NativeID)
	if !ok {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	cfg, err := traceAPICall(apiCtx, LogstreamConfigurationResourceType, "read", "Logging.LogstreamConfiguration", func(ctx context.Context) (*ts.LogstreamConfiguration, error) {
		return c.Logging().LogstreamConfiguration(ctx, logType)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	b, _ := json.Marshal(logstreamResponseFrom(cfg, req.NativeID))
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (h logstreamConfigurationHandler) update(ctx context.Context, c tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var props LogstreamConfigurationProperties
	if err := json.Unmarshal(req.DesiredProperties, &props); err != nil {
		return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "invalid logstream configuration properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	props.LogType = req.NativeID
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	pr, err := h.apply(apiCtx, c, resource.OperationUpdate, req.NativeID, &props)
	if err != nil {
		return nil, err
	}
	return &resource.UpdateResult{ProgressResult: pr}, nil
}

func (logstreamConfigurationHandler) delete(ctx context.Context, c tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	logType, ok := logstreamFromNativeID(req.NativeID)
	if !ok {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, "unsupported logType "+req.NativeID, resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	if err := traceAPICallNoResult(apiCtx, LogstreamConfigurationResourceType, "delete", "Logging.DeleteLogstreamConfiguration", func(ctx context.Context) error {
		return c.Logging().DeleteLogstreamConfiguration(ctx, logType)
	}); err != nil {
		return &resource.DeleteResult{ProgressResult: fail(resource.OperationDelete, req.NativeID, err.Error(), mapTailscaleError(err))}, nil
	}
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (logstreamConfigurationHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: append([]string(nil), supportedLogTypes...)}, nil
}

// ---------------------------------------------------------------------------
// AWS external ID (log streaming bootstrap)
// ---------------------------------------------------------------------------

// AWSExternalIDProperties models the AWS external ID used by S3 RoleARN log
// streaming endpoints. The Tailscale API exposes only a create-or-get endpoint,
// so this resource is create/get-oriented: read idempotently fetches the shared
// reusable external ID, update is not supported, and delete is a no-op (external
// IDs are not deletable).
type AWSExternalIDProperties struct {
	ExternalID            string `json:"externalId"`
	TailscaleAWSAccountID string `json:"tailscaleAwsAccountId,omitempty"`
	Reusable              bool   `json:"reusable,omitempty"`
}

type awsExternalIDHandler struct{}

func (awsExternalIDHandler) create(ctx context.Context, c tailscaleAPI, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props AWSExternalIDProperties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "invalid aws external id properties: "+err.Error(), resource.OperationErrorCodeInvalidRequest)}, nil
	}
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	ext, err := traceAPICall(apiCtx, AWSExternalIDResourceType, "create", "Logging.CreateOrGetAwsExternalId", func(ctx context.Context) (*ts.AWSExternalID, error) {
		return c.Logging().CreateOrGetAwsExternalId(ctx, props.Reusable)
	})
	if err != nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", err.Error(), mapTailscaleError(err))}, nil
	}
	if ext == nil {
		return &resource.CreateResult{ProgressResult: fail(resource.OperationCreate, "", "empty aws external id response", resource.OperationErrorCodeInternalFailure)}, nil
	}
	props = AWSExternalIDProperties{ExternalID: ext.ExternalID, TailscaleAWSAccountID: ext.TailscaleAWSAccountID, Reusable: props.Reusable}
	pr := progress(resource.OperationCreate, resource.OperationStatusSuccess, props.ExternalID)
	pr.ResourceProperties = marshalProperties(props)
	return &resource.CreateResult{ProgressResult: pr}, nil
}

// read idempotently resolves the shared reusable external ID. Because the API
// offers no plain GET, we re-issue create-or-get with reusable=true, which
// returns the shared reusable external ID if one already exists for the tailnet.
//
// The Tailscale API cannot read back an arbitrary (non-reusable) external ID:
// create-or-get always resolves to the single shared reusable ID. To keep the
// native ID stable and avoid reporting reusable=true for a resource created
// with reusable=false, we compare the resolved ID against req.NativeID. A
// mismatch means the managed resource was created non-reusable (or has been
// rotated) and can no longer be found this way, so we report NotFound rather
// than silently substituting a different ID.
func (awsExternalIDHandler) read(ctx context.Context, c tailscaleAPI, req *resource.ReadRequest) (*resource.ReadResult, error) {
	apiCtx, cancel := apiContext(ctx)
	defer cancel()
	ext, err := traceAPICall(apiCtx, AWSExternalIDResourceType, "read", "Logging.CreateOrGetAwsExternalId", func(ctx context.Context) (*ts.AWSExternalID, error) {
		return c.Logging().CreateOrGetAwsExternalId(ctx, true)
	})
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: mapTailscaleError(err)}, nil
	}
	if ext == nil || ext.ExternalID != req.NativeID {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	b, _ := json.Marshal(AWSExternalIDProperties{ExternalID: ext.ExternalID, TailscaleAWSAccountID: ext.TailscaleAWSAccountID, Reusable: true})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(b)}, nil
}

func (awsExternalIDHandler) update(_ context.Context, _ tailscaleAPI, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	return &resource.UpdateResult{ProgressResult: fail(resource.OperationUpdate, req.NativeID, "aws external id is immutable", resource.OperationErrorCodeNotUpdatable)}, nil
}

func (awsExternalIDHandler) delete(_ context.Context, _ tailscaleAPI, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{ProgressResult: progress(resource.OperationDelete, resource.OperationStatusSuccess, req.NativeID)}, nil
}

func (awsExternalIDHandler) list(_ context.Context, _ tailscaleAPI, _ *resource.ListRequest) (*resource.ListResult, error) {
	return &resource.ListResult{NativeIDs: []string{}}, nil
}
