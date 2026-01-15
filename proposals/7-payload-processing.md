# Payload Processing

* Authors: @shaneutt, @kflynn, @shachartal 
* Status: Proposed

# What?

Define standards for declaratively adding processing steps to HTTP requests and
responses in Kubernetes across the entire payload, this means processing the
body of the request, and possibly the response, in addition to the headers.

# Why?

Modern workloads require the ability to process the full payload of an HTTP
request and response, including both headers and the body:

* **AI Inference Security**: Guard against bad prompts for inference requests,
  or misaligned responses.
* **AI Inference Optimization**: Route requests based on semantics. Enable
  caching based on semantic similarity to reduce inference costs and enable
  faster response times for common requests. Enable RAG systems to supplement
  inference requests with additional context to get better results.
* **Web Application Security**: Enforce signature-based detection rules, anomaly
  detection systems, scan uploads, call external auth with payload data, etc.

Payload processing can also encompass various use cases outside of AI, such as
external authorization or rate limiting. Despite these use cases, though,
payload processing is not standardized in Kubernetes today.

## Definitions

* **Payload Processors**: Features capable of processing the full payload of
  requests and/or responses (including headers and body).

> **Note**:  At a definition level, we do not intend "Payload Processors" to be
> construed with any existing details or any other implementations that might
> have similarities. For instance, we are not trying to prescribe that these
> are done natively, or as extensions. We are also aware that many existing API
> Gateways include "filter" mechanisms which could be seen as fitting this
> definition, but we are not limiting discussion to only these existing mechanisms.

## User Stories

* As a developer of an application that performs AI inference as part of its
  function:

   * I want the prompt of requests to be processed for semantics so that the
     backend target can be dynamically adapted based on semantics, for instance
     identifying whether a request is a "math request" and then targeting the
     most appropriate model.

   * I want declarative configuration of failure modes for processing steps
     (fail-open, fail-closed, fallback, etc) to ensure safe and efficient
     runtime behavior of my application.

   * I want predictable ordering of all payload processing steps to ensure
     safe and consistent runtime behavior.

* As a developer of Agentic AI platforms:

  * I need the ability to process the payload of ModelContextProtocol (MCP)
    requests to make routing and security decisions.

  * I want to set or modify request headers based on payload attributes so that
    the system can look up a session header from a store (by tool) and route the
    request to the correct backend MCP server.

* As a security engineer, I want to be able to add a detection engine which
  scans requests to identify malicious or anomalous request payloads and
  block, sanitize, and/or report them before they reach backends.

* As a cluster admin, I want to be able to add semantic caching to inference
  requests in order to detect repeated requests and return cached results,
  reducing overall inference costs and improving latency for common requests.

* As a compliance officer:

   * I want to be able to add processors that examine inference **requests**
     for personally identifiable information (PII) so that any PII can result
     in the request being blocked, sanitized, or reported before sending it to
     the inference backend.

   * I want to be able to add processors that examine inference **responses**
     for malicious or misaligned results so that any such results can be
     dropped, sanitized, or reported before the response is sent to the
     requester.

## Goals

* Ensure that declarative APIs, standards, and guidance on best practices
  exist for adding Payload Processors to HTTP requests and responses on
  Kubernetes.
* Ensure that there is adequate documentation for developers to be able to
  easily build implementations of Payload Processors according to the
  standards.
* Support composability, pluggability, and ordered processing of Payload
  Processors.
* Ensure the APIs can provide clear and easily observable defaulting behavior.
* Ensure the APIs can provide clear and obvious runtime behavior.
* Provide failure mode options for Payload Processors.

## Non-Goals

* Requiring every request or response to be processed by a payload processor.
  The mechanisms described in this proposal are intended to be optional
  extensions.

# How?

This proposal is meant to drive towards multiple outcomes and engagements with
the SIGs:

1. A proposal for "Payload Processors" in SIG Network, via [Gateway API].
2. Development of example "GuardRails" extensions using Payload Processing
   (e.g. prompt guards). SIG Security (and potentially some LLM security
   specialists) will be engaged for review to get any concerns or insights they
   can provide.
3. (possibly, to be discussed further) a proposal to re-implement the existing
   Body-Based Router (BBR) in the [Gateway API Inference Extension (GIE)] using
   the standardized mechanisms.
4. (possibly, to be discussed further) a connection with the existing Gateway
   API [Firewall GEP].

[Gateway API]:https://github.com/kubernetes-sigs/gateway-api
[Gateway API Inference Extension (GIE)]:https://github.com/kubernetes-sigs/gateway-api-inference-extension
[Firewall GEP]:https://github.com/kubernetes-sigs/gateway-api/issues/3614

## Gateway API - Payload Processors

The following is a proposal for [Gateway API] to enable "Payload Processing"
within its API surface (`Gateway`, `HTTPRoute`, etc). The overview includes
high level concepts and technical requirements as derived from our user
stories and the implementation entation details include examples to illustrate
how this would work (but are not intended to be new APIs that we actually
propose).

