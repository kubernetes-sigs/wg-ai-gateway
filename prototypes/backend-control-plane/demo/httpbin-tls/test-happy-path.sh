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

# Tests the httpbin TLS demo happy path:
#   1. Verifies the Gateway is Programmed
#   2. Verifies the HTTPRoute is Accepted
#   3. Port-forwards to the envoy proxy and curls /get (routed to httpbin.org:443 via TLS)
#   4. Validates the response
#
# Prerequisites: run ./demo/httpbin-tls/setup.sh first.
#
# Usage: ./demo/httpbin-tls/test-happy-path.sh
# Run from the backend-control-plane/ directory.

set -euo pipefail

CONTEXT="kind-wg-ai-gateway"
LOCAL_PORT=8081
REMOTE_PORT=8080
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

cleanup() {
  if [ -n "${PF_PID:-}" ] && kill -0 "${PF_PID}" 2>/dev/null; then
    kill "${PF_PID}" 2>/dev/null || true
    wait "${PF_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "==> Checking resource status..."

# Check GatewayClass
GC_STATUS=$(kubectl --context "${CONTEXT}" get gatewayclass wg-ai-gateway -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null || true)
if [ "${GC_STATUS}" = "True" ]; then
  pass "GatewayClass wg-ai-gateway is Accepted"
else
  fail "GatewayClass wg-ai-gateway is not Accepted (status: ${GC_STATUS:-unknown})"
fi

# Check Gateway
GW_STATUS=$(kubectl --context "${CONTEXT}" get gateway httpbin-tls-gateway -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || true)
if [ "${GW_STATUS}" = "True" ]; then
  pass "Gateway httpbin-tls-gateway is Programmed"
else
  fail "Gateway httpbin-tls-gateway is not Programmed (status: ${GW_STATUS:-unknown})"
fi

# Check HTTPRoute
HR_STATUS=$(kubectl --context "${CONTEXT}" get httproute httpbin-tls-route -o jsonpath='{.status.parents[0].conditions[?(@.type=="Accepted")].status}' 2>/dev/null || true)
if [ "${HR_STATUS}" = "True" ]; then
  pass "HTTPRoute httpbin-tls-route is Accepted"
else
  fail "HTTPRoute httpbin-tls-route is not Accepted (status: ${HR_STATUS:-unknown})"
fi

# Check envoy proxy pod
ENVOY_READY=$(kubectl --context "${CONTEXT}" get pods -l app=envoy-httpbin-tls-gateway -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)
if [ "${ENVOY_READY}" = "True" ]; then
  pass "Envoy proxy pod is Ready"
else
  fail "Envoy proxy pod is not Ready (status: ${ENVOY_READY:-unknown})"
fi

echo ""
echo "==> Testing traffic routing (httpbin.org via upstream TLS)..."

# Start port-forward
kubectl --context "${CONTEXT}" port-forward svc/envoy-httpbin-tls-gateway "${LOCAL_PORT}:${REMOTE_PORT}" &>/dev/null &
PF_PID=$!
sleep 2

# Verify port-forward is running
if ! kill -0 "${PF_PID}" 2>/dev/null; then
  fail "Port-forward failed to start"
  echo ""
  echo "Results: ${PASS} passed, ${FAIL} failed"
  exit 1
fi

# Curl the /get endpoint (traffic goes: localhost -> envoy -> TLS -> httpbin.org:443)
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:${LOCAL_PORT}/get 2>/dev/null || true)
HTTP_CODE=$(echo "${RESPONSE}" | tail -1)
BODY=$(echo "${RESPONSE}" | sed '$d')

if [ "${HTTP_CODE}" = "200" ]; then
  pass "GET /get returned HTTP 200"
else
  fail "GET /get returned HTTP ${HTTP_CODE:-no response}"
fi

# Validate response body contains expected fields
if echo "${BODY}" | grep -q '"url"'; then
  pass "Response contains 'url' field"
else
  fail "Response missing 'url' field"
fi

if echo "${BODY}" | grep -q 'httpbin.org'; then
  pass "Response shows traffic routed to httpbin.org"
else
  fail "Response does not show httpbin.org routing"
fi

# Test that non-matching paths return 404
HTTP_404=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${LOCAL_PORT}/nonexistent 2>/dev/null || true)
if [ "${HTTP_404}" = "404" ]; then
  pass "GET /nonexistent returned HTTP 404 (route not matched)"
else
  fail "GET /nonexistent returned HTTP ${HTTP_404:-no response} (expected 404)"
fi

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ "${FAIL}" -gt 0 ]; then
  exit 1
fi
