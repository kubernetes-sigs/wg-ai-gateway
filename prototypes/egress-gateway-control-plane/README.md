# Egress Gateway Control Plane Prototype

A controller that manages Envoy proxies for a dedicated `EgressGateway` resource, separate from the standard Gateway API `Gateway`.

## What this explores

During Gateway API meeting discussions, the preference was expressed for a dedicated `EgressGateway` resource rather than reusing the standard `Gateway`. Several Gateway fields (`Hostname`, `Addresses`) are ingress-oriented, and a separate resource provides clearer RBAC boundaries, avoids mixed ingress/egress confusion, and allows the API to be shaped for egress-specific use cases.

This prototype:

- Defines an `EgressGateway` CRD in the `ainetworking.prototype.x-k8s.io` API group
- Watches `EgressGateway`, `HTTPRoute`, and `XBackendDestination` resources
- Converts `EgressGateway` to a standard `Gateway` object internally via an adapter function
- Reuses the existing backend-control-plane translator, deployer, and xDS packages unchanged

## Dependencies

This prototype imports packages from `backend-control-plane/`:

- `backend/api/v0alpha0` — CRD types (EgressGateway, XBackendDestination)
- `backend/k8s/client/` — Generated clientsets, informers, listers
- `pkg/translator/envoy` — Gateway API to Envoy xDS translation
- `pkg/deployer/envoy` — Envoy proxy deployment and lifecycle
- `pkg/xds/envoy` — gRPC xDS control plane server
- `pkg/constants` — Shared constants (notably `XDSServerServiceName` and `XDSServerPort`)

## How to build and run

Prerequisites: Docker, Kind, and kubectl.

```bash
# From this directory (prototypes/egress-gateway-control-plane/):

# Full environment setup: Kind cluster, local registry, MetalLB, Gateway API CRDs,
# build, and deploy the controller — all in one command:
make dev-setup

# Apply example EgressGateway + Backend + HTTPRoute (httpbin.org, port 80)
make example

# Apply TLS example (httpbin.org, port 8080)
make example-tls

# View controller logs
make logs

# Tear down everything (Kind cluster + registry)
make dev-teardown
```

Individual targets are also available if you need finer control:

```bash
make build              # Build the controller binary
make docker-build-local # Build and push Docker image to local registry
make deploy-local       # Install CRDs and deploy controller to Kind cluster
```

## Demos

End-to-end demo scripts are provided under `demo/`. Each demo handles full environment setup, traffic testing, and teardown:

```bash
# httpbin via egress (plaintext HTTP listener on port 80, TLS to upstream)
./demo/httpbin/setup.sh
./demo/httpbin/test-happy-path.sh
./demo/httpbin/teardown.sh

# httpbin via egress (listener on port 8080, TLS to upstream)
./demo/httpbin-tls/setup.sh
./demo/httpbin-tls/test-happy-path.sh
./demo/httpbin-tls/teardown.sh
```

## Semantic differences from Gateway

| Concern | Gateway | EgressGateway |
|---------|---------|---------------|
| Hostname on listeners | Yes | No (egress has no frontend hostname) |
| Addresses in status | Yes | No |
| GatewayClass | `wg-ai-gateway` | `wg-ai-egress-gateway` |
| Controller name | `sigs.k8s.io/wg-ai-gateway-envoy-controller` | `sigs.k8s.io/wg-ai-egress-gateway-envoy-controller` |
| HTTPRoute parentRef kind | `Gateway` (default) | `EgressGateway` (explicit) |
| ownerReferences on managed resources | Set (GC cleans up on Gateway delete) | Not set (controller handles cleanup in delete handler) |

## Debugging

```bash
# Resource status
kubectl get egressgateway <name> -o yaml
kubectl get httproute <name> -o yaml
kubectl get xbackenddestination <name> -o yaml

# Controller logs
make logs
# or: kubectl logs -n ai-gateway-system deploy/ai-egress-gateway-controller -c manager -f

# Envoy admin (port-forward to the pod — the Service only exposes the listener port)
kubectl port-forward pod/<envoy-pod> 15000:15000
curl localhost:15000/config_dump | python3 -m json.tool   # full xDS state
curl localhost:15000/clusters                              # upstream cluster status
curl localhost:15000/listeners                             # active listeners
```

## Known prototype limitations

- **HTTPRoute matching by name only.** The translator's `listHTTPRoutesForGateway` matches by name/namespace, not by kind. If an `EgressGateway` and `Gateway` share the same name in the same namespace, both controllers would process the same HTTPRoutes.
- **No GatewayClass reconciliation.** The controller does not watch or update `GatewayClass` resources. The GatewayClass status will show `Pending` / `Waiting for controller` — this has no effect on functionality.
- **BackendTLS not wired.** The `BackendTLS` field on `EgressGatewaySpec` is defined but not yet plumbed into the translation pipeline. Backend TLS is configured per-port on the `XBackendDestination` instead.
- **No ownerReferences on managed resources.** The deployer template hardcodes `kind: Gateway` in ownerReferences. Since no `Gateway` exists, the adapter omits the UID so ownerReferences are skipped (the template conditionally omits them when `GatewayUID` is empty). The controller's delete handler cleans up managed resources instead.
- **xDS Service name is shared.** The envoy bootstrap template uses `constants.XDSServerServiceName` (`ai-gateway-controller`) to connect to the xDS server. The egress controller's Service must use this name, not a controller-specific name.
