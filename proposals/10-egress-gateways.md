# Egress Gateways

* Authors: @shaneutt @usize @keithmattix
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

### Reverse-Proxy Egress Model

This proposal focuses on a reverse-proxy egress model, where destinations are explicitly configured. It aims to define:

1. Resource model using Gateway + HTTPRoute with an additional resource (e.g. `Backend`) for destinations (Service or FQDN).
    - Other resource models (e.g., Mesh-attached egress via sidecars) are possible and are explicitly left open for future exploration.
2. Two routing modes: Endpoint (direct) and Parent (gateway chaining).
3. Policy scoping: Gateway (global posture), Route (filters, per-request), Backend (per-destination).
4. Extension points for AI use cases (payload processing, guardrails), without assuming an AI-only design.

#### Backend Placement

This proposal uses the following resource relationship:
```
Gateway <-[parentRef]- HTTPRoute -[backendRef]-> Backend
```
A `Backend` represents an external destination. Today, egress is typically achieved via a synthetic `Service`; this proposal instead uses `Backend` to represent that destination directly.

A `Service` does not need to know about a `Gateway`, and likewise this proposal does not attach `Backend` to a particular `Gateway`. A single `Backend` may be referenced by multiple `HTTPRoute` objects and consumed by multiple Gateways. In contrast, `HTTPRoute` does attach to specific Gateways via `parentRef`, since it represents configuration that is scoped to a particular Gateway.

### Forward-Proxy Egress Model (Future Work)

Another egress pattern is a dynamic forward-proxy model, where the egress gateway accepts requests to arbitrary external hostnames rather than routing only to a fixed set of Backends.

This document does not define a forward-proxy API. We may explore a complementary forward-proxy design in a follow-up proposal or subsection.

### Network Egress Model (Needs Discussion)

Some precedent exists throughout the community for egress which is supported at a network level.

