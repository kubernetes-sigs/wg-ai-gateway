# Agents.md

## Purpose

This repository contains the proposals and prototypes of the Kubernetes AI Gateway Working Group. You can find the charter [here](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md).

## Project Structure

- `proposals/`: This directory contains the proposals for the AI Gateway Working Group. These will heavily reference concepts from the [Kubernetes Gateway API subproject](https://github.com/kubernetes-sigs/gateway-api), so start with that project if you encounter unfamiliar terms or ideas (especially the `/geps` directory in that repo).
- `prototypes/`: All prototype code lives here under a single Go module (`prototypes/go.mod`). Each prototype has its own subdirectory (e.g. `prototypes/backend-control-plane/`). Cross-cutting code shared across prototypes lives in `prototypes/internal/` (e.g. `prototypes/internal/backend/` for Backend API types and generated clients).
  - `prototypes/internal/backend/api/`: Backend CRD type definitions (XBackendDestination).
  - `prototypes/internal/backend/k8s/`: Generated Kubernetes client code (clientsets, informers, listers) and CRD YAML. This is generated code, so do not edit it directly.
  - `prototypes/backend-control-plane/`: A Gateway API controller prototype. See its [README](prototypes/backend-control-plane/README.md) for details.

## General Principles

- **Simplicity**: This is a prototype meant to prove/disprove the validity of the proposals. It does not need to be production-ready or have all the bells and whistles of a full implementation.
- **Modularity**: The code should be modular and extensible, allowing for easy addition of new features or even data planes (e.g. the [agentgateway](https://github.com/agentgateway/agentgateway/) proxy).
