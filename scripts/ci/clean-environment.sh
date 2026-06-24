#!/bin/bash
# © 2026 DoerteDev
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

if [ -z "${TAILSCALE_API_KEY:-}" ] && { [ -z "${TAILSCALE_OAUTH_CLIENT_ID:-}" ] || [ -z "${TAILSCALE_OAUTH_CLIENT_SECRET:-}" ]; }; then
  echo "clean-environment.sh: Tailscale credentials are not set; skipping cleanup."
  exit 0
fi

export TAILSCALE_CLEANUP_PREFIXES="${TAILSCALE_CLEANUP_PREFIXES:-${TEST_PREFIX:-formae-test-}}"

echo "clean-environment.sh: Sweeping Tailscale test resources with prefixes: ${TAILSCALE_CLEANUP_PREFIXES}"
go run ./scripts/ci/tailscale-cleanup
