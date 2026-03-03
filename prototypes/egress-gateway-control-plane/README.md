# Egress Gateway Control Plane Prototype

A controller that manages Envoy proxies for a dedicated `EgressGateway` resource, separate from the standard Gateway API `Gateway`.

## What this explores

During Gateway API meeting discussions, the preference was expressed for a dedicated `EgressGateway` resource rather than reusing the standard `Gateway`. Several Gateway fields (`Hostname`, `Addresses`) are ingress-oriented, and a separate resource provides clearer RBAC boundaries, avoids mixed ingress/egress confusion, and allows the API to be shaped for egress-specific use cases.

This prototype:

- Defines an `EgressGateway` CRD in the `ainetworking.prototype.x-k8s.io` API group
- Watches `EgressGateway`, `HTTPRoute`, and `XBackendDestination` resources
- Converts `EgressGateway` to a standard `Gateway` object internally
- Reuses the existing backend-control-plane translator, deployer, and xDS packages unchanged

## Dependencies

This prototype imports packages from `backend-control-plane/`:

- `backend/api/v0alpha0` — CRD types (EgressGateway, XBackendDestination)
- `backend/k8s/client/` — Generated clientsets, informers, listers
- `pkg/translator/envoy` — Gateway API to Envoy xDS translation
- `pkg/deployer/envoy` — Envoy proxy deployment and lifecycle
- `pkg/xds/envoy` — gRPC xDS control plane server

## How to build and run

Prerequisites: Docker, Kind, kubectl, and the backend-control-plane dev environment.

```bash
# From this directory (prototypes/egress-gateway-control-plane/):

# Build the controller binary
make build

# Build and push Docker image to local registry
make docker-build-local

# Install CRDs and deploy controller to Kind cluster
make deploy-local

# Apply example EgressGateway + Backend + HTTPRoute
make example

# View controller logs
make logs
```

## Semantic differences from Gateway

| Concern | Gateway | EgressGateway |
|---------|---------|---------------|
| Hostname on listeners | Yes | No (egress has no frontend hostname) |
| Addresses in status | Yes | No |
| GatewayClass | `wg-ai-gateway` | `wg-ai-egress-gateway` |
| Controller name | `sigs.k8s.io/wg-ai-gateway-envoy-controller` | `sigs.k8s.io/wg-ai-egress-gateway-envoy-controller` |
| HTTPRoute parentRef kind | `Gateway` (default) | `EgressGateway` (explicit) |

## Known prototype limitations

- The translator's `listHTTPRoutesForGateway` matches by name/namespace only, not by kind. If an `EgressGateway` and `Gateway` share the same name in the same namespace, both controllers would process the same HTTPRoutes.
- Does not watch `GatewayClass` resources (simplification for prototype).
- The `BackendTLS` field on `EgressGatewaySpec` is defined but not yet wired into the translation pipeline.
