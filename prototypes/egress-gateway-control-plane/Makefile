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

export GO111MODULE=on

PROTOTYPE_ROOT := $(dir $(CURDIR))
BACKEND_CRD_DIR := $(PROTOTYPE_ROOT)/backend-control-plane/backend/k8s/crds

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the egress-gateway controller binary.
	cd $(PROTOTYPE_ROOT) && go build -o egress-gateway-control-plane/bin/manager ./egress-gateway-control-plane/cmd/main.go

##@ Development Environment

.PHONY: dev-setup
dev-setup: kind-cluster registry-setup gateway-api-install metallb-install build docker-build-local deploy-local ## Complete development environment setup

.PHONY: dev-teardown
dev-teardown: ## Tear down development environment
	@echo "Tearing down development environment..."
	kind delete cluster --name wg-ai-gateway || true
	docker stop kind-registry || true
	docker rm kind-registry || true

.PHONY: kind-cluster
kind-cluster: ## Create a Kind cluster with local registry
	@echo "Creating Kind cluster with local registry..."
	@if ! docker ps | grep -q "kind-registry"; then \
		docker network create kind || true; \
		docker run -d --restart=always -p 5000:5000 --name "kind-registry" --network=kind registry:2; \
	else \
		echo "Registry already running"; \
	fi
	@if ! kind get clusters | grep -q "wg-ai-gateway"; then \
		kind create cluster -v6 --name wg-ai-gateway --config=$(PROTOTYPE_ROOT)/backend-control-plane/hack/kind-config.yaml; \
		kubectl --context kind-wg-ai-gateway annotate node wg-ai-gateway-control-plane "kind.x-k8s.io/registry=kind-registry:5000"; \
	else \
		echo "Kind cluster 'wg-ai-gateway' already exists"; \
	fi

.PHONY: registry-setup
registry-setup: ## Setup local registry for Kind
	@echo "Setting up local registry in kind network..."
	@if ! docker ps | grep -q "kind-registry"; then \
		docker network create kind || true; \
		docker run -d --restart=always -p 5000:5000 --name "kind-registry" --network=kind registry:2; \
	else \
		echo "Registry already running"; \
	fi

.PHONY: metallb-install
metallb-install: ## Install MetalLB for LoadBalancer services
	@echo "Installing MetalLB..."
	kubectl --context kind-wg-ai-gateway apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml
	sleep 5
	@echo "Waiting for MetalLB to be ready..."
	kubectl --context kind-wg-ai-gateway wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s
	@echo "Configuring MetalLB address pool..."
	@SUBNET=$$(docker network inspect kind | jq -r '.[0].IPAM.Config[] | select(.Subnet | contains(":") | not) | .Subnet' | head -1 | sed 's|\([0-9]*\.[0-9]*\)\..*|\1.255.248/28|'); \
	echo "apiVersion: metallb.io/v1beta1" > /tmp/metallb-config.yaml; \
	echo "kind: IPAddressPool" >> /tmp/metallb-config.yaml; \
	echo "metadata:" >> /tmp/metallb-config.yaml; \
	echo "  name: example" >> /tmp/metallb-config.yaml; \
	echo "  namespace: metallb-system" >> /tmp/metallb-config.yaml; \
	echo "spec:" >> /tmp/metallb-config.yaml; \
	echo "  addresses:" >> /tmp/metallb-config.yaml; \
	echo "  - $$SUBNET" >> /tmp/metallb-config.yaml; \
	echo "---" >> /tmp/metallb-config.yaml; \
	echo "apiVersion: metallb.io/v1beta1" >> /tmp/metallb-config.yaml; \
	echo "kind: L2Advertisement" >> /tmp/metallb-config.yaml; \
	echo "metadata:" >> /tmp/metallb-config.yaml; \
	echo "  name: empty" >> /tmp/metallb-config.yaml; \
	echo "  namespace: metallb-system" >> /tmp/metallb-config.yaml
	kubectl --context kind-wg-ai-gateway apply -f /tmp/metallb-config.yaml
	rm /tmp/metallb-config.yaml

.PHONY: gateway-api-install
gateway-api-install: ## Install Gateway API CRDs
	@echo "Installing Gateway API CRDs..."
	kubectl --context kind-wg-ai-gateway apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml

##@ Deployment

.PHONY: docker-build-local
docker-build-local: ## Build and push docker image to local registry.
	docker build -t localhost:5000/wg-ai-egress-gateway:latest -f Dockerfile $(PROTOTYPE_ROOT)
	docker push localhost:5000/wg-ai-egress-gateway:latest

.PHONY: deploy-local
deploy-local: install ## Deploy controller to local Kind cluster.
	@echo "Deploying egress-gateway controller to Kind cluster..."
	kubectl --context kind-wg-ai-gateway apply -f config/

.PHONY: install
install: ## Install CRDs into the K8s cluster.
	kubectl --context kind-wg-ai-gateway apply -f $(BACKEND_CRD_DIR)/

.PHONY: example
example: ## Apply example EgressGateway, Backend, and HTTPRoute.
	kubectl --context kind-wg-ai-gateway apply -f config/samples/egress.yaml

.PHONY: example-tls
example-tls: ## Apply TLS example EgressGateway, Backend, and HTTPRoute.
	kubectl --context kind-wg-ai-gateway apply -f config/samples/egress-tls.yaml

.PHONY: logs
logs: ## Show controller logs.
	kubectl --context kind-wg-ai-gateway logs -l app.kubernetes.io/name=ai-egress-gateway-controller -c manager --tail=100 -f
