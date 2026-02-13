# Development Environment Setup

This document provides instructions for setting up a local development environment for the WG AI Gateway controller.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Go](https://golang.org/doc/install) (1.21+)

## Quick Start

### Automated Setup

Run the development setup script to create a complete environment:

```bash
./hack/dev-setup.sh
```

This will:
1. Create a Kind cluster with a local registry in the kind network
2. Install Gateway API CRDs
3. Install AI Gateway CRDs
4. Install MetalLB for LoadBalancer services
5. Build and deploy the controller

### Manual Setup

If you prefer to run steps manually:

```bash
# Create Kind cluster with local registry
make kind-cluster

# Install MetalLB for LoadBalancer services
make metallb-install

# Install Gateway API CRDs
make gateway-api-install

# Build and deploy the controller
make build
make docker-build-local
make deploy-local
```

## Testing

### Deploy Examples

Deploy sample resources to test the controller:

```bash
# Deploy example Gateway and Backend (FQDN)
make example

# Or deploy Kubernetes service example
kubectl --context kind-wg-ai-gateway apply -f config/samples/kubernetes-service.yaml
```

### Test Endpoints

After deploying examples, get the LoadBalancer IP and test:

```bash
# Get the external IP assigned by MetalLB
kubectl --context kind-wg-ai-gateway get svc httpbin-gateway-service

# Test FQDN backend (replace EXTERNAL-IP with actual IP)
curl http://EXTERNAL-IP/get

# Test Kubernetes service backend
curl http://EXTERNAL-IP/echo
```

### View Logs

Monitor controller logs:

```bash
make logs
```

## Development Workflow

### Code Changes

1. Make your code changes
2. Rebuild and redeploy:
   ```bash
   make build
   make docker-build-local
   make deploy-local
   ```

### Testing Changes

1. Update or create test resources in `config/samples/`
2. Apply changes:
   ```bash
   kubectl --context kind-wg-ai-gateway apply -f config/samples/your-test.yaml
   ```
3. Monitor logs and test endpoints

## Useful Commands

### Controller Management

```bash
make build                 # Build the controller binary
make docker-build-local    # Build and push to local registry
make deploy-local          # Deploy to Kind cluster
make logs                  # View controller logs
```

### Environment Management

```bash
make dev-setup             # Complete environment setup
make dev-teardown          # Clean up everything
make kind-cluster          # Create just the Kind cluster
make registry-setup        # Set up local registry
```

### Resource Management

```bash
# View resources
kubectl --context kind-wg-ai-gateway get gateways
kubectl --context kind-wg-ai-gateway get httproutes
kubectl --context kind-wg-ai-gateway get backends

# Describe resources for debugging
kubectl --context kind-wg-ai-gateway describe gateway httpbin-gateway
kubectl --context kind-wg-ai-gateway describe httproute httpbin-route
```

## Troubleshooting

### Common Issues

1. **LoadBalancer IP pending**: Check MetalLB installation with `kubectl --context kind-wg-ai-gateway get pods -n metallb-system`
2. **Registry issues**: Ensure Docker is running and registry is in kind network
3. **Image not found**: Rebuild and push with `make docker-build-local`

### Debug Steps

1. Check controller logs: `make logs`
2. Verify resources exist: `kubectl --context kind-wg-ai-gateway get all -A`
3. Check CRDs installed: `kubectl --context kind-wg-ai-gateway get crds`

### Clean Restart

```bash
make dev-teardown
make dev-setup
```

## Architecture

The development environment includes:

- **Kind cluster**: Lightweight Kubernetes cluster
- **Local registry**: Docker registry accessible as `kind-registry:5000` within the cluster
- **MetalLB**: LoadBalancer implementation for Kind clusters
- **Gateway API**: Standard Gateway API CRDs
- **AI Gateway CRDs**: Custom Backend CRD
- **Controller**: AI Gateway controller managing Envoy proxies
- **Examples**: Sample Gateway, HTTPRoute, and Backend resources with LoadBalancer services

## Next Steps

- Review the [controller implementation](pkg/controllers/)
- Explore [translator logic](pkg/translator/envoy/)
- Add new Backend types or Gateway features
- Contribute back to the project!
