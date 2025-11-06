# Egress Gateways

* Authors: @shaneutt @usize
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

* As a gateway admin providing egress routing to external services, I need
  to be able to verify the identity of that external source and enforce
  authentication to secure that connection.

* As a gateway admin providing egress routing to external services, I need
  to be able to verify the client connection to the external service.

* As a gateway admin providing egress routing to external services, I need
  to be able to manage certificate authorities for egress connections, such
  that I can pin certificates or provide custom authorities (e.g.
  intermediate, self-signed, etc). I should also be able to integrate
  Certificate Revocation Lists (CRLs) to untrust revoked certificates.

* As a gateway admin providing egress routing to external service, DNS
  resolution for these sources needs to be controlled and secured. I need to
  be able to fine-tune control the DNS resolution of remote FQDNs including
  the ability to enable reverse DNS mapping checks.

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

## Overview

This proposal aims to provide egress gateway capabilities by defining:

1. Resource model using Gateway + HTTPRoute with a Backend for destinations (Service or FQDN).
2. Two routing modes: Endpoint (direct) and Parent (gateway chaining).
3. Policy scoping: Gateway (global posture), Route (filters, per-request), Backend (per-destination).
4. Extension points for AI use cases (payload processing, guardrails), without assuming an AI-only design.


## Open Design Questions

### Gateway Resource: Reuse vs. New Type

Two approaches are under consideration:

**Option A: Reuse Gateway API Gateway**
- Leverage existing `Gateway`, `HTTPRoute`, and `GRPCRoute` resources
- HTTPRoute references to external backends make it an egress gateway
- Requires Backend resource to represent external destinations

**Option B: New EgressGateway Resource**
- Introduce dedicated `EgressGateway` resource type
- Enables egress-specific fields (e.g., global CIDR allow-lists) without policy attachment overhead
- Clearer separation of ingress vs egress concerns

Option B implies defining equivalents of parentRefs, listeners, and route attachment; this is a significant fork from Gateway API and should be justified by clear need for an egress specific spec.

**Recommendation needed**: Feedback requested on whether the semantics justify a new resource or if Gateway reuse is sufficient.

### Backend Resource and Policy Application

To avoid heavy reliance on policy attachment, embed **typed** policy into `Backend` with clear phases and failure semantics, or use separate CRDs with `targetRef: Backend`.

```yaml
apiVersion: agentic.networking.x-k8s.io/v1alpha1
kind: Backend
metadata:
  name: openai-backend
spec:
  destination:
    type: FQDN
    fqdn:
      hostname: api.openai.com
      port: 443
  tls:
    mode: SIMPLE            # SIMPLE | MUTUAL
    serverName: api.openai.com
    caBundleRef:
      name: vendor-ca
    # clientCertificateRef:  # if MUTUAL
    #   name: egress-client-cert
  extensions:
  - name: inject-credentials
    type: CredentialInjector
    phase: request-headers   # request-headers|request-body|connect|backend-request|backend-response|response-body|response-headers
    priority: 10
    failOpen: false
    config:
      secretRef:
        name: openai-api-key
        namespace: platform-secrets
  - name: rate-qos
    type: QoSController
    phase: backend-request
    priority: 30
    failOpen: true
    config:
      requestsPerMinute: 1000
```
Alternatively, policies MAY be separate CRDs (e.g., `BackendTLSPolicy`, `EgressPolicy`) with `spec.targetRef: Backend`, avoiding schema growth on `Backend`.

## Routing Modes

### Endpoint Mode
Client traffic flows through the egress gateway directly to an external endpoint (FQDN or IP). The gateway applies policies and routing logic before forwarding to the destination.

### Parent Mode
Client traffic flows through a local egress gateway to an upstream gateway before reaching the final endpoint. This enables gateway chaining for multi-cluster or multi-zone topologies. The local egress gateway treats the parent as a single upstream. Local retries are limited to establishing the parent connection. Request-level retries are performed by the parent. Implementations MUST prevent retry loops across gateways.

## Policy Application Scopes

Policies must be applicable at three levels:

1. **Gateway-level**: Global rules affecting all traffic (e.g., cluster-wide CIDR restrictions, denied model lists)
2. **Route-level**: Per-request logic via filters `HTTPRoute.rules[].filters[ExtensionRef]` (e.g., payload transforms, compliance checks)
3. **Backend-level**: credentials, TLS, DNS, rate/QoS via `Backend.extensions` or backend-targeted policies.

### Conflict Resolution
When multiple policies influence the same request:
- **Specificity precedence**: Route > Backend > Gateway.
- **Same-level ties**: Implementations MUST use a deterministic tie-break (e.g., lexical name order) and surface status indicating the conflict.

## AI Workload Considerations

For inference and agentic workloads, the solution must support:

- **[Payload Processing](../7-payload-processing.md)**: Request/response transformations (PII redaction, prompt injection detection, content filtering)
  - note: Evaluation of payload processors occurs in the data plane; controllers reconcile objects into proxy configuration.
- **Protocol Support**: HTTP/gRPC for inference APIs, with future consideration for MCP and A2A protocols
- **Multi-destination Routing**: Failover between cloud providers and cross-cluster endpoints

## Observability Considerations

- Implementations SHOULD expose metrics tagged by `{gateway, route, backend, namespace, serviceAccount}` and surface conditions (e.g., `Accepted`, `Programmed`, `Degraded`).
- Denials and transform failures SHOULD emit Events.

## Next Steps

1. Decide on Gateway resource approach (reuse vs. new EgressGateway type)
2. Define Backend resource schema with embedded policy rules
3. Specify filter extension points for payload processing
4. Align with multi-cluster and agentic networking proposals

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
