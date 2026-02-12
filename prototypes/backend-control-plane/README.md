# Backend Control Plane Prototype

A Gateway API controller that manages Envoy proxies for AI Gateway backends using xDS.

## What this explores

This prototype implements a Kubernetes controller that:

- Watches Gateway, HTTPRoute, and XBackendDestination (Backend) resources
- Translates them into Envoy xDS configuration (listeners, clusters, routes)
- Deploys and manages Envoy proxy instances per Gateway
- Supports FQDN and Kubernetes Service backends with HTTP routing

It extends the Gateway API with a custom `XBackendDestination` CRD to represent individual egress backends with per-port protocol and TLS configuration.

## How to build and run

Prerequisites: Docker, Kind, kubectl.

```bash
# From this directory (prototypes/backend-control-plane/):

# Set up the full dev environment (Kind cluster, MetalLB, CRDs, controller)
make dev-setup

# Or step by step:
make kind-cluster          # Create Kind cluster
make registry-setup        # Start local Docker registry
make gateway-api-install   # Install Gateway API CRDs
make metallb-install       # Install MetalLB for LoadBalancer IPs
make build                 # Build the controller binary
make docker-build-local    # Build and push Docker image
make deploy-local          # Deploy controller + CRDs to cluster

# Deploy the example Gateway + Backend + HTTPRoute
make example

# View controller logs
make logs

# Tear down when done
make dev-teardown
```

## Directory structure

```
backend-control-plane/
  cmd/              Entry point for the controller binary
  pkg/
    controllers/    Kubernetes controller reconciliation logic
    translator/     Gateway API to Envoy xDS translation
    deployer/       Envoy proxy deployment and lifecycle
    xds/            gRPC xDS control plane server
    constants/      Shared constants
    schema/         CRD schema registration
  config/
    controller.yaml Controller deployment manifest (RBAC, Deployment, Service)
    samples/        Example Gateway, Backend, and HTTPRoute manifests
  hack/             Development and verification scripts
```

Cross-cutting Backend API types and generated clients live in `../internal/backend/`.

## Demo scripts

Automated scripts for the httpbin demo live in `demo/httpbin/`:

```bash
# Full setup: Kind cluster, controller, example resources
./demo/httpbin/setup.sh

# Test the happy path (port-forwards and curls /get)
./demo/httpbin/test-happy-path.sh

# Tear everything down
./demo/httpbin/teardown.sh
```

## Assumptions

- Targets Kind clusters for local development (MetalLB provides LoadBalancer IPs)
- HTTP-only routing for now (HTTPS/TLS termination is defined in the CRD but not yet implemented)
- Single Envoy proxy per Gateway

## Open questions

- How should the `Backend` kind in HTTPRoute backendRefs map to the `XBackendDestination` CRD long-term?
- Should TLS origination to backends be handled at the Envoy level or delegated to a sidecar?
- How should multiple controller instances coordinate on shared Backend resources?
