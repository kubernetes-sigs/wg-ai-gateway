# Agents.md

## Purpose

This repository contains the proposals and prototypes of the Kubernetes AI Gateway Working Group. You can find the charter [here](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md).

## Project Structure

- `proposals/`: This directory contains the proposals for the AI Gateway Working Group. These will heavily reference concepts from the [Kubernetes Gateway API subproject](https://github.com/kubernetes-sigs/gateway-api), so start with that project if you encounter unfamiliar terms or ideas (especially the `/geps` directory in that repo).
- `hack/`: This directory contains scripts for code generation of Kubernetes CRDs and other tasks like linting.
- `cmd/`: Contains the entrypoint for our prototype binaries.
- `k8s/`: Contains the Kubernetes CRD definitions and client code for the AI Gateway resources. This is generated code, so do not edit it directly.
- `pkg/`: Contains the core logic for the AI Gateway prototype implementations. The code here is generally going to be a Kubernetes controller that watches for Gateway API resources as well as CRDs outlined in `proposals/` and defined in `k8s/`. The central controller logic will then translate that code into Kubernetes resources (for the underlying Gateway infrastructure) as well as configuration for the AI Gateway itself. This AI Gateway is typically going to be an Envoy proxy with some AI-specific filters and configuration, so refer to other implementations like Istio, Envoy Gateway, and kgateway for inspiration. You'll also want to refer to the [Envoy xDS documentation](https://www.envoyproxy.io/docs/envoy/latest/api-v3/api) as well as the [Go bindings](https://github.com/envoyproxy/go-control-plane) to understand how to generate configuration for Envoy.

## General Principles

- **Simplicity**: This is a prototype meant to prove/disprove the validity of the proposals. It does not need to be production-ready or have all the bells and whistles of a full implementation.
- **Modularity**: The code should be modular and extensible, allowing for easy addition of new features or even data planes (e.g. the [agentgateway](https://github.com/agentgateway/agentgateway/) proxy).
