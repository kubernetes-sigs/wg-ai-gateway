# Payload Processing

* Authors: @shaneutt, @kflynn
* Status: Proposed

# What?

Define standards for declaratively adding processing steps to HTTP requests and
responses in Kubernetes across the entire payload, this means processing the
body of the request in addition to the headers.

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

TODO in a later PR.

> This should be left blank until the "What?" and "Why?" are agreed upon,
> as defining "How?" the goals are accomplished is not important unless we can
> first even agree on what the problem is, and why we want to solve it.
>
> This section is fairly freeform, because (again) these proposals will
> eventually find there way into any number of different final proposal formats
> in other projects. However, the general guidance is to break things down into
> highly focused sections as much as possible to help make things easier to
> read and review. Long, unbroken walls of code and YAML in this document are
> not advisable as that may increase the time it takes to review.

# Relevant Links

* [Original Slack Discussion](https://kubernetes.slack.com/archives/C09EJTE0LV9/p1757621006832049)

