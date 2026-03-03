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
	kubectl --context kind-wg-ai-gateway apply -f config/samples/

.PHONY: logs
logs: ## Show controller logs.
	kubectl --context kind-wg-ai-gateway logs -l app.kubernetes.io/name=ai-egress-gateway-controller -c manager --tail=100 -f
