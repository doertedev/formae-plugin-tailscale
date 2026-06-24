// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// Shared test helpers compiled into every build (unit, integration, and
// conformance) so handler tests can reuse the same assertion utilities.

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return b
}

func decodeJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode json: %v\n%s", err, string(b))
	}
}

func requireSuccess(t *testing.T, pr *resource.ProgressResult) {
	t.Helper()
	if pr == nil {
		t.Fatal("missing progress result")
	}
	if pr.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("operation failed: status=%s code=%s message=%s", pr.OperationStatus, pr.ErrorCode, pr.StatusMessage)
	}
	if pr.NativeID == "" {
		t.Fatal("operation returned empty NativeID")
	}
}

// requireErrorCode asserts a progress result carries the expected error code.
// Failure messages include status and message so the cause is visible without
// inspecting the helper body.
func requireErrorCode(t *testing.T, pr *resource.ProgressResult, code resource.OperationErrorCode) {
	t.Helper()
	if pr == nil {
		t.Fatalf("missing progress result; want error code %s", code)
	}
	if pr.ErrorCode != code {
		t.Fatalf("ErrorCode: want %s got %s (status=%s message=%s)", code, pr.ErrorCode, pr.OperationStatus, pr.StatusMessage)
	}
}

// requireNativeID asserts a progress result carries the expected native id.
func requireNativeID(t *testing.T, pr *resource.ProgressResult, id string) {
	t.Helper()
	if pr == nil {
		t.Fatalf("missing progress result; want nativeID %q", id)
	}
	if pr.NativeID != id {
		t.Fatalf("NativeID: want %q got %q", id, pr.NativeID)
	}
}

// requireNativeIDs asserts a list result matches ids exactly, in order.
func requireNativeIDs(t *testing.T, list *resource.ListResult, ids ...string) {
	t.Helper()
	if list == nil {
		t.Fatalf("missing list result; want %v", ids)
	}
	if len(list.NativeIDs) != len(ids) {
		t.Fatalf("NativeIDs: want %v got %v", ids, list.NativeIDs)
	}
	for i, want := range ids {
		if list.NativeIDs[i] != want {
			t.Fatalf("NativeIDs: want %v got %v", ids, list.NativeIDs)
		}
	}
}

// requireUnsupportedWriteOps asserts that create/update/delete on a read-only
// (or otherwise non-writable) resource all fail with InvalidRequest. Shared by
// the device-inventory and user read-only resource tests.
func requireUnsupportedWriteOps(t *testing.T, p *Plugin, resourceType, nativeID string) {
	t.Helper()
	ctx := context.Background()
	for _, op := range []struct {
		name string
		do   func() *resource.ProgressResult
	}{
		{"create", func() *resource.ProgressResult {
			r, _ := p.Create(ctx, &resource.CreateRequest{ResourceType: resourceType})
			if r == nil {
				return nil
			}
			return r.ProgressResult
		}},
		{"update", func() *resource.ProgressResult {
			r, _ := p.Update(ctx, &resource.UpdateRequest{ResourceType: resourceType, NativeID: nativeID})
			if r == nil {
				return nil
			}
			return r.ProgressResult
		}},
		{"delete", func() *resource.ProgressResult {
			r, _ := p.Delete(ctx, &resource.DeleteRequest{ResourceType: resourceType, NativeID: nativeID})
			if r == nil {
				return nil
			}
			return r.ProgressResult
		}},
	} {
		pr := op.do()
		code := ""
		if pr != nil {
			code = string(pr.ErrorCode)
		}
		if code != string(resource.OperationErrorCodeInvalidRequest) {
			t.Fatalf("%s %s: want InvalidRequest got %q", resourceType, op.name, code)
		}
	}
}
