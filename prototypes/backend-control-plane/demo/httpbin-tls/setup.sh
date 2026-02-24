#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Sets up the httpbin TLS demo environment:
#   Kind cluster, controller, Gateway (port 8080), Backend (TLS to httpbin.org:443),
#   and HTTPRoute.
#
# Usage: ./demo/httpbin-tls/setup.sh
# Run from the backend-control-plane/ directory.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROL_PLANE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CONTEXT="kind-wg-ai-gateway"

cd "${CONTROL_PLANE_DIR}"

# Check prerequisites
for cmd in docker kind kubectl; do
  if ! command -v "${cmd}" &>/dev/null; then
    echo "Error: ${cmd} is required but not installed." >&2
    exit 1
  fi
done

echo "==> Setting up dev environment (Kind cluster, registry, MetalLB, CRDs, controller)..."
make dev-setup

echo "==> Deploying TLS example Gateway, Backend, and HTTPRoute..."
kubectl --context "${CONTEXT}" apply -f config/samples/tls/

echo "==> Waiting for envoy proxy pod to be ready..."
kubectl --context "${CONTEXT}" wait --for=condition=ready pod -l app.kubernetes.io/component=proxy --timeout=120s 2>/dev/null || \
  kubectl --context "${CONTEXT}" wait --for=condition=ready pod -l app=envoy-httpbin-tls-gateway --timeout=120s 2>/dev/null || \
  sleep 15  # fallback: give the deployer time to create and start the proxy

echo "==> Waiting for Gateway to be programmed..."
for i in $(seq 1 30); do
  STATUS=$(kubectl --context "${CONTEXT}" get gateway httpbin-tls-gateway -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || true)
  if [ "${STATUS}" = "True" ]; then
    break
  fi
  sleep 2
done

if [ "${STATUS}" != "True" ]; then
  echo "Warning: Gateway not yet Programmed after 60s. Check controller logs:" >&2
  echo "  make logs" >&2
fi

echo ""
echo "Setup complete. To test:"
echo "  ./demo/httpbin-tls/test-happy-path.sh"
echo ""
echo "To view logs:"
echo "  make logs"
echo ""
echo "To tear down:"
echo "  ./demo/httpbin-tls/teardown.sh"
