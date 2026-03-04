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

# Demonstrates what goes wrong when a standard Gateway listener has a
# hostname set while being used for egress traffic.
#
# The translator produces a virtual host with domains: ["api.example.com"]
# instead of domains: ["*"]. Egress clients send Host headers like
# "Host: localhost:PORT" or "Host: httpbin.org" — neither matches, so
# traffic gets 404'd.
#
# Prerequisites: the httpbin demo must be running.
#   ./demo/httpbin/setup.sh
#
# Usage: ./demo/hostname-conflict/demo.sh
# Run from the egress-gateway-control-plane/ directory.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROL_PLANE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BACKEND_DIR="$(cd "${CONTROL_PLANE_DIR}/../backend-control-plane" && pwd)"
PROTOTYPE_ROOT="$(cd "${CONTROL_PLANE_DIR}/.." && pwd)"
CONTEXT="kind-wg-ai-gateway"
PASS=0
FAIL=0
PF_PID=""

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

cleanup() {
  echo ""
  echo "==> Cleaning up..."

  # Kill port-forwards
  if [ -n "${PF_PID:-}" ] && kill -0 "${PF_PID}" 2>/dev/null; then
    kill "${PF_PID}" 2>/dev/null || true
    wait "${PF_PID}" 2>/dev/null || true
  fi
  if [ -n "${PF_PID2:-}" ] && kill -0 "${PF_PID2}" 2>/dev/null; then
    kill "${PF_PID2}" 2>/dev/null || true
    wait "${PF_PID2}" 2>/dev/null || true
  fi

  # Delete Gateway resources
  kubectl --context "${CONTEXT}" delete httproute httpbin-gateway-hostname-route --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete gateway egress-with-hostname --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete xbackenddestination httpbin-gw-backend --ignore-not-found=true 2>/dev/null || true
  # The gateway controller's deployer created infra for this gateway — clean it up.
  kubectl --context "${CONTEXT}" delete deploy envoy-egress-with-hostname --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete svc envoy-egress-with-hostname --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete configmap envoy-egress-with-hostname --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete serviceaccount envoy-egress-with-hostname --ignore-not-found=true 2>/dev/null || true

  # Remove backend controller deployment (leave the egress controller's resources intact)
  kubectl --context "${CONTEXT}" delete deployment ai-gateway-controller -n ai-gateway-system --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete serviceaccount ai-gateway-controller -n ai-gateway-system --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete clusterrolebinding ai-gateway-controller --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete clusterrole ai-gateway-controller --ignore-not-found=true 2>/dev/null || true
  kubectl --context "${CONTEXT}" delete gatewayclass wg-ai-gateway --ignore-not-found=true 2>/dev/null || true

  # Restore the xDS Service selector to point back at the egress controller
  kubectl --context "${CONTEXT}" patch svc ai-gateway-controller -n ai-gateway-system \
    --type=json \
    -p='[{"op":"replace","path":"/spec/selector/app.kubernetes.io~1name","value":"ai-egress-gateway-controller"}]' \
    2>/dev/null || true

  # Restart the egress controller so it re-pushes xDS to envoy-egress
  kubectl --context "${CONTEXT}" rollout restart deployment ai-egress-gateway-controller -n ai-gateway-system 2>/dev/null || true

  echo "  Cleanup complete."
}
trap cleanup EXIT

echo "========================================================================"
echo " Hostname-on-Listener Conflict Demo"
echo ""
echo " Shows that a Gateway listener with hostname set breaks egress routing"
echo " because the envoy virtual host only matches that specific hostname,"
echo " not the Host headers that egress clients actually send."
echo "========================================================================"

# --------------------------------------------------------------------------
# Phase 1: Show the working EgressGateway (no hostname → domains: ["*"])
# --------------------------------------------------------------------------

echo ""
echo "--- Phase 1: Working EgressGateway (no hostname on listener) ---"
echo ""

# Verify the egress setup is running
EGRESS_READY=$(kubectl --context "${CONTEXT}" get egressgateway egress -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || true)
if [ "${EGRESS_READY}" != "True" ]; then
  echo "Error: EgressGateway 'egress' is not Programmed. Run ./demo/httpbin/setup.sh first." >&2
  exit 1
fi

# Get the envoy config to show domains: ["*"]
ENVOY_POD=$(kubectl --context "${CONTEXT}" get pods -l app=envoy-egress -o jsonpath='{.items[0].metadata.name}')
kubectl --context "${CONTEXT}" port-forward "pod/${ENVOY_POD}" 15100:15000 &>/dev/null &
PF_PID=$!
sleep 3

