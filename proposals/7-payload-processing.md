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
The most straightforward and incremental appraoch woudld be to add support for payload processing as a filter at the rule level. Here is an example of what this may look like:


```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: example-httproute
spec:
  parentRefs:
  - name: httpbin-gateway
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
    filters:
    - type: PayloadProcessing
      payloadProcessing:
      - name: guardrails
        backendRefs:
        - kind: Service
          name: sql-injection-scanner
        config:
          timeout: "500ms"
          failureMode: open | closed
          # Context to send in gRPC(?) Metadata
        context:
          tenant_id: "customer-a"
```


If the amount of details and settings we need to support at the processor level is greater than what we want in-line within a `HTTPRoute`, or if we want to re-use a single set of payload processing constructs across multiple HTTPRoute rules, we can propose a reference to a re-usable custom resource. A minimal `HTTPRoute` that references a processor pipeline is shown below.

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
    filters:
    - type: PayloadProcessingRef
      p]ayloadProcessingRef:
        name: prompt-injection-protection
---
apiVersion: gateway.networking.k8s.io/v1alpha1
kind: PayloadProcessingPipeline
metadata:
  name: prompt-injection-protection
spec:
  # Sequence of processors
  processors:
  - name: malicious-prompt-prevention
    backendRefs:
    - kind: Service
      name: sql-injection-scanner
    config:
      timeout: "250ms"
      failureMode: open | closed
      context:
        tenant_id: "customer-a"
```

## Payload Processing Protocol

Payload Processors MUST implement the **Payload Processing Protocol (PPP)** — a
gRPC-based, vendor-neutral protocol that allows the gateway to share HTTP
request and response data with processors and receive processing decisions in
return.

### Design Principles

1. **Vendor-neutral** — no proxy-specific concepts in the protocol.
2. **Phase-based** — the gateway sends lifecycle phases; the processor responds
   with actions.
3. **Streaming** — one bidirectional gRPC stream per HTTP transaction. The
   exchange is phase-by-phase: the gateway sends a phase and waits for the
   processor's response before sending the next. By default the gateway MAY
   forward processed data immediately, enabling streaming; processors that
   need to correlate later phases (e.g. mutate headers based on body) can
   signal `hold` to defer forwarding.
4. **Minimal** — the smallest useful protocol; extensible later via new
   fields and phases.
5. **Composable** — processors don't need to know about each other or about
   pipelines.

### Interaction Model

For each HTTP transaction the gateway processes, it opens **one gRPC stream**
to the processor. The exchange is **phase-by-phase**: the gateway sends a phase,
then waits for the processor's response before sending the next phase. When the
processor responds with `Continue`, the gateway MAY forward the processed data
immediately (enabling streaming). If the processor responds with `hold=true`,
the gateway MUST buffer the data — the processor will finalize it in a
subsequent phase (e.g. mutating headers after inspecting the body). The stream
closes when the transaction completes or the processor signals early
termination.

```
Gateway                              Processor
  │                                      │
  ├─── RequestHeaders ──────────────────►│
  │◄──── ProcessingAction ───────────────┤
  │                                      │
  ├─── RequestBody (chunk 1) ───────────►│
  │◄──── ProcessingAction ───────────────┤
  │                                      │
  ├─── RequestBody (chunk N, final) ────►│
  │◄──── ProcessingAction ───────────────┤
  │                                      │
  │   ... upstream processes request ... │
  │                                      │
  ├─── ResponseHeaders ─────────────────►│
  │◄──── ProcessingAction ───────────────┤
  │                                      │
  ├─── ResponseBody (final) ────────────►│
  │◄──── ProcessingAction ───────────────┤
  │                                      │
  ├─── stream close ────────────────────►│
```

A processor that only inspects requests can end early:

```
Gateway                              Processor
  │                                      │
  ├─── RequestHeaders ──────────────────►│
  │◄──── Continue{end.response=true} ────┤  ← "request is clean, skip response"
  │                                      │
  ├─── stream close ────────────────────►│
```

A processor that detects a violation can short-circuit:

```
Gateway                              Processor
  │                                      │
  ├─── RequestHeaders ──────────────────►│
  │◄──── Continue ───────────────────────┤
  │                                      │
  ├─── RequestBody ─────────────────────►│
  │◄───── ImmediateResponse{403} ────────┤  ← "guardrail violation detected"
  │                                      │
  ├─── stream close ────────────────────►│
```

A processor that mutates headers based on body content (BBR-style):

```
Gateway                              Processor
  │                                      │
  ├─── RequestHeaders ──────────────────►│
  │◄───── Continue{hold=true} ───────────┤  ← "don't forward headers yet"
  │                                      │
  ├─── RequestBody ─────────────────────►│
  │◄───── Continue{header_mutation:{     ┤  ← hold released; headers + body
  │        set: "x-model: math"}}        │    forwarded with mutations applied
  │                                      │
  ├─── stream close ────────────────────►│
```

### Transaction Cancellation

If the client or upstream server abruptly terminates the HTTP transaction
(e.g. client disconnect, server abort), the gateway MUST cancel the gRPC
stream. The processor will observe this as a standard gRPC cancellation
(`CANCELLED` status code) and MUST free any resources associated with the
`transaction_id`. No explicit abort message is needed — cancellation is
handled by the transport layer.

- **Normal stream close** (half-close after final phase response) =
  transaction completed successfully.
- **gRPC cancellation** = transaction aborted; processor SHOULD release
  associated resources.

### Static Phase Selection

Each processor declares which phases it receives in the gateway configuration.
This is the primary mechanism for controlling what data flows to a processor.
The in-protocol `EndPhases` (see below) can only **narrow** at runtime — it
cannot enable phases that were not statically configured.

```yaml
processors:
- name: sql-injection-guard
  backendRefs:
  - kind: Service
    name: sql-guard
  config:
    timeout: "250ms"
    failureMode: closed
    phases:
      requestHeaders: true
      requestBody: true
      responseHeaders: false
      responseBody: false
