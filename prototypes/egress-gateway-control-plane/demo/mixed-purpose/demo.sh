#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
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

# Demonstrates that EgressGateway structurally prevents mixed-purpose
# gateway configurations that the standard Gateway resource allows.
#
# This demo uses --dry-run=server so it validates against the real CRD
# schemas without creating any resources. It requires Gateway API CRDs
# and the EgressGateway CRD to be installed.
#
# Prerequisites: a running cluster with CRDs installed.
#   Either run ./demo/httpbin/setup.sh first, or:
#     make dev-setup
#
# Usage: ./demo/mixed-purpose/demo.sh
# Run from the egress-gateway-control-plane/ directory.

set -euo pipefail

CONTEXT="kind-wg-ai-gateway"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

echo "========================================================================"
echo " Mixed-Purpose Gateway Prevention Demo"
echo ""
echo " Shows that EgressGateway structurally rejects ingress-oriented fields"
echo " that the standard Gateway accepts without complaint."
echo "========================================================================"

# --------------------------------------------------------------------------
# Part 1: EgressGateway rejects ingress-oriented fields
# --------------------------------------------------------------------------

echo ""
echo "--- Part 1: EgressGateway rejects ingress-oriented fields ---"
echo ""

# Test 1: hostname on listener
echo "1) Attempting to create EgressGateway with hostname on a listener..."
echo ""
cat <<'YAML'
   kind: EgressGateway
   spec:
     listeners:
     - name: ingress-https
       hostname: "api.example.com"     # <-- ingress-oriented
       port: 443
       protocol: HTTPS
YAML
echo ""

OUTPUT=$(kubectl --context "${CONTEXT}" apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: ainetworking.prototype.x-k8s.io/v0alpha0
kind: EgressGateway
metadata:
  name: mixed-test
  namespace: default
spec:
  gatewayClassName: wg-ai-egress-gateway
  listeners:
  - name: ingress-https
    hostname: "api.example.com"
    port: 443
    protocol: HTTPS
EOF
)

if echo "${OUTPUT}" | grep -q 'unknown field "spec.listeners\[0\].hostname"'; then
  pass "Rejected: unknown field \"spec.listeners[].hostname\""
  echo "         ${OUTPUT}" | head -1
else
  fail "Expected rejection of hostname field"
  echo "         ${OUTPUT}"
fi

echo ""

# Test 2: addresses on spec
echo "2) Attempting to create EgressGateway with spec.addresses..."
echo ""
cat <<'YAML'
   kind: EgressGateway
   spec:
     addresses:                         # <-- ingress-oriented
     - type: IPAddress
       value: "10.0.0.1"
     listeners:
     - name: http
       port: 80
       protocol: HTTP
YAML
echo ""

OUTPUT=$(kubectl --context "${CONTEXT}" apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: ainetworking.prototype.x-k8s.io/v0alpha0
kind: EgressGateway
metadata:
  name: mixed-test
  namespace: default
spec:
  gatewayClassName: wg-ai-egress-gateway
  addresses:
  - type: IPAddress
    value: "10.0.0.1"
  listeners:
  - name: http
    port: 80
    protocol: HTTP
EOF
)

if echo "${OUTPUT}" | grep -q 'unknown field "spec.addresses"'; then
  pass "Rejected: unknown field \"spec.addresses\""
  echo "         ${OUTPUT}" | head -1
else
  fail "Expected rejection of addresses field"
  echo "         ${OUTPUT}"
fi

echo ""

# Test 3: both hostname and addresses
echo "3) Attempting to create EgressGateway with both hostname and addresses..."
echo ""
cat <<'YAML'
   kind: EgressGateway
   spec:
     addresses:                         # <-- ingress-oriented
     - type: IPAddress
       value: "10.0.0.1"
     listeners:
     - name: ingress-https
       hostname: "api.example.com"     # <-- ingress-oriented
       port: 443
       protocol: HTTPS
     - name: egress-http
       port: 8080
       protocol: HTTP
YAML
echo ""

OUTPUT=$(kubectl --context "${CONTEXT}" apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: ainetworking.prototype.x-k8s.io/v0alpha0
kind: EgressGateway
metadata:
  name: mixed-test
  namespace: default
spec:
  gatewayClassName: wg-ai-egress-gateway
  addresses:
  - type: IPAddress
    value: "10.0.0.1"
  listeners:
  - name: ingress-https
    hostname: "api.example.com"
    port: 443
    protocol: HTTPS
  - name: egress-http
    port: 8080
    protocol: HTTP
EOF
)