echo "EgressGateway listener (no hostname):"
echo ""
echo "  spec:"
echo "    listeners:"
echo "    - name: http"
echo "      port: 80"
echo '      protocol: HTTP     # <-- no hostname field'
echo ""

EGRESS_DOMAINS=$(curl -s http://localhost:15100/config_dump 2>/dev/null | python3 -c "
import json, sys
dump = json.load(sys.stdin)
for cfg in dump.get('configs', []):
    if 'ListenersConfigDump' in cfg.get('@type', ''):
        for l in cfg.get('dynamic_listeners', []):
            ls = l.get('active_state', {}).get('listener', {})
            for fc in ls.get('filter_chains', []):
                for f in fc.get('filters', []):
                    rc = f.get('typed_config', {}).get('route_config', {})
                    for vh in rc.get('virtual_hosts', []):
                        print(vh.get('domains'))
" 2>/dev/null)

echo "Resulting envoy virtual host domains: ${EGRESS_DOMAINS}"
echo ""

# Test traffic
kubectl --context "${CONTEXT}" port-forward svc/envoy-egress 18080:80 &>/dev/null &
PF_PID2=$!
sleep 2

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18080/get 2>/dev/null || true)
if [ "${HTTP_CODE}" = "200" ]; then
  pass "EgressGateway: GET /get returned HTTP 200 (domains: [\"*\"] matches any Host header)"
else
  fail "EgressGateway: GET /get returned HTTP ${HTTP_CODE:-no response} (expected 200)"
fi

# Clean up phase 1 port-forwards
kill "${PF_PID}" "${PF_PID2}" 2>/dev/null || true
wait "${PF_PID}" "${PF_PID2}" 2>/dev/null || true
PF_PID=""
PF_PID2=""

# --------------------------------------------------------------------------
# Phase 2: Deploy Gateway with hostname → broken egress
# --------------------------------------------------------------------------

echo ""
echo "--- Phase 2: Gateway with hostname on listener (broken egress) ---"
echo ""

echo "Building the backend-control-plane controller image..."
docker build -t localhost:5000/wg-ai-gateway:latest -f "${BACKEND_DIR}/Dockerfile" "${PROTOTYPE_ROOT}" --quiet
docker push localhost:5000/wg-ai-gateway:latest --quiet 2>/dev/null || docker push localhost:5000/wg-ai-gateway:latest

echo "Deploying backend-control-plane controller..."
kubectl --context "${CONTEXT}" apply -f "${BACKEND_DIR}/config/" 2>/dev/null

echo "Waiting for backend controller to be ready..."
kubectl --context "${CONTEXT}" wait --for=condition=available deployment/ai-gateway-controller \
  -n ai-gateway-system --timeout=60s 2>/dev/null

echo ""
echo "Creating Gateway with hostname on listener..."
echo ""
echo "  kind: Gateway"
echo "  spec:"
echo "    listeners:"
echo "    - name: egress-http"
echo '      hostname: "api.example.com"  # <-- this is the problem'
echo "      port: 80"
echo "      protocol: HTTP"
echo ""

kubectl --context "${CONTEXT}" apply -f - <<'EOF'
apiVersion: ainetworking.prototype.x-k8s.io/v0alpha0
kind: XBackendDestination
metadata:
  name: httpbin-gw-backend
  namespace: default
spec:
  destination:
    type: Fqdn
    fqdn:
      hostname: httpbin.org
    ports:
    - number: 443
      protocol: HTTP
      tls:
        mode: Simple
        insecureSkipVerify: true
        sni: httpbin.org
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: egress-with-hostname
  namespace: default
spec:
  gatewayClassName: wg-ai-gateway
  listeners:
  - name: egress-http
    hostname: "api.example.com"
    port: 80
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: httpbin-gateway-hostname-route
  namespace: default
spec:
  parentRefs:
  - name: egress-with-hostname
    kind: Gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: "/get"
    backendRefs:
    - name: httpbin-gw-backend
      kind: Backend
      group: ainetworking.prototype.x-k8s.io
      port: 443
EOF

echo ""
echo "Waiting for Gateway to be programmed..."
for i in $(seq 1 30); do
  GW_STATUS=$(kubectl --context "${CONTEXT}" get gateway egress-with-hostname \
    -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || true)
  if [ "${GW_STATUS}" = "True" ]; then
    break
  fi
  sleep 2
done

if [ "${GW_STATUS}" != "True" ]; then
  echo "Warning: Gateway not yet Programmed after 60s. Continuing anyway..." >&2
fi

