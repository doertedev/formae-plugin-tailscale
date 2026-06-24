// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

//go:build conformance && !integration

// Conformance tests for the plugin. Run with: make conformance-test
package main

import (
	"testing"

	conformance "github.com/platform-engineering-labs/formae/pkg/plugin-conformance-tests"
)

func TestPluginConformance(t *testing.T) {
	conformance.RunCRUDTests(t)
}

func TestPluginDiscovery(t *testing.T) {
	conformance.RunDiscoveryTests(t)
}