- name: response-pii-scanner
  backendRefs:
  - kind: Service
    name: pii-scanner
  config:
    timeout: "500ms"
    failureMode: closed
    phases:
      requestHeaders: false
      requestBody: false
      responseHeaders: true
      responseBody: true
```

### Protocol Definition (Protobuf)

```protobuf
syntax = "proto3";
package gateway.payloadprocessing.v1alpha1;

// One bidirectional stream per HTTP transaction per processor.
service PayloadProcessor {
  rpc Process(stream PayloadPhase) returns (stream ProcessingAction);
}

// ─── Gateway → Processor ───────────────────────────────────

message PayloadPhase {
  // Opaque identifier for this HTTP transaction, stable across phases.
  string transaction_id = 1;

  // Arbitrary key/value context from the gateway configuration
  // (e.g. tenant_id, pipeline stage). Set on the first message,
  // MAY be omitted on subsequent phases of the same stream.
  map<string, string> context = 2;

  oneof phase {
    HttpRequestHeaders   request_headers  = 10;
    HttpBody             request_body     = 11;
    HttpResponseHeaders  response_headers = 20;
    HttpBody             response_body    = 21;
  }
}

message HttpRequestHeaders {
  string method    = 1;
  string path      = 2;
  string authority = 3;
  string scheme    = 4;
  // All remaining headers.
  repeated HttpHeader headers = 5;
  // True when no request body follows (e.g. GET).
  bool end_of_stream = 6;
}

message HttpResponseHeaders {
  uint32 status_code = 1;
  repeated HttpHeader headers = 2;
  // True when no response body follows (e.g. 204).
  bool end_of_stream = 3;
}

message HttpBody {
  bytes body = 1;
  // True when this is the last (or only) chunk.
  bool end_of_stream = 2;
}

message HttpHeader {
  string name  = 1;
  string value = 2;
}

// ─── Processor → Gateway ───────────────────────────────────

message ProcessingAction {
  oneof action {
    // Accept this phase, optionally mutate, continue.
    Continue          continue           = 1;
    // Short-circuit: send this response to the client immediately.
    ImmediateResponse immediate_response = 2;
  }
}

message Continue {
  // Optional mutations. If absent, the phase passes through unmodified.
  HeaderMutation header_mutation = 1;
  BodyMutation   body_mutation   = 2;

  // Dynamically narrow which subsequent phases this processor
  // receives. Can only disable phases that the static configuration
  // has enabled — cannot enable new ones.
  EndPhases end_phases = 3;

  // If true, the gateway MUST NOT forward this phase's data yet.
  // The processor will provide final mutations in a subsequent phase.
  // When the processor later responds without hold (or the stream
  // ends), all held data is released with any accumulated mutations
  // applied.
  // Default (false): the gateway MAY forward processed data
  // immediately, enabling streaming.
  bool hold = 4;
}

message EndPhases {
  // Stop sending request body chunks. Meaningful during request
  // headers or between request body chunks.
  bool request_body = 1;
  // Stop sending ALL response phases (headers and body).
  // Meaningful during any request phase.
  bool response = 2;
  // Stop sending response body chunks, but still send response
  // headers. Meaningful during response headers phase.
  // Ignored if `response` is true.
  bool response_body = 3;
}

message ImmediateResponse {
  uint32 status_code          = 1;
  repeated HttpHeader headers = 2;
  bytes body                  = 3;
}

// ─── Mutations ─────────────────────────────────────────────

message HeaderMutation {
  // Set (upsert) these headers.
  repeated HttpHeader set_headers    = 1;
  // Remove headers by exact name match.
  repeated string     remove_headers = 2;
}

message BodyMutation {
  oneof mutation {
    // Replace the body content with this value.
    bytes replace = 1;
    // Clear the body entirely.
    bool  clear   = 2;
  }
}
```

### Action Semantics

| Action | When | Effect |
|---|---|---|
| `Continue{}` | Any phase | Pass through unmodified, proceed to next phase. |
| `Continue{header_mutation, body_mutation}` | Any phase | Apply mutations, proceed. |
| `Continue{end_phases.response=true}` | Request phases | "I'm done — don't show me the response." |
| `Continue{end_phases.request_body=true}` | Request headers / body | "I've seen enough of the request body." |
| `Continue{end_phases.response_body=true}` | Response headers | "I only needed to see response headers." |
| `Continue{hold=true}` | Any phase | Buffer this data; processor will finalize in a later phase. |
| `Continue{hold=true}` then `Continue{header_mutation}` | Headers then body | BBR pattern: mutate headers after inspecting body. |
| `ImmediateResponse` | Any phase | Short-circuit; gateway sends this response to client, closes stream. |

### What the Protocol Does NOT Define

The following remain gateway-level configuration concerns:

* **Timeout** per processor
* **Failure mode** (open / closed)
* **Static phase selection** (which phases to send)
* **Pipeline ordering** and concurrency between (non-mutating) processors
* **Load balancing** across processor replicas

## Open Design Questions

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

