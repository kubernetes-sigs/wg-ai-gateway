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

export GO111MODULE=on

# Work around Kind alpha using an older Docker client API version
export DOCKER_API_VERSION=1.44

PROTOTYPE_ROOT := $(dir $(CURDIR))
CRD_DIR := $(CURDIR)/install-crd
CONFIG_DIR := $(CURDIR)/config

# Agentgateway image to pull into the Kind cluster
AGENTGATEWAY_IMAGE ?= ghcr.io/agentgateway/agentgateway:latest

# Print the help menu
.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: vet fmt build ## Test, verify and build this project

##@ Code Quality

.PHONY: fmt
fmt: ## Run go fmt against code
	cd $(PROTOTYPE_ROOT) && gofmt -w ./payload-processor/controller/

.PHONY: vet
vet: ## Run go vet against code
	cd $(PROTOTYPE_ROOT) && go vet ./payload-processor/controller/...

.PHONY: test
test: vet ## Run tests
	cd $(PROTOTYPE_ROOT) && go test -race -cover ./payload-processor/controller/...

##@ Build

.PHONY: build
build: fmt vet ## Build controller binary
	cd $(PROTOTYPE_ROOT) && go build -o payload-processor/bin/controller ./payload-processor/controller/cmd/

.PHONY: docker-build-local
docker-build-local: ## Build and push docker image to local registry
	@echo "Building and pushing to local registry..."
	docker build -t localhost:5000/payload-processor-controller:latest -f controller/Dockerfile $(PROTOTYPE_ROOT)
	docker push localhost:5000/payload-processor-controller:latest

##@ Development Environment

.PHONY: dev-setup
dev-setup: kind-cluster registry-setup gateway-api-install payloadprocessor-crd-install metallb-install build docker-build-local deploy-local ## Complete development environment setup

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
		kind create cluster -v6 --name wg-ai-gateway --config=hack/kind-config.yaml; \
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

.PHONY: payloadprocessor-crd-install
payloadprocessor-crd-install: ## Install PayloadProcessor CRDs and RBAC
	@echo "Installing PayloadProcessor CRDs..."
	kubectl --context kind-wg-ai-gateway apply -f $(CRD_DIR)/

.PHONY: agentgateway-image-load
agentgateway-image-load: ## Pull and load agentgateway image into Kind cluster
	@echo "Loading agentgateway image into Kind cluster..."
	docker pull $(AGENTGATEWAY_IMAGE) || true
	kind load docker-image $(AGENTGATEWAY_IMAGE) --name wg-ai-gateway

.PHONY: deploy-local
deploy-local: payloadprocessor-crd-install ## Deploy controller to local Kind cluster
	@echo "Deploying payload-processor controller to Kind cluster..."
	kubectl --context kind-wg-ai-gateway apply -f $(CONFIG_DIR)/controller.yaml

.PHONY: undeploy-local
undeploy-local: ## Remove controller from local Kind cluster
	@echo "Removing payload-processor controller from Kind cluster..."
	kubectl --context kind-wg-ai-gateway delete -f $(CONFIG_DIR)/controller.yaml --ignore-not-found

.PHONY: redeploy
redeploy: docker-build-local ## Rebuild and redeploy controller
	kubectl --context kind-wg-ai-gateway rollout restart deployment/payload-processor-controller -n ai-gateway-system

.PHONY: logs
logs: ## Show controller logs
	kubectl --context kind-wg-ai-gateway logs -l app.kubernetes.io/name=payload-processor-controller -c controller --tail=100 -f -n ai-gateway-system

.PHONY: example
example: ## Apply example PayloadProcessor and backends
	kubectl --context kind-wg-ai-gateway apply -f testdata/simulator-backends.yaml
	kubectl --context kind-wg-ai-gateway apply -f testdata/payload-processor-bbr.yaml

##@ Misc

.PHONY: run
run: build ## Run the controller locally
	./bin/controller --xds-port=9978

.PHONY: deps
deps: ## Install dependencies
	cd $(PROTOTYPE_ROOT) && go mod download && go mod tidy

##@ Code Generation

API_DIR := $(CURDIR)/api
CLIENT_OUTPUT_DIR := $(API_DIR)/k8s/client
CLIENT_OUTPUT_PKG := $(shell cd $(PROTOTYPE_ROOT) && go list -m)/payload-processor/api/k8s/client
CODEGEN_PKG = $(shell cd $(PROTOTYPE_ROOT) && go env GOPATH)/pkg/mod/k8s.io/code-generator@$(shell cd $(PROTOTYPE_ROOT) && go list -m -f '{{.Version}}' k8s.io/code-generator)
CODEGEN_SCRIPT := $(CODEGEN_PKG)/kube_codegen.sh
BOILERPLATE_FILE := hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate
generate: clientsets ## Generate clientsets, listers, and informers

.PHONY: clientsets
clientsets: ## Generate typed clientsets, listers, and informers for PayloadProcessor CRD
	@echo "--- Ensuring code-generator is in module cache..."
	@cd $(PROTOTYPE_ROOT) && go mod download k8s.io/code-generator
	@echo "+++ Generating client code..."
	@bash -c 'source $(CODEGEN_SCRIPT); \
		kube::codegen::gen_client \
		    --with-watch \
		    --output-dir $(CLIENT_OUTPUT_DIR) \
		    --output-pkg $(CLIENT_OUTPUT_PKG) \
		    --boilerplate $(BOILERPLATE_FILE) \
		    $(CURDIR)/'