echo "Waiting for envoy proxy pod..."
for i in $(seq 1 30); do
  ENVOY_READY=$(kubectl --context "${CONTEXT}" get pods -l app=envoy-egress-with-hostname \
    -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)
  if [ "${ENVOY_READY}" = "True" ]; then
    break
  fi
  sleep 2
done

if [ "${ENVOY_READY}" != "True" ]; then
  echo "Error: Envoy proxy pod for Gateway not ready. Check controller logs." >&2
  kubectl --context "${CONTEXT}" logs -n ai-gateway-system deploy/ai-gateway-controller -c manager --tail=20 2>/dev/null || true
  exit 1
fi

echo ""

# Get the envoy config to show the broken domains
GW_ENVOY_POD=$(kubectl --context "${CONTEXT}" get pods -l app=envoy-egress-with-hostname \
  -o jsonpath='{.items[0].metadata.name}')
kubectl --context "${CONTEXT}" port-forward "pod/${GW_ENVOY_POD}" 15200:15000 &>/dev/null &
PF_PID=$!
sleep 3

GW_VHOSTS=$(curl -s http://localhost:15200/config_dump 2>/dev/null | python3 -c "
import json, sys
dump = json.load(sys.stdin)
for cfg in dump.get('configs', []):
    if 'ListenersConfigDump' in cfg.get('@type', ''):
        for l in cfg.get('dynamic_listeners', []):
            ls = l.get('active_state', {}).get('listener', {})
            for fc in ls.get('filter_chains', []):
                for f in fc.get('filters', []):
                    rc = f.get('typed_config', {}).get('route_config', {})
                    for vh in rc.get('virtual_hosts', []):
                        print('  vhost:', vh.get('name'))
                        print('  domains:', vh.get('domains'))
                        for route in vh.get('routes', []):
                            m = route.get('match', {})
                            cluster = route.get('route', {}).get('cluster') or \
                                route.get('route', {}).get('weighted_clusters', {}).get('clusters', [{}])[0].get('name', '?')
                            print('  route:', m, '->', cluster)
" 2>/dev/null)

echo "Resulting envoy virtual host:"
echo "${GW_VHOSTS}"
echo ""

# Test traffic — this should fail because Host header won't match "api.example.com"
kubectl --context "${CONTEXT}" port-forward svc/envoy-egress-with-hostname 18081:80 &>/dev/null &
PF_PID2=$!
sleep 2

echo "Sending: curl http://localhost:18081/get"
echo "  (curl sends Host: localhost:18081 — does NOT match api.example.com)"
echo ""

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18081/get 2>/dev/null || true)
if [ "${HTTP_CODE}" = "404" ]; then
  pass "Gateway with hostname: GET /get returned HTTP 404 (Host header mismatch)"
else
  fail "Gateway with hostname: GET /get returned HTTP ${HTTP_CODE:-no response} (expected 404)"
fi

echo ""

# Show that it works IF you match the hostname
echo "Sending: curl -H 'Host: api.example.com' http://localhost:18081/get"
echo "  (manually forcing Host to match the listener hostname)"
echo ""

HTTP_CODE2=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: api.example.com" http://localhost:18081/get 2>/dev/null || true)
if [ "${HTTP_CODE2}" = "200" ]; then
  pass "Gateway with hostname: GET /get with matching Host header returned HTTP 200"
else
  fail "Gateway with hostname: GET /get with matching Host returned HTTP ${HTTP_CODE2:-no response} (expected 200)"
fi

kill "${PF_PID}" "${PF_PID2}" 2>/dev/null || true
wait "${PF_PID}" "${PF_PID2}" 2>/dev/null || true
PF_PID=""
PF_PID2=""

# --------------------------------------------------------------------------
# Summary
# --------------------------------------------------------------------------

echo ""
echo "========================================================================"
echo " Side-by-side comparison"
echo ""
echo "  EgressGateway listener (no hostname):"
echo "    → envoy vhost domains: [\"*\"]"
echo "    → matches ANY Host header"
echo "    → egress clients work naturally"
echo ""
echo "  Gateway listener with hostname: \"api.example.com\":"
echo "    → envoy vhost domains: [\"api.example.com\"]"
echo "    → ONLY matches Host: api.example.com"
echo "    → egress clients get 404 (they send Host: localhost, or Host: <svc-ip>)"
echo ""
echo " For ingress, hostname is essential — it routes by incoming SNI/Host."
echo " For egress, hostname is actively harmful — it breaks client traffic."
echo " EgressGateway prevents this mistake at the CRD schema level."
echo "========================================================================"
echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ "${FAIL}" -gt 0 ]; then
  exit 1
fi