Prime examples include Linkerd (which supports BOTH `Gateway`, and network level (effectively `Mesh` level),
and OVN-Kubernetes (`EgressRoute`).

TODO: We need to analyze this use case and decide if it needs coverage from this proposal as well,
and if not, document it as an alternative considered.
## Open Design Questions

### Gateway Resource

**Preferred Approach: Reuse Gateway API Gateway**
- Leverage existing `Gateway`, `HTTPRoute`, and `GRPCRoute` resources
- HTTPRoute references to external backends make it an egress gateway
- Requires Backend resource to represent external destinations

**Alternative Considered: New EgressGateway Resource**
- Introduce dedicated `EgressGateway` resource type
- Enables egress-specific fields (e.g., global CIDR allow-lists) without policy attachment overhead
- Clearer separation of ingress vs egress concerns

**Cons**
- Implies defining equivalents of parentRefs, listeners, and route attachment.

**Alternative Considered: Egress Definition Resource**
- Use a new resource to define which requests should be considered egress traffic, in order to express egress policies independently of the egress gateway. This follows the model used by some service meshes (for example, [Linkerd’s EgressNetwork resource](https://linkerd.io/2-edge/reference/egress-network/#egressnetwork-semantics)).
- Allows egress to be expressed at the data-plane level without the need for a Gateway instance.


This proposal focuses on the `Gateway`, `Route` and `Backend` model for egress, but MUST NOT preclude Mesh-based egress models in future work.

### Backend Resource and Policy Application

`Backend`s provide a first-class way to describe external destinations (for example, FQDNs) and the connection policy needed to reach them.

Implementations MAY also allow Backends to reference additional egress-related extensions for concerns such as credential injection or QoS,
but the detailed semantics of those extensions are out of scope for this proposal.


```yaml
apiVersion: gateway.networking.k8s.io/v1alpha1
kind: Backend
metadata:
  name: openai-backend
spec:
  destination:
    type: FQDN
    fqdn:
      hostname: api.openai.com
    ports:
    - number: 443
      protocol: TLS
      tls:
        mode: None | Simple | Mutual
        sni: api.openai.com
        caBundleRef:
          name: vendor-ca
  extensions:
  - name: inject-credentials
    type: gateway.networking.k8s.io/CredentialInjector:v1
    phase: request-headers
    priority: 10
    failOpen: false
    config:
      secretRef:
        name: openai-api-key
        namespace: platform-secrets
```

#### Producer vs. Consumer Role

In the context of egress routing, the `Backend` resource always represents the **consumer** side of the connection. Regardless of whether or not the `Backend` is defined in the same namespace as the route sending traffic to it (or the Gateway hosting that route), the `Backend` describes how the client (i.e. the egress gateway) should connect to a service outside of the cluster. Therefore, it's a consumer resource.

There is, however, an open question about what the appropriate role for `Backend`s of type `KubernetesService`. If a `Backend` points to a Service within the same cluster, is it still a consumer resource (describing how the gateway should connect to that Service), or is it a producer resource (describing how the Service expects clients to connect to it)? This question is important because it influences how we think about policy application and ownership semantics for `Backend`s. I propose that we treat all `Backend`s as consumer resources for the sake of consistency and simplicity. This means that even if a `Backend` points to a `Service` within its same namespace, it still describes how the gateway should connect to that Service from a client perspective. This approach has the added benefit of allowing different namespaces to define their own `Backend` resources for the same Service, each with potentially different connection policies (e.g. TLS settings, extensions) without ReferenceGrant. This aligns with the idea that different consumers may have different requirements for how they connect to the same service.

One consequence of this approach is that `Backend` is not a suitable target for any producer-side policies (e.g. authorization or authentication). This is difficult to enforce at the API level ([GEP-713](https://gateway-api.sigs.k8s.io/geps/gep-713/) for policy attachment does not specify categories distinguishing producer vs consumer), so implementations MUST consider `Backend` only as a consumer when attaching policies to it. Note that this does not preclude the possibility of defining egress authorization policies (e.g. consumer X can only talk to servics A and B) if (and only if!) the implementation can guarantee enforcement. Most implementations should likely be setting producer-side authorization/authentication policy at the egress gateway instead.


#### TLS Policy

The example above inlines a basic TLS configuration directly on the `Backend` resource. This is intentional.
Gateway API’s existing `BackendTLSPolicy` is designed around Service-based backends only and may end up being too restrictive for our needs. More specifically, using it for egress today would require representing each external FQDN as a synthetic Service, which this proposal aims to avoid. Furthermore, one could argue that inlined TLS policy provides simpler UX, especially in egress use-cases. `BackendTLSPolicy` also doesn't allow setting client certificates per backend; only once at `Gateway.spec.tls.backend`. This is overly restrictive for external FQDNs; no one "owns" a particular FQDN and can speak authoritatively about how every service in the cluster should talk to it. Generally speaking, the TLS field within `Backend` is meant to describe the TLS configuration that a client should use when talking to a backend. As the `Backend` resource shape stabilizes, we SHOULD evaluate whether `BackendTLSPolicy` can be reused, extended, or aligned for external FQDN egress use cases.

##### A note on the evolution of TLS within Gateway API

Today, the TLS story in Gateway API is [fractured](https://gateway-api.sigs.k8s.io/guides/tls/):
- `Gateway.spec.listeners[].tls` defines general TLS settings for incoming connections (Standard)
  - Defines how to handle all TLS traffic to that listener(e.g. terminate it, pass it through)
- `Gateway.spec.tls` defines mTLS settings for the entire Gateway (Experimental)
  - the `backend` field describes how the Gateway should connect to backends
    - Specifically, it allows setting client certificates for mutual TLS
  - the `frontend` field allows defining the validation context that should be used for incoming connections
    - Used to validate the client certificate in mTLS scenarios
- `TLSRoute` allows users to specify how TLS should be handled on a per-route basis (Experimental)
  - Most useful for SNI-based routing in passthrough scenarios
- `BackendTLSPolicy` allows users to define TLS settings when connecting to backends (Standard)
  - Specifically, validation context for the server certificates presented by upstream backends.
  - Allows setting SNI for backend connections.
  - Allows defining SANs.

Note: more discussion on many of these fields can be found in [GEP 2907](https://gateway-api.sigs.k8s.io/geps/gep-2907/)

The proposed `Backend` resource introduces yet another place to define TLS settings, and there is certainly a cost to further fragmenting the TLS story. At minimum, I believe an additional configuration point is needed for the egress gateway story simply because there is no standard egress story in Kubernetes. `Service` type `ExternalName` is the closest analogue, however, many organizations shy away from it completely due to cross-namespace security risks. Furthermore, naive usage of `ExternalName` can easily break SNI and TLS because the HTTP Host/:authority header will point to the cluster-internal FQDN rather than the external hostname. Clients would have to manually override the Host header and SNI or (or try to use `BackendTLSPolicy` to set SNI but I think you still have the Host header problem). The `Backend` resource allows us to define a clear and unambiguous way to represent external FQDNs and how to connect to them securely.

Things get murkier for `Backend`s of type `KubernetesService`. Despite the reasons for inline policy mentioned above, there is a strong argument for reusing `BackendTLSPolicy` here to avoid duplication and user confusion. Perhaps there should be a separate resource for `ExternalFQDN` backends that allows inline TLS, while `KubernetesService` backends reuse `BackendTLSPolicy`? Or maybe there should be another field within backend to mark a destination as being external (regardless of whether it's an FQDN or an IP)? We should certainly revisit those alternatives if we hit a wall in pursuing the current direction. In the interest of exploring this proposal fully and the cohesiveness of TLS within Gateway APi as a whole, I propoes the following guidelines for TLS policy:

1. `Gateway.spec.listeners[].tls` remains the source of truth for incoming TLS connections to the Gateway.
2. `Gateway.spec.tls.backend` is removed in favor of the `Backend` resource's TLS field
    1. `Gateway.spec.tls.frontend` remains for gateway-wide mTLS validation of incoming connections.
3. `Backend` is explicitly disallowed as a targetRef for `BackendTLSPolicy`
4. We pursue aligning `BackendTLSPolicy` and `Backend.spec.tls` as closely as possible w.r.t types.
    1. There may be different semantics or defaults for different types of backends (FQDN vs Service) or resources (BackendTLSPolicy), but the shape should be as similar as possible to avoid user confusion.
    2. In the short term, if you don't need mTLS, users should prefer `BackendTLSPolicy` for Kubernetes services.
    3. We can revisit this recommendation as `Backend` and other decompositions of `Service` evolve.
5. `TLSRoute` retains its current role as the way to express per-route TLS handling (e.g. SNI routing in passthrough mode).

#### Backend Extensions

Backends MAY reference extension processors via `spec.extensions[]`. This proposal does **not** define processor semantics, catalogs, schema validation, or execution ordering. The details of those topics are covered in the separate  **[Payload Processing proposal](../7-payload-processing.md)** and will be refined independently of the egress routing model. However, it is worth discussing the semantics of these egress extensions at a high level.

Fundamentally, there are 3 options for where extension processors can be applied in the egress flow:

1. **Route-level**: Extensions are applied as filters on the `HTTPRoute`, affecting individual requests as they are routed to a `Backend`.
2. **Backend-level**: Extensions are applied at the `Backend`, affecting all requests sent to that destination. Note that because multiple `Backend`s can point to the same destination (e.g. FQDN or `Service`), users may encounter unexpected behavior if different `Backend`s define conflicting extensions for the same destination (on different routes for example).
3. **Policy attachment**: Extensions are applied as policies attached to the `Backend` resource.

The key difference between both options 1 and 2 vs. option 3 really boils down to an even higher-level question of filters vs. policies in Gateway API. We won't attempt to define a broad rule in this proposal, but we will align with the outcome of that discussion.

Digging more deeply into option 1 vs. option 2, there are, of course, tradeoffs to consider. Route-level extensions provide more granular control, allowing different routes to apply different processors to the same `Backend`. This is useful when different consumers have different requirements for how they interact with the same destination. However, it may lead to duplication (if multiple routes need to apply the same processor to the same `Backend`) or accidental misconfigurations (if users forget to apply a necessary processor on a route). Backend-level extensions provide a centralized way to manage processors for a given `Backend` resource. This is useful when there are common requirements for how all consumers should interact with a destination. However, it may limit flexibility if different routes need different processors for the same `Backend`. To navigate these tradeoffs, I believe we must answer one fundamental question: how would we distinguish between extensions available on the `HTTPRoute` vs those available on the `Backend`? Would they be the same set of extensions, or would some extensions only make sense at one level or the other? For example, would a payload transformer (e.g. PII redaction) make sense at the `Backend` level, or is that inherently a per-request concern that belongs on the `HTTPRoute`? Conversely, would a rate limiter make sense at the `HTTPRoute` level, or is that inherently a per-destination concern that belongs on the `Backend`?

If we deicde that they are the same set of extensions (just applied with different granularity), then we must decide whether both levels are necessary. If, on the other hand, we want to separate some extensions to be `Backend`-only, then we must define clear distinctions between not just route vs. backend extensions, but also whether or not there is a need for a third type: frontend extensions (especially in the mesh case where there may not be a `Gateway` or explicit route).

__(Note: All examples of extensions in this document are illustrative only)__

#### Scope and Persona Ownership

While the namespaced ownership semantics of Kubernetes `Service`s are well-defined, the story for our proposed `Backend` resource is less clear, specifically for FQDN destinations. The fundamental question at issue is: who "owns" the destination, and what is the appropriate scope for defining it? There are two basic options:

- **Namespaced Backends**: Each namespace defines its own `Backend` resources for the external destinations it needs to reach. This model aligns with existing Kubernetes patterns, where resources are scoped to the namespace of the consuming workload. While this model allows __service owners__ to manage their own backends independently, it may lead to duplication if multiple namespaces need to reach the same external service. Furthermore, it may complicate cross-namespace policy enforcement if, for example, the egress gateway is in a central namespace (e.g. "egress-system") and multiple, disparate namespaces define conflicting `Backend` resources for the same FQDN. In this case, the gateway implementation would have to apply different policy depending on the source namespace of the request which could get combinatorially expensive. It also removes any ability for the cluster admin to centrally manage and audit egress destinations or apply a default set of policies for all egress traffic to said destination.

- **Cluster-scoped Backends**: `Backend` resources are defined at the cluster scope, allowing a single definition per external destination. This model aligns with the idea that __platform operators__ or __cluster admins__ are responsible for managing egress destinations and their associated policies. It simplifies policy enforcement at the gateway level, as there is a single source of truth for each destination. However, it may limit the flexibility of service owners to define custom backends or policies for their specific needs.

Realistically, both models have merit and are widely used across many gateway/mesh implementations. Prior art from the Network Policy subproject (i.e. `AdminNetworkPolicy` vs `NetworkPolicy`) suggests that both cluster-scoped and namespaced resources can coexist to serve different personas and use cases. We should consider whether:

1. Whether `Backend` should be namespaced or cluster-scoped.
2. Whether we should define both namespaced and cluster-scoped variants of `Backend` (e.g. `GlobalBackend` or `ClusterWideBakcend`)to serve different personas (service owners vs platform operators).

After multiple discussions, I am currently proposing making `Backend` a namespaced resource. This aligns with the idea of `Backend` being a consumer resource, where each namespace can define its own backends independently. One of the realizations that led me to this conclusion is that many of the admin use-cases for having a cluster-scoped `Backend` for external FQDNs can be reframed as making the cluster-admin the "service owner" of a (hypothetical) cluster-wide `Frontend` concept/resource. In other words, the cluser-admin would potentially be able to define a `Frontend` that represents a specific external service (e.g. *.openai.com) and configure TLS and other policies (authentication and authorization) that ensure clients are connecting to that `Frontend` in the way they expect (e.g. using certain certificates, with a proper SNI being set, etc). This `Frontend` could even have an optional `parentRef` to an egress gateway that, when combined with a `NetworkPolicy` enforces that all traffic to that or any `Frontend` must go through the egress gateway. While this entire flow is hypothetical and not in scope for this proposal, it does illustrate that cluster-scoped `Backend`s may not be strictly necessary to achieve the admin use-cases we are trying to solve for.

#### Schema Definition

```go
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// Backend is the Schema for the backends API.
type Backend struct {
  metav1.TypeMeta `json:",inline"`
  // metadata is a standard object metadata.
  // +optional
  metav1.ObjectMeta `json:"metadata,omitempty"`
  // spec defines the desired state of Backend.
  // +required
  Spec BackendSpec `json:"spec"`
  // status defines the observed state of Backend.
  // +optional
  Status BackendStatus `json:"status,omitempty"`
}

// BackendSpec defines the desired state of Backend.
type BackendSpec struct {
  // destination defines the backend destination to route traffic to.
  // +required
  Destination BackendDestination `json:"destination"`
  // extensions defines optional extension processors that can be applied to this backend.
  // +optional
  Extensions []BackendExtension `json:"extensions,omitempty"`
}

// TODO: Do we need the destination field or can we inline this all
// in spec?
// +kubebuilder:validation:ExactlyOneOf=fqdn;service;ip
type BackendDestination struct {
  // +required
  Type BackendType `json:"type"`
  // +optional
  Ports []BackendPort `json:"ports,omitempty"`
  // +optional
  FQDN *FQDNBackend `json:"fqdn,omitempty"`
  // Service *ServiceBackend `json:"service,omitempty"`
  // IP *IPBackend `json:"ip,omitempty"`
}

// BackendType defines the type of the Backend destination.
// +kubebuilder:validation:Enum=Fqdn;Ip;KubernetesService
type BackendType string

const (
  // Fqdn represents a fully qualified domain name.
  BackendTypeFqdn BackendType = "Fqdn"
  // Ip represents an IP address.
  BackendTypeIp BackendType = "Ip"
  // KubernetesService represents a Kubernetes Service.
  BackendTypeKubernetesService BackendType = "KubernetesService"
)

type BackendPort struct {
  // Number defines the port number of the backend.
  // +required
  // +kubebuilder:validation:Minimum=1
  // +kubebuilder:validation:Maximum=65535
  Number uint32 `json:"number"`
  // Protocol defines the protocol of the backend.
  // +required
  // +kubebuilder:validation:MaxLength=256
  Protocol BackendProtocol `json:"protocol"`
  // TLS defines the TLS configuration that a client should use when talking to the backend.
  // TODO: To prevent duplication on the part of the user, maybe this should be declared once at the
  // top level with per-port overrides?
  // +optional
  TLS *BackendTLS `json:"tls,omitempty"`
  // +optional
  ProtocolOptions *BackendProtocolOptions `json:"protocolOptions,omitempty"`
}

// BackendProtocol defines the protocol for backend communication.
// +kubebuilder:validation:Enum=HTTP;HTTP2;TCP;MCP
type BackendProtocol string

const (
  BackendProtocolHTTP  BackendProtocol = "HTTP"
  BackendProtocolHTTP2 BackendProtocol = "HTTP2"
  BackendProtocolTCP   BackendProtocol = "TCP"
  BackendProtocolMCP   BackendProtocol = "MCP"
)

type BackendTLS struct {
  // Mode defines the TLS mode for the backend.
  // +required
  Mode BackendTLSMode `json:"mode"`
  // SNI defines the server name indication to present to the upstream backend.
  // +optional
  SNI string `json:"sni,omitempty"`
  // CaBundleRef defines the reference to the CA bundle for validating the backend's
  // certificate.
  // Defaults to system CAs if not specified.
  // +optional
  CaBundleRef []ObjectReference `json:"caBundleRef,omitempty"`

  InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

  // ClientCertificateRef defines the reference to the client certificate for mutual
  // TLS. Only used if mode is MUTUAL.
  // +optional
  ClientCertificateRef *SecretObjectReference `json:"clientCertificateRef,omitempty"`

  SubjectAltNames []string `json:"subjectAltNames,omitempty"`
}

// BackendTLSMode defines the TLS mode for backend connections.
// +kubebuilder:validation:Enum=Simple;Mutual;None
type BackendTLSMode string

const (
  // Do not modify or configure TLS. If your platform (or service mesh)
  // transparently handles TLS, use this mode.
  BackendTLSModeNone BackendTLSMode = "None"
  // Enable TLS with simple server certificate verification.
  BackendTLSModeSimple BackendTLSMode = "Simple"
  // Enable mutual TLS.
  BackendTLSModeMutual BackendTLSMode = "Mutual"
)

// +kubebuilder:validation:ExactlyOneOf=mcp
type BackendProtocolOptions struct {
  // +optional
  MCP *MCPProtocolOptions `json:"mcp,omitempty"`
}

type MCPProtocolOptions struct {
  // MCP protocol version. MUST be a valid MCP version string
  // per the project's strategy: https://modelcontextprotocol.io/specification/versioning
  // +optional
  // +kubebuilder:validation:MaxLength=256
  Version string `json:"version,omitempty"`
  // URL path for MCP traffic. Default is /mcp.
  // +optional
  // +kubebuilder:default:=/mcp
  Path string `json:"path,omitempty"`
}

// FQDNBackend describes a backend that exists outside of the cluster.
// Hostnames must not be cluster.local domains or otherwise refer to
// Kubernetes services within a cluster. Implementations must report
// violations of this requirement in status.
type FQDNBackend struct {
  // Hostname of the backend service. Examples: "api.example.com"
  // +required
  Hostname string `json:"hostname"`
}

type BackendExtension struct {
  // +required
  Name string `json:"name"`
  // +required
  Type string `json:"type"`
  // +optional
  // +kubebuilder:validation:Type=object
  // +kubebuilder:pruning:PreserveUnknownFields
  RawConfig *apiextensionsv1.JSON `json:"rawConfig,omitempty"`
}

// BackendStatus defines the observed state of Backend.
type BackendStatus struct {
  // Controllers is a list of controllers that are responsible for managing the Backend.
  //
  // +listType=map
  // +listMapKey=name
  // +kubebuilder:validation:MaxItems=8
  // +kubebuilder:validation:Required
  Controllers []BackendControllerStatus `json:"controllers"`
}

type BackendControllerStatus struct {
  // Name is a domain/path string that indicates the name of the controller that manages the
  // Backend. Name corresponds to the GatewayClass controllerName field when the
  // controller will manage parents of type "Gateway". Otherwise, the name is implementation-specific.
  //
  // Example: "example.net/import-controller".
  //
  // The format of this field is DOMAIN "/" PATH, where DOMAIN and PATH are valid Kubernetes
  // names (https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names).
  //
  // A controller MUST populate this field when writing status and ensure that entries to status
  // populated with their controller name are removed when they are no longer necessary.
  //
  // +required
  Name ControllerName `json:"name"`
  // For Kubernetes API conventions, see:
  // https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
  // conditions represent the current state of the Backend resource.
  // Each condition has a unique type and reflects the status of a specific aspect of the resource.
  //
  // Standard condition types include:
  // - "Available": the resource is fully functional
  // - "Progressing": the resource is being created or updated
  // - "Degraded": the resource failed to reach or maintain its desired state
  //
  // The status of each condition is one of True, False, or Unknown.
  // +listType=map
  // +listMapKey=type
  // +optional
  Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```


## Routing Modes

### Endpoint Mode
Client traffic flows through the egress gateway directly to an external endpoint (FQDN or IP). The gateway applies policies and routing logic before forwarding to the destination.

### Parent Mode
Client traffic flows through a local egress gateway to an upstream gateway before reaching the final endpoint. This enables gateway chaining for multi-cluster or multi-zone topologies. The local egress gateway treats the parent as a single upstream. Local retries are limited to establishing the parent connection. Request-level retries are performed by the parent.

Operators MUST use network policy or sidecar/egress proxy configuration to deny direct egress from workloads and force all outbound traffic to the Gateway.
Retry loops across gateways are prohibited; implementations MUST tag requests to prevent looped retries.

### Workload-to-Gateway Addressing

**Open Questions**

How do workloads in the cluster address and connect to the egress `Gateway`? Several approaches are under consideration and are **not** specified by this document:

- **Service-wrapped Gateway**: Wrap the `Gateway` in a `Service` to obtain a `.cluster.local` FQDN. This works today but requires a synthetic `Service`.
- **Direct Gateway addressing**: Allow `Gateway` resources to obtain `.cluster.local` addresses directly (would require decomposing some `Service` DNS capabilities).
- **Service as `HTTPRoute.parentRef`**: In mesh implementations, use a `Service` as a `parentRef` so traffic destined for that Service is transparently routed through a `Gateway`.
- **Simplified frontend resource**: A lighter-weight resource that maps `.cluster.local` FQDNs directly to `Backend` objects without requiring a full `Gateway` + `HTTPRoute` configuration.

Additionally, a `Backend` may be a suitable place to define failover priorities between endpoints, which `HTTPRoute` cannot currently express (it only supports weighted load balancing).

## Policy Application Scopes

Policies must be applicable at three levels:

1. **Gateway-level**: Global rules affecting all traffic (e.g., cluster-wide CIDR restrictions, denied model lists)
2. **Route-level**: Per-request logic via filters `HTTPRoute.rules[].filters[ExtensionRef]` (e.g., payload transforms, compliance checks)
3. **Backend-level**: credentials, TLS, DNS, rate/QoS via `Backend.extensions` or backend-targeted policies.

### Conflict Resolution
When multiple policies influence the same request:
- **Specificity precedence**: Route > Backend > Gateway.
- **Same-level ties**: Implementations MUST use a deterministic tie-break where the oldest resource (based on `metadata.creationTimestamp`) wins, and surface status indicating the conflict.

Implementations MUST apply this ordering to ensure consistent behavior.

## AI Workload Considerations

For inference and agentic workloads, the solution must support:

- **[Payload Processing](../7-payload-processing.md)**: Request/response transformations (PII redaction, prompt injection detection, content filtering)
  - note: Evaluation of payload processors occurs in the data plane; controllers reconcile objects into proxy configuration.
- **Protocol Support**: HTTP/gRPC for inference APIs, with future consideration for MCP and A2A protocols
- **Multi-destination Routing**: Failover between cloud providers and cross-cluster endpoints

## Observability Considerations

- Implementations MUST expose metrics tagged by `{gateway, route, backend, namespace, serviceAccount}` and surface conditions (e.g., `Accepted`, `Programmed`, `Degraded`).
- Denials and transform failures MUST emit Events.

## Service Mesh Considerations
- Often no Gateway resource centralizing the application of the policy or configuration
- Meshes may often trade off stronger namespace-oriented tenenacy in favor of ease-of-configuration
- `Backend` MAY be used as a routing target within a mesh (without a Gateway), but such usage is considered implementation-specific at this time and may be broken later.
    - If implemented this way, it is strongly encouraged that the `Backend`'s configuration only applies to clients in the same namespace as the `Backend` resource to avoid cross-namespace policy conflicts.
    - Mesh implementations should consider proposing a cluster-wide `Backend` resource OR a `backendSelector` capability on a future `Frontend` resource to ease cross-namespace, cluster-wide usage.

## What about BackendTrafficPolicy?

Inlining TLS is discussed at length above, but a similar conversation should be had for the fields that current live in `BackendTrafficPolicy`, such as connection timeouts, retries, and load balancing. This is less pressing since `BackendTrafficPolicy` is still experimental, but we should decide whether or not we need both `BackendTrafficPolicy` and `Backend` or if we can consolidate them.

## Next Steps

1. Define Backend resource schema.
2. Specify default Backend policies e.g. CredentialInjector and QoSController.
3. Specify filter extension points for payload processing.
4. Align with multi-cluster and agentic networking proposals.
5. Prototype this design in at least one implementation to validate `Backend` vs `Route` vs Mesh placement and refine the API surface based on real-world usage.

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


# Prior Art

Several implementations exist for egress traffic management in Kubernetes. Understanding
these approaches informs our design decisions and highlights gaps that this proposal
aims to address.

## Existing Implementations

| Implementation | Resource Model | Scope |
|----------------|----------------|-------|
| **Istio** | [ServiceEntry], [Gateway], [VirtualService], [DestinationRule] | [Namespace] |
| **Linkerd** | [EgressNetwork], TLSRoute | [Namespace or global] |
| **Cilium** | [CiliumEgressGatewayPolicy] | [Cluster] |
| **OVN-Kubernetes** | [EgressIP], [EgressService], [EgressFirewall] | [Namespace] |

[ServiceEntry]: https://istio.io/latest/docs/reference/config/networking/service-entry/
[Gateway]: https://istio.io/latest/docs/reference/config/networking/gateway/
[VirtualService]: https://istio.io/latest/docs/reference/config/networking/virtual-service/
[DestinationRule]: https://istio.io/latest/docs/reference/config/networking/destination-rule/
[Namespace]: https://istio.io/latest/docs/tasks/traffic-management/egress/egress-gateway/
[EgressNetwork]: https://linkerd.io/2-edge/reference/egress-network/
[Namespace or global]: https://linkerd.io/2-edge/reference/egress-network/#namespace-semantics
[CiliumEgressGatewayPolicy]: https://docs.cilium.io/en/stable/network/egress-gateway/egress-gateway/
[Cluster]: https://docs.cilium.io/en/stable/network/egress-gateway/egress-gateway/#ciliumegressgatewaypolicy
[EgressIP]: https://ovn-kubernetes.io/features/cluster-egress-controls/egress-ip/
[EgressService]: https://ovn-kubernetes.io/features/cluster-egress-controls/egress-service/
[EgressFirewall]: https://rcarrata.com/openshift/egress-firewall/

## Istio

Istio offers a centralized egress gateway in complement to direct egress from pods via a sidecar proxy.

### Centralized Egress Gateway

The dedicated Egress `Gateway` is framed as a mirror image of an ingress gateway, acting as a choke point albeit for all traffic exiting a mesh.

> An ingress gateway allows you to define entry points into the mesh that all incoming traffic flows through. Egress gateway is a symmetrical concept; it defines exit points from the mesh. Egress gateways allow you to apply Istio features, for example, monitoring and route rules, to traffic exiting the mesh.

[source](https://istio.io/latest/docs/tasks/traffic-management/egress/egress-gateway/)

### Direct Egress via Sidecar

Envoy sidecars send traffic directly to external services once those services are modeled in Istio’s registry via `ServiceEntry`

With respect to scoping, notably, `ServiceEntry` is namespace scoped, but visible by default to all namespaces.

> The ’exportTo’ field allows for control over the visibility of a service declaration to other namespaces in the mesh. By default, a service is exported to all namespaces.

[source](https://istio.io/latest/docs/reference/config/networking/service-entry/)

### Key Takeaway

Istio’s pattern is closest to a mesh attached gateway with an optional centralized egress gateway, where the centralized egress gateway serves as an optional chokepoint in cases where e.g., all traffic exiting a mesh (or node) must adhere to a given policy, or in clusters where application nodes have no public IPs. In their model external destinations are represented via `ServiceEntry` as opposed to something like a `Backend`.

## Linkerd

Linkerd's model is mesh-attached, with policy enforced at the sidecar proxy. There is no mandatory centralized gateway.

> Linkerd’s egress control is implemented in the sidecar proxy itself; separate egress gateways are not required (though they can be supported).

[source](https://linkerd.io/2-edge/features/egress/)

There is an important caveat regarding egress via service mesh.

> No service mesh can provide a strong security guarantee about egress traffic by itself; for example, a malicious actor could bypass the Linkerd sidecar - and thus Linkerd’s egress controls - entirely. Fully restricting egress traffic in the presence of arbitrary applications thus typically requires a more comprehensive approach.

[source](https://linkerd.io/2-edge/reference/egress-network/)

### EgressNetwork

The key primitive in Linkerd's approach is `EgressNetwork`. It may represent multiple external destinations.

> EgressNetwork can encompass a set of destinations. This set can vary in size - from a single IP address to the entire network space that is not within the boundaries of the cluster.

Fundamentally, this means that `EgressNetwork` exists to classify outbound traffic, not to represent a concrete upstream endpoint or its connection semantics. Policy is applied via Gateway API's `HTTPRoute` and `TLSRoute` which attach to the `EgressNetwork` as parent.

In this model, Gateway API resources act purely as a policy expression language, not as a description of deployed gateway infrastructure.

`EgressNetwork` maintains a distinction between namespace and global scoping via a designated global namespace.

> EgressNetworks are namespaced resources, which means that they affect only clients within the namespace that they reside in. The only exception is EgressNetworks created in the global egress namespace: these EgressNetworks affect clients in all namespaces.

[source](https://linkerd.io/2-edge/reference/egress-network/#egressnetwork-semantics)

### Key Takeaway

Linkerd’s approach attaches Gateway API routes to a first-class object that classifies external destinations, rather than representing concrete upstream endpoints or connection semantics. It does not require a centralized egress proxy, relying instead on sidecar enforcement, while explicitly acknowledging that stronger enforcement may require additional mechanisms.

## Network-Level Approaches

Cilium and OVN-Kubernetes offer egress routing policy at the L3/L4 level, focused on source identity and network-level filtering rather than application protocol semantics.

### Cilium

> The egress gateway feature routes all IPv4 and IPv6 connections originating from pods and destined to specific cluster-external CIDRs through particular nodes, from now on called "gateway nodes".

[source](https://docs.cilium.io/en/stable/network/egress-gateway/egress-gateway/#egress-gateway)

Policy is applied via the `CiliumEgressGatewayPolicy` resource. This resource is cluster scoped. It serves to select pods and matching destination CIDRs and then ensures that egress traffic matching those criteria is routed through specific nodes with specific source IPs.

### OVN-Kubernetes

`EgressIP` ensures that traffic from configured pods or namespaces present a consistent source IP to external services.
`EgressFirewall` supports namespace-scoped Allow/Deny policies for traffic from pods to IPs outside the cluster.
`EgressService` has a one-to-one mapping with a `LoadBalancer` Service:

> [EgressService] enables the egress traffic of pods backing a LoadBalancer service to use a different network than the main one and/or their source IP to be the Service's ingress IP.

[source](https://ovn-kubernetes.io/features/cluster-egress-controls/egress-service/)

### Key Takeaway

These approaches are complementary to our use case. This proposal focuses on application-layer (L7) policy enforcement and routing semantics, e.g., [payload processing](7-payload-processing.md), which operates above the network level.