[Gateway API]:https://github.com/kubernetes-sigs/gateway-api

### Overview

> **WIP**: this will be updated iteratively across multiple PRs.

Requirements:

* **Declarative**: The administrator of a `Gateway` or `HTTPRoute` needs to be
  able to add
  payload processors declaratively, as part of the route definition.
* **Ordered**: Each specified payload processor needs to be able to be
  optionally executed by the
  Gateway API implementation in the order it was configured.
* **Abundancy**: A route needs to be able to configure a potentially very large
  number of payload processors.
* **Routing**: Some payload processors may trigger routing to a specific
  backend (e.g. semantic routing, semantic caching). This optional routing may
  only trigger conditionally.
* **Constraints**: Payload processors can have constraints and limitations
  applied (e.g. latency timeout)
* **Failure Modes**: Payload processors need to have configurable failure modes
  which determine what happens if the extension fails during processing.
* **Payload Influences Headers**: Payload processors need to be able to modify
  and set headers based on information in the payload (e.g. session
  information).
* **Rejection**: Payload processors need to be able to reject requests,
   _and_ responses (e.g. PII detected in response)

### Implementation Details



### Operational Requirements and Assumptions

* It is assumed that Payload Processors will generally operate on payloads that have already been decrypted before passing them to the processor. However we do not preclude the possibility that future implementations may want the payloads decrypted _by the processors_. For now we're not building for this use case, but we'll leave the door open.
* Gateway MUST support Processors running as workloads on the cluster, as well as remote endpoints. (We should probably focus on cluster-local Services for the first phase)
* Topology-Aware Routing (or similar constraints) are highly desirable when configuring Gateway consumption of Processors (both due to latency
concerns, and also since cross-node/cross-AZ traffic may have cloud-networking costs associated with it).
* Processor MAY support scale based on operational metrics from Gateway.
* Processor MAY present indications of its capacity (or lack thereof) to Gateway. Gateway MAY support reducing the traffic 
load on the Processor if such indications are presented. In some scenarios, the user MAY prefer degrading payload processing
over significant impact on data plane performance.

## Resource Model


### Payload Processor Custom Resource


`HTTPRoute` **rules** are the most specific level where we wish to prescribe Payload Processors.
The amount of details and settings we need to support at the processor level is greater than what we can comfortable inline within a sub-object
of `HTTPRoute` (See also how long `spec.rules.filters` can become on its own, today).

The approach below creates a new custom resource kind for Payload Processing Pipelines.
A minimal `HTTPRoute` that references a processor pipeline is shown below.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: example-httproute
spec:
  parentRefs:
  - name: example-gateway
  hostnames:
  - "www.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /query
    backendRefs:
    - name: example-svc
      port: 8080
    processing:
    - name: prompt-injection-guardrails
      pipelineRef:
        name: prompt-injection-protection
---
apiVersion: gateway.networking.k8s.io/v1alpha1
kind: PayloadProcessingPipeline
metadata:
  name: prompt-injection-protection
spec:
  # Sequence of processors
  processor:
  - name: malicious-prompt-prevention
    # Indicates if this processor rule can mutate passed data
    allowMutating: true
    # Determines which parts of the request/response pair is of interest to the processor
    phases: ["request-headers", "request-body", "response-headers", "response-body"]
    # Defines the callout type, in this case it is backed by a service hosted on the cluster
    endpointRef:
      kind: Service
      name: bad-prompt-detector
    # Configuration for the callout itself
    remoteConfig:
      timeout: "250ms"
      fail: open | closed
      # Context to send in gRPC Metadata
      context: 
        tenant_id: "customer-a"
```

## Open Design Questions

* Payload Processors MUST implement a custom (gRPC?) protocol (specifics TBD, part of a future PR). This protocol allows them to receive phases of the request lifecycle and emit one of several signals, such as:
    * "give me the next part"
    * "this request is approved"
    * rejection of the request/response
    * "this response is approved"
    * "change this part I received to be as follows: ..."
* Payload Processors MAY receive additional context as part of their configuration... Is this a real use case?
* We need to figure out whether to allow conceptual "processing loops" between `spec.rules.matches` and **mutating** payload processors.
    * Would an approach that avoids processing loops between `spec.rules.matches` and Payload Processors be making any use case impossible to implement?
* There is potential overlap between `spec.rules.filters` and `spec.rules.processors`:
    * Do we avoid it for now?
    * Processers can supplant filters, but should they?
    * If we retain both, should Processors be invoked before or after any filters? (Do we want to support both approaches?)
* How do Gateway-level Processors and HTTPRoute-level Processors co-exist?
    * Do we allow defining both? (current assumption is that it is allowed)
    * How do we co-prioritize them?
* Expressibility of concurrency between Pipeline `.spec.rules` list items (perhaps as second-level list)



# Relevant Links

* [Original Slack Discussion](https://kubernetes.slack.com/archives/C09EJTE0LV9/p1757621006832049)