if echo "${OUTPUT}" | grep -q 'unknown field'; then
  pass "Rejected: multiple unknown fields"
  echo "         ${OUTPUT}" | head -1
else
  fail "Expected rejection of mixed fields"
  echo "         ${OUTPUT}"
fi

# --------------------------------------------------------------------------
# Part 2: Standard Gateway accepts mixed-purpose config
# --------------------------------------------------------------------------

echo ""
echo "--- Part 2: Standard Gateway accepts the same mixed-purpose config ---"
echo ""

echo "4) Creating a Gateway with hostname, addresses, AND an egress-style listener..."
echo ""
cat <<'YAML'
   kind: Gateway
   spec:
     addresses:
     - type: IPAddress
       value: "10.0.0.1"
     listeners:
     - name: ingress-https              # ingress-style
       hostname: "api.example.com"
       port: 443
       protocol: HTTPS
     - name: egress-http                # egress-style
       port: 8080
       protocol: HTTP
YAML
echo ""

OUTPUT=$(kubectl --context "${CONTEXT}" apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: mixed-test
  namespace: default
spec:
  gatewayClassName: wg-ai-egress-gateway
  addresses:
  - type: IPAddress
    value: "10.0.0.1"
  listeners:
  - name: ingress-https
    hostname: "api.example.com"
    port: 443
    protocol: HTTPS
  - name: egress-http
    port: 8080
    protocol: HTTP
EOF
)

if echo "${OUTPUT}" | grep -q 'created (server dry run)'; then
  pass "Accepted: Gateway allows mixed ingress + egress listeners"
  echo "         ${OUTPUT}"
else
  fail "Expected Gateway to accept mixed config"
  echo "         ${OUTPUT}"
fi

# --------------------------------------------------------------------------
# Part 3: Side-by-side schema comparison
# --------------------------------------------------------------------------

echo ""
echo "--- Part 3: CRD schema comparison ---"
echo ""

GW_SPEC_FIELDS=$(kubectl --context "${CONTEXT}" get crd gateways.gateway.networking.k8s.io \
  -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties}' 2>/dev/null \
  | python3 -c "import json,sys; print(', '.join(sorted(json.load(sys.stdin).keys())))")

EG_SPEC_FIELDS=$(kubectl --context "${CONTEXT}" get crd egressgateways.ainetworking.prototype.x-k8s.io \
  -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties}' 2>/dev/null \
  | python3 -c "import json,sys; print(', '.join(sorted(json.load(sys.stdin).keys())))")

GW_LISTENER_FIELDS=$(kubectl --context "${CONTEXT}" get crd gateways.gateway.networking.k8s.io \
  -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.listeners.items.properties}' 2>/dev/null \
  | python3 -c "import json,sys; print(', '.join(sorted(json.load(sys.stdin).keys())))")

EG_LISTENER_FIELDS=$(kubectl --context "${CONTEXT}" get crd egressgateways.ainetworking.prototype.x-k8s.io \
  -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.listeners.items.properties}' 2>/dev/null \
  | python3 -c "import json,sys; print(', '.join(sorted(json.load(sys.stdin).keys())))")

echo "  Gateway spec fields:        ${GW_SPEC_FIELDS}"
echo "  EgressGateway spec fields:   ${EG_SPEC_FIELDS}"
echo ""
echo "  Gateway listener fields:     ${GW_LISTENER_FIELDS}"
echo "  EgressGateway listener fields: ${EG_LISTENER_FIELDS}"
echo ""
echo "  Removed from spec:     addresses"
echo "  Added to spec:         backendTLS"
echo "  Removed from listener: hostname"

# --------------------------------------------------------------------------
# Summary
# --------------------------------------------------------------------------

echo ""
echo "========================================================================"
echo " Summary"
echo ""
echo " The standard Gateway resource accepts mixed ingress/egress configs"
echo " without complaint — hostname, addresses, and egress listeners can"
echo " all coexist in one resource. Nothing prevents a user from creating"
echo " a confusing mixed-purpose Gateway."
echo ""
echo " EgressGateway rejects these fields at the CRD schema level."
echo " The API surface is structurally limited to egress concerns."
echo "========================================================================"
echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ "${FAIL}" -gt 0 ]; then
  exit 1
fi
