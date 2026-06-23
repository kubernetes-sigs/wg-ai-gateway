# Payload Processing

* Authors: @shaneutt, @kflynn, @shachartal, @jaellio
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

1. A GEP proposal for a `PayloadProcessor` resource in SIG Network, via
   [Gateway API], using the policy attachment pattern ([GEP-713]).
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
[GEP-713]:https://gateway-api.sigs.k8s.io/geps/gep-713/

## Gateway API - PayloadProcessor Resource

We propose a new `PayloadProcessor` CRD that attaches to a `Gateway` or
`HTTPRoute` via the standard policy attachment pattern ([GEP-713]) and defines
an ordered list of processors. Each processor is either **InProcess** (CEL
expressions evaluated in the data plane for header mutation based on body
content) or **ExtProcess** (an external gRPC service that receives the payload
for arbitrary processing). Processors execute sequentially with per-processor
failure modes, enabling composable processing pipelines such as "extract model
name from body → set routing header → reject if PII detected."

The full GEP is tracked in [GEP-XXXX: PayloadProcessor Resource].

[GEP-XXXX: PayloadProcessor Resource]:../gep/gep-XXXX-payload-processor.md

### Overview

Requirements:

* **Declarative**: The administrator of a `Gateway` or `HTTPRoute` needs to be
  able to add payload processors declaratively via policy attachment.
* **Ordered**: Processors execute sequentially in array order with
  short-circuit rejection — if any processor rejects, subsequent processors
  are skipped.
* **Two Processor Types**: **InProcess** processors use CEL expressions to
  extract data from request bodies and mutate headers or body fields (e.g. body-based
  routing). **ExtProcess** processors delegate to external gRPC services for
  arbitrary processing (e.g. PII scanning, prompt injection detection).
* **Two Processing Phases**: **PreRouting** processors execute before HTTPRoute
  matching (targets `Gateway` or `ListenerSet`), enabling body-based routing.
  **PostRouting** processors execute after route selection (targets `Gateway`,
  `ListenerSet`, or `HTTPRoute`), enabling security scanning and enrichment.
* **Failure Modes**: Each processor declares `FailClosed` (default, reject on
  error) or `FailOpen` (skip on error), enabling fine-grained control over
  behavior when processing fails.
* **Payload Influences Headers/Body**: InProcess processors use CEL to set, add, or
  remove headers or body fields, based on request content or other custom logic.
* **Rejection**: Processors can reject requests and responses (we will focus on
  requests in the first phase).
* **Reusability**: As a separate CRD, a single `PayloadProcessor` can be
  reused across multiple routes and Gateways.

### Processing Flow

```
Client Request
    │
    ▼
┌──────────────────────┐
│  PreRouting Phase    │ ◄── PayloadProcessor (targetRef: Gateway)
│  InProcess/ExtProc   │     Mutate headers/body
└──────────┬───────────┘
           │ (headers/body mutated)
           ▼
┌──────────────────────┐
│  HTTPRoute Matching  │ ◄── Standard header/path/method matching
└──────────┬───────────┘
           │ (route selected)
           ▼
┌──────────────────────┐
│  PostRouting Phase   │ ◄── PayloadProcessor (targetRef: Gateway/HTTPRoute)
│  InProcess/ExtProc   │     PII scanning, enrichment, etc.
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Backend             │
└──────────────────────┘
```

### Operational Requirements and Assumptions

* It is assumed that Payload Processors will generally operate on payloads that
  have already been decrypted before passing them to the processor. However we
  do not preclude the possibility that future implementations may want the
  payloads decrypted _by the processors_. For now we're not building for this
  use case, but we'll leave the door open.
* Gateway MUST support Processors running as workloads on the cluster, as well
  as remote endpoints. (We focus on cluster-local Services for the first phase.)
* Topology-Aware Routing (or similar constraints) are highly desirable when
  configuring Gateway consumption of Processors (both due to latency concerns,
  and also since cross-node/cross-AZ traffic may have cloud-networking costs
  associated with it).
* Processor MAY support scale based on operational metrics from Gateway.
* Processor MAY present indications of its capacity (or lack thereof) to
  Gateway. Gateway MAY support reducing the traffic load on the Processor if
  such indications are presented. In some scenarios, the user MAY prefer
  degrading payload processing over significant impact on data plane
  performance.

## Resource Model

### PayloadProcessor CRD

The `PayloadProcessor` is a namespace-scoped CRD that attaches to a `Gateway`
or `HTTPRoute` via `targetRef` (policy attachment, [GEP-713]). It contains an
ordered list of processors (1–16), each with a name, type, failure mode, and
type-specific configuration.

We chose a separate CRD with policy attachment over inline HTTPRoute filters
because:

1. Pre-routing processing (the primary use case) requires Gateway-level
   attachment, which inline filters cannot express.
2. Processing pipelines can be complex and benefit from dedicated resources.
3. Reusability across routes reduces configuration duplication.
4. Consistency with the policy attachment pattern used by other Gateway API
   extensions.

### API Definition

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha1
kind: PayloadProcessor
metadata:
  name: example-processor
  namespace: default
