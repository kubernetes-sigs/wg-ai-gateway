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

# We need all the Make variables exported as env vars.
# Note that the ?= operator works regardless.

# Enable Go modules.
export GO111MODULE=on

PROTOTYPE_ROOT := $(dir $(CURDIR))
BACKEND_DIR := $(CURDIR)/backend
API_DIR := $(BACKEND_DIR)/api
CRD_DIR := $(BACKEND_DIR)/k8s/crds
CLIENT_OUTPUT_DIR := $(BACKEND_DIR)/k8s/client

# Print the help menu.
.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: vet fmt verify test build;$(info $(M)...Begin to test, verify and build this project.) @ ## Test, verify and build this project.

# Run go fmt against code
.PHONY: fmt
fmt: ;$(info $(M)...Begin to run go fmt against code.)  @ ## Run go fmt against code.
	cd $(PROTOTYPE_ROOT) && gofmt -w ./backend-control-plane/pkg

# Run go vet against code
.PHONY: vet
vet: ;$(info $(M)...Begin to run go vet against code.)  @ ## Run go vet against code.
	cd $(PROTOTYPE_ROOT) && go vet ./backend-control-plane/pkg/...

# Run go test against code
.PHONY: test
test: vet;$(info $(M)...Begin to run tests.)  @ ## Run tests.
	cd $(PROTOTYPE_ROOT) && go test -race -cover ./backend-control-plane/pkg/...


# Run static analysis.
.PHONY: verify
verify:
	hack/verify-all.sh -v

# Build manager binary
.PHONY: build
build: fmt vet ;$(info $(M)...Building manager binary.) @ ## Build manager binary.
	cd $(PROTOTYPE_ROOT) && go build -o backend-control-plane/bin/manager ./backend-control-plane/cmd/main.go

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

.PHONY: install
install: ## Install CRDs into the K8s cluster
	kubectl --context kind-wg-ai-gateway apply -f $(CRD_DIR)/

.PHONY: deploy-local
deploy-local: install ## Deploy controller to local Kind cluster
	@echo "Deploying controller to Kind cluster..."
	kubectl --context kind-wg-ai-gateway apply -f config/

.PHONY: docker-build-local
docker-build-local: ## Build and push docker image to local registry
	@echo "Building and pushing to local registry..."
	docker build -t localhost:5000/wg-ai-gateway:latest -f Dockerfile $(PROTOTYPE_ROOT)
	docker push localhost:5000/wg-ai-gateway:latest

.PHONY: logs
logs: ## Show controller logs
	kubectl --context kind-wg-ai-gateway logs -l app.kubernetes.io/name=ai-gateway-controller -c manager --tail=100 -f

.PHONY: example
example: ## Apply example Gateway and HTTPRoute
	kubectl --context kind-wg-ai-gateway apply -f config/samples/

# Run the controller locally
.PHONY: run
run: build ;$(info $(M)...Running controller locally.) @ ## Run the controller locally.
	./bin/manager

# Install dependencies
.PHONY: deps
deps: ;$(info $(M)...Installing dependencies.) @ ## Install dependencies.
	cd $(PROTOTYPE_ROOT) && go mod download && go mod tidy

REPO_ROOT:=${CURDIR}

## @ Code Generation Variables

# Find the code-generator package in the Go module cache.
# The 'go mod download' command in the targets ensures this will succeed.
CODEGEN_PKG = $(shell cd $(PROTOTYPE_ROOT) && go env GOPATH)/pkg/mod/k8s.io/code-generator@$(shell cd $(PROTOTYPE_ROOT) && go list -m -f '{{.Version}}' k8s.io/code-generator)
CODEGEN_SCRIPT := $(CODEGEN_PKG)/kube_codegen.sh

# The root Go package for your generated client code.
CLIENT_OUTPUT_PKG := $(shell cd $(PROTOTYPE_ROOT) && go list -m)/backend-control-plane/backend/k8s/client
BOILERPLATE_FILE := hack/boilerplate/boilerplate.generatego.txt


## @ Code Generation

.PHONY: generate
generate: manifests deepcopy register clientsets ## Generate manifests, deepcopy code, and clientsets.

.PHONY: manifests
manifests: controller-gen ## Generate CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="$(API_DIR)/..." output:crd:artifacts:config=$(CRD_DIR)

.PHONY: deepcopy
deepcopy: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="$(BOILERPLATE_FILE)" paths="$(API_DIR)/..."

.PHONY: clientsets
clientsets: ## Generate clientsets, listers, and informers.
	@echo "--- Ensuring code-generator is in module cache..."
	@cd $(PROTOTYPE_ROOT) && go mod download k8s.io/code-generator
	@echo "+++ Generating client code..."
	@bash -c 'source $(CODEGEN_SCRIPT); \
		kube::codegen::gen_client \
		    --with-watch \
		    --output-dir $(CLIENT_OUTPUT_DIR) \
		    --output-pkg $(CLIENT_OUTPUT_PKG) \
		    --boilerplate $(BOILERPLATE_FILE) \
		    $(BACKEND_DIR)/'

.PHONY: register
register: ## Generate register code for CRDs under api/v0alpha0
	@echo "--- Ensuring code-generator is in module cache..."
	@cd $(PROTOTYPE_ROOT) && go mod download k8s.io/code-generator
	@echo "+++ Generating register code for api/v0alpha0..."
	@bash -c 'source $(CODEGEN_SCRIPT); \
		kube::codegen::gen_register \
		    --boilerplate $(BOILERPLATE_FILE) \
		    $(API_DIR)/v0alpha0'

## @ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.19.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN) ## Installs controller-gen if not already installed.
	chmod +x ./hack/install-tool.sh
	./hack/install-tool.sh \
		"$(CONTROLLER_GEN)" \
		"sigs.k8s.io/controller-tools/cmd/controller-gen" \
		"$(CONTROLLER_TOOLS_VERSION)" \
		"$(LOCALBIN)"
