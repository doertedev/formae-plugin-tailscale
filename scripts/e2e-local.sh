#!/bin/bash
# © 2026 DoerteDev
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${WORK_DIR:-$(mktemp -d /private/tmp/formae-plugin-tailscale-e2e.XXXXXX)}"
RUN_ID="${RUN_ID:-$(date +%s)}"
STACK_LABEL="tailscale-e2e-${RUN_ID}"

if [ -f "${ROOT_DIR}/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${ROOT_DIR}/.env"
  set +a
fi

if [ -z "${TAILSCALE_API_KEY:-}" ] && { [ -z "${TAILSCALE_OAUTH_CLIENT_ID:-}" ] || [ -z "${TAILSCALE_OAUTH_CLIENT_SECRET:-}" ]; }; then
  echo "ERROR: set TAILSCALE_API_KEY or TAILSCALE_OAUTH_CLIENT_ID/TAILSCALE_OAUTH_CLIENT_SECRET."
  echo "The local E2E flow creates real Tailscale resources."
  exit 1
fi

cleanup() {
  set +e
  if [ -f "${WORK_DIR}/main.pkl" ]; then
    formae destroy --yes --watch "${WORK_DIR}/main.pkl"
  fi
  TEST_PREFIX="formae-test-" "${ROOT_DIR}/scripts/ci/clean-environment.sh"
}
trap cleanup EXIT

mkdir -p "${WORK_DIR}/schema"
cp -R "${ROOT_DIR}/schema/pkl" "${WORK_DIR}/schema/pkl"

# Derive the formae version from the plugin manifest so this script stays in
# sync with minFormaeVersion (auto-bumped by `make build`) instead of drifting.
FORMAE_VERSION="${FORMAE_VERSION:-$(pkl eval -x minFormaeVersion "${ROOT_DIR}/formae-plugin.pkl" 2>/dev/null || echo "0.87.0")}"

cat > "${WORK_DIR}/PklProject" <<PKL
amends "pkl:Project"

dependencies {
  ["tailscale"] = import("schema/pkl/PklProject")
  ["formae"] {
    uri = "package://hub.platform.engineering/plugins/pkl/schema/pkl/formae/formae@${FORMAE_VERSION}"
  }
}
PKL

cat > "${WORK_DIR}/main.pkl" <<PKL
// SPDX-License-Identifier: Apache-2.0

amends "@formae/forma.pkl"

import "@formae/formae.pkl"
import "@tailscale/tailscale.pkl"
import "@tailscale/events/webhook.pkl"
import "@tailscale/iam/auth_key.pkl"
import "@tailscale/iam/oauth_client.pkl"
import "@tailscale/network/service.pkl"

forma {
  new formae.Stack {
    label = "${STACK_LABEL}"
    description = "Tailscale plugin local E2E test"
  }

  new formae.Target {
    label = "tailscale-e2e"
    config = new tailscale.Config {
      apiKey = read?("env:TAILSCALE_API_KEY")
      oauthClientID = read?("env:TAILSCALE_OAUTH_CLIENT_ID")
      oauthClientSecret = read?("env:TAILSCALE_OAUTH_CLIENT_SECRET")
      tailnet = read?("env:TAILSCALE_TAILNET") ?? "-"
    }
  }

  new auth_key.AuthKey {
    label = "auth-key-${RUN_ID}"
    reusable = true
    ephemeral = false
    preauthorized = false
    expirySeconds = 3600
    description = "formae-test-${RUN_ID}-auth-key"
  }

  new oauth_client.OAuthClient {
    label = "oauth-client-${RUN_ID}"
    description = "formae-test-${RUN_ID}-oauth-client"
    scopes = new Listing { "auth_keys:read" }
  }

  new webhook.Webhook {
    label = "webhook-${RUN_ID}"
    endpointUrl = "https://example.com/formae-test-${RUN_ID}-webhook"
    providerType = "slack"
    subscriptions = new Listing { "nodeCreated" }
  }

  new service.Service {
    label = "service-${RUN_ID}"
    name = "svc:formae-test-${RUN_ID}-service"
    comment = "formae-test-${RUN_ID}-service"
    ports = new Listing { "443" }
  }
}
PKL

echo "E2E workspace: ${WORK_DIR}"
echo "Installing local plugin..."
make -C "${ROOT_DIR}" install

echo "Evaluating forma..."
formae eval "${WORK_DIR}/main.pkl" >/dev/null

echo "Simulating apply..."
formae apply --mode reconcile --simulate --yes "${WORK_DIR}/main.pkl"

echo "Applying resources..."
TAILSCALE_PLUGIN_DEBUG="${TAILSCALE_PLUGIN_DEBUG:-1}" formae apply --mode reconcile --yes --watch "${WORK_DIR}/main.pkl"

echo "Checking inventory..."
formae inventory resources --query "stack:${STACK_LABEL}" --max-results 20

echo "Destroying resources..."
formae destroy --yes --watch "${WORK_DIR}/main.pkl"
trap - EXIT

echo "Sweeping leftovers..."
TEST_PREFIX="formae-test-${RUN_ID}" "${ROOT_DIR}/scripts/ci/clean-environment.sh"

echo "Local E2E complete."