spec:
  # targetRef identifies the Gateway or HTTPRoute this policy applies to.
  # Follows the standard policy attachment pattern (GEP-713).
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway          # or HTTPRoute
    name: my-gateway

  # phase determines when processors execute relative to route selection.
  # PreRouting: before HTTPRoute matching (targets Gateway or ListenerSet)
  # PostRouting: after route selected (targets Gateway, ListenerSet, or HTTPRoute)
  phase: PreRouting

  # processors is an ordered list of processing steps (1-16).
  # Executed sequentially; if any processor rejects, subsequent processors
  # are skipped and the request is rejected.
  processors:
  - name: extract-model             # unique within this resource, 1-63 chars
    type: InProcess                  # InProcess or ExtProcess
    failureMode: FailClosed          # FailClosed (default) or FailOpen
    timeout: "500ms"                 # optional per-processor timeout

    # inProcess: configuration for in-process (data plane) processing.
    # Required when type is InProcess.
    inProcess:
      request:
        # set: overwrite or create headers with CEL expression values
        setHeaders:
        - name: X-Gateway-Model-Name
          value: 'json(request.body).model' # CEl expression
        - name: X-Gateway-Custom-Header
          value: '"my-custom-value"' # string literal interpreted by CEL
        # remove: remove headers by name
        removeHeaders: []
        # set: overwrite or create body fields with CEL expression values
        setBodyFields:
        - name: 'json(request.body).stream' # CEL expression
          value: '"true"' # body can be built using static fields, CEL expressions on the body of the request or response, etc.
        - name: 'json(request.body).response_format.type' # CEL expression - creates fields if they do not exist
          value: '"json_object"'
        # remove: remove body fields by name
        removeBodyFields:
        - name: temperature
  - name: pii-scanner
    type: ExtProcess
    failureMode: FailClosed
    timeout: "1s"

    # extProcess: configuration for external processor.
    # Required when type is ExtProcess.
    extProcess:
      backendRef:
        kind: Service
        name: pii-scanner-service
        port: 4444
```

### Example — Body-Based Routing

The primary use case is extracting a field from the request body (e.g. the
model name in an AI inference request) to set a header that HTTPRoute can
match on:

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha1
kind: PayloadProcessor
metadata:
  name: model-header-setter
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: ai-gateway
  phase: PreRouting
  processors:
  - name: extract-model
    type: InProcess
    failureMode: FailClosed
    inProcess:
      request:
        set:
        - name: X-Gateway-Model-Name
          value: 'json(request.body).model'
---
# HTTPRoute matches on the header set by PayloadProcessor
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: gpt4-route
spec:
  parentRefs:
  - name: ai-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v1/chat/completions
      headers:
      - name: X-Gateway-Model-Name
        value: gpt-4
    backendRefs:
    - name: gpt4-backend
      port: 8080
```

## Remaining Open Questions

* **ExtProc Wire Protocol**: The wire protocol between the gateway and external
  processor services is not standardized by the initial GEP and is deferred to
  a companion GEP. Implementations MAY use Envoy's ext_proc, a custom
  protocol, or any other mechanism in the interim.
    * Should we mandate support for Envoy ext_proc (for `extProcess`) initially
      as well as require the gateway implementation to support full-duplexing?
* **Processing Loops**: The current design avoids processing loops — PreRouting
  processors execute once, mutate headers, and then HTTPRoute matching occurs
  on the mutated headers with no re-entry.
* **Gateway and HTTPRoute Target Co-existence**: Gateway-targeted PreRouting processors
  execute first, then HTTPRoute matching, then HTTPRoute-targeted or
  Gateway-targeted PostRouting processors. If PayloadProcessors target the same
  phase with different target references (gateway vs. HTTPRoute), Gateway-level
  processors execute before HTTPRoute-level processors. If PayloadProcessors
  target the same phase with the same target reference, the newer resource is
  ignored and the older resource is used. The failure/conflict is reflected
  in the status of the newer resource.
* **CEL Cost Budgets**: A cost budget mechanism for data plane CEL evaluation
  may be needed, similar to Kubernetes admission webhook CEL cost budgets.
* **Body Buffer Size**: Whether the maximum body buffer size should be
  configurable per-PayloadProcessor or remain up to the implementation is still
  under discussion.
* **Parallel Processing**: The ability to specify and process multiple payload
  processors in parallel (both InProcess and ExtProcess). Adds complexity but
  should be considered for performance in the next phase.
* **Header and Body Modification Order**: There is no defined order for when
  headers and body modifications occur relative to each other. This could lead to
  unexpected behavior if the order matters for the processing logic.
* **InProcess and ExtProcess Processing**: ExtProcess processors are considered
  the heavy lifters of processing, while InProcess processors are more
  lightweight and suitable for final formatting and transformation task.
  ExtProcess processors are processed before InProcess processors.

## Proof of Concept

The [agentgateway PayloadProcessor POC] validates the core design:

* **CRD**: `PayloadProcessor` in `ainetworking.x-k8s.io/v0alpha0` with
  InProcess and ExtProc schema
* **Implementation**: Go controller plugin translates `InProcess` processors
  to standard policies; Rust data plane processes them with automatic body
  buffering. The controller also translates `ExtProcess` processors to
  policies which the data plane translates to Envoy `ext_proc` calls.
* **Demo**: Body-based routing with three backends (gpt-4, claude, default)
  using `json(request.body).model` CEL expression to extract model name and
  set `X-Gateway-Model-Name` header for HTTPRoute matching

[agentgateway PayloadProcessor POC]:https://github.com/kubernetes-sigs/wg-ai-gateway/pull/56

# Relevant Links

* [GEP-XXXX: PayloadProcessor Resource](../gep/gep-XXXX-payload-processor.md)
* [GEP-713: Policy Attachment](https://gateway-api.sigs.k8s.io/geps/gep-713/)
* [Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension)
* [Envoy External Processing](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter)
* [Gateway API Firewall GEP (#3614)](https://github.com/kubernetes-sigs/gateway-api/issues/3614)
* [Original Slack Discussion](https://kubernetes.slack.com/archives/C09EJTE0LV9/p1757621006832049)
