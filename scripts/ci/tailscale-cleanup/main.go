// © 2026 DoerteDev
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/doertedev/formae-plugin-tailscale/internal/testcleanup"
)

func main() {
	os.Exit(testcleanup.Main())
}
