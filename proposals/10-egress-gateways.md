# Egress Gateways

* Authors: @shaneutt
* Status: Proposed

# What?

Provide standards in Kubernetes to route traffic outside of the cluster.

# Why?

Applications are increasingly utilizing inference as a part of their logic.
This may be for chatbots, knowledgebases, or a variety of other potential use
cases. The inference workloads to support this may not always be on the same
cluster as the requesting workload. Inference workloads may be on a separate
cluster, as the organization centralizes them, or they may be located
specifically for reasons of regionality. Even then, not all organizations are
going to run inference workloads themselves, and will utilize 3rd party cloud
services. All of this points to a need to provide standards for how Kubernetes
workloads reach these external inference sources, and provide the same AI
Gateway security, control and management capabilities that are required for the
ingress use case.

## User Stories

* As a gateway admin I need to provide workloads within my cluster access to
  services outside of my cluster, in particular cloud and otherwise hosted
  services.

* As a gateway admin I need to manage access tokens for 3rd party AI services
  so that workloads on the cluster can perform inference within needing to
  manage these secrets themselves, and so that I can manage access from all
  workloads in a uniform manner.

* As a gateway admin providing access and token management for 3rd party AI
  cloud services to workloads, I need fail-over from one cloud provider to
  others when the primary cloud provider is overwhelmed or in a failure state.

* As a cluster admin I need to provide inference to workloads on my cluster,
  but I provide a dedicated cluster for this so that I can manage it
  separately.

* As a cluster admin I need to provide inference to workloads on my cluster,
  but I do not run AI workloads on Kubernetes. I use a cloud service to run
  models (e.g. Vertex, Bedrock) and need workloads to have managed access to
  that service to perform inference.

* As a developer of an application that requires inference as part of its
  function, I need my application to have access to external AI cloud services
  which offer specific, specialized features only offered by that provider.

* As a developer of an application that requires inference as part of its
  function, I need fail-over to 3rd party providers if local AI workloads are
  overwhelmed or in a failure state.

* As a platform operator I need to attribute outbound traffic per namespace or
  workload to enforce rate or API utilization limits.

* As a compliance engineer I need to guarantee that outbound traffic to
  third-party AI resources obeys regulatory restrictions such as region locks.

## Goals

* Define the standards for Gateways that route and manage traffic destined for
  external resources outside of the cluster.
* Define (or refine) the standards by which token management for Gateways can
  be employed to enable access to backends that require auth.
* Foundationally the standards for egress Gateways should be based on standards
  based networking first, layering up to inference and agentic use cases.

# How?

TODO: in later PRs.


> **This should be left blank until the "What?" and "Why?" are agreed upon,
> as defining "How?" the goals are accomplished is not important unless we can
> first even agree on what the problem is, and why we want to solve it.
>
> This section is fairly freeform, because (again) these proposals will
> eventually find there way into any number of different final proposal formats
> in other projects. However, the general guidance is to break things down into
> highly focused sections as much as possible to help make things easier to
> read and review. Long, unbroken walls of code and YAML in this document are
> not advisable as that may increase the time it takes to review.

# Additional Criteria

The following are things we need to resolve before we can consider this
proposal complete and ready to move out to other areas.

- [ ] We need to decide how the multi-cluster aspect of egress gateways
  interacts with the [GIE's multi-cluster proposal], if at all. This may end up
  with multiple different multi-cluster options for users, so we'll need to be
  clear about why there are multiple options, and what one solves over the
  other. SIG MC needs to be a part of this conversation.
- [ ] The Agentic Networking Subproject has a [proposal for external MCP/A2A]
  services, making them a stakeholder for egress gateways as well. We need to
  work with them to incorporate their user stories and requirements so that
  what we ultimately ship covers the combined use cases.

[GIE's multi-cluster proposal]:https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/docs/proposals/1374-multi-cluster-inference
[proposal for external MCP/A2A]:https://docs.google.com/document/d/17kA-78gq25BgS2ElHMCd-zy__9clVL-GZQcHCm52854/edit?tab=t.0

# Relevant Links

* [Istio's implementation of Egress Gateways](https://istio.io/latest/docs/tasks/traffic-management/egress/egress-gateway/)
