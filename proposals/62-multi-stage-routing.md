# Multi-Stage Request Processing

* Authors: @nirrozenbaum
* Status: Proposed

# What?

Define standards in Kubernetes for expressing a single logical client request
that is fulfilled by multiple, ordered backend interactions, rather than by a
single backend interaction.

# Why?

Routing in Kubernetes today assumes that a client request maps to one backend
interaction:

```
Client Request → Route → Backend
```

This assumption has served traditional applications well, and it is deeply
embedded in how routes, policies and observability are defined. A growing class
of workloads breaks it. For these workloads, what the client sees as one request
is fulfilled by a sequence of distinct backend interactions, each of which is
independently routable and may need its own networking configuration:

* **Inference disaggregation**: a single inference request is served by
  separate prefill and decode backends, and increasingly by separate encode,
  prefill and decode backends. Each is a distinct pool of endpoints with
  distinct routing and scaling characteristics, and selecting the right
  endpoint within a pool is itself a load-aware decision made per request. The
  interactions are also coupled: state established while serving one is
  consumed by the next.

* **Representation and protocol transformation**: an inference request arrives
  over HTTP and is first sent to a tokenizer service, after which later
  interactions operate on a tokens-in/tokens-out representation, potentially
  over a different protocol like gRPC.

* **Service composition and fan-out**: a single client request requires calls
  to several independent backends, possibly in parallel, before a response can
  be constructed. This case is not AI specific, and predates the AI workloads
  above.

Today there are several projects in CNCF landscape that are trying to solve these issues.
Taking llm-d as an example - llm-d started with having a sidecar in each decode pod to 
orchestrate a sub-request to prefill pod before running the decode phase, later on it evolved
to having a separate coordinator component which is now used to orchestrate sub-requests. 

Because there is no way to express this in routing configuration, every project builds 
the capability outside the gateway. This has three consequences, and together they 
are the motivation for this proposal:

**It fragments the ecosystem.** Projects solving the same problem arrive at
incompatible architectures. Prefill/decode disaggregation is the clearest
example: each CNCF project (like llm-d or AIBrix) implements it differently, with no shared
configuration surface between them. A platform team that adopts one of these
cannot carry its routing configuration to another, and a gateway implementation
cannot support the pattern generically because there is nothing generic to
support. The pattern is common; only the expression of it is bespoke.

**It duplicates capabilities the gateway already provides.** Once orchestration
lives outside the gateway, the sub-requests it issues fall outside the gateway's
reach — so retries, backoff, timeouts, fail-open/close behavior,
connection management and traffic policy all have to be reimplemented alongside
it, per project. This is a substantial amount of subtle, load-bearing behavior
to rebuild, and rebuilding it is not the point of any of these projects.

**It leaves operators without a coherent view of a request.** The stages of a
logical request are where the interesting behavior lives: which backend was
slow, which interaction failed, where the request was retried, where it
ended. When those interactions are invisible to the gateway, they cannot be
uniformly observed, and an operator is left correlating them by hand across
whatever each project happens to expose.

Kubernetes has a strong tradition of standardizing the networking substrate so
that workloads do not each solve it again. Multi-stage request processing is a
gap in that substrate, and it is being filled today by duplicated,
non-portable effort.

## Definitions

* **Logical request**: the request as the client issues and perceives it. One
  logical request, one client-visible response.

* **Stage**: one of the backend interactions that together fulfill a logical
  request. A stage is independently routable and may carry its own networking
  configuration.

* **Orchestration**: the workload-specific decisions made while a logical
  request is being fulfilled — which stage runs next, whether a stage is
  skipped, how the input to a stage is derived from what came before, and
  whether the exchange should end early.

> **Note**: these definitions are intended to describe the shape of the problem,
> not to prescribe a mechanism. In particular, "stage" is not meant to imply any
> existing filter, chain, or extension mechanism in any specific implementation.

## User Stories

* As an **application developer** consuming an AI Gateway:

  * I want to send one request and receive one response, without knowing how
    many backend interactions were required to serve it, so that the serving
    architecture can evolve without changing my client.

* As an **inference platform provider**:

  * I want to express that requests to a model are served by a prefill backend
    followed by a decode backend, so that I can adopt disaggregated serving
    while reducing the overhead of building networking behavior myself.

  * I want each stage to carry its own routing and networking configuration, so
    that a prefill interaction and a decode interaction can have different
    timeouts and failure behavior, as they have genuinely different performance
    characteristics.

  * I want this expressed in a portable way, so that I can change gateway
    implementation without rewriting how my workload is composed.

* As a **cluster or gateway administrator**:

  * I want to see the backend interactions caused by a logical request, so that
    I can attribute latency and failures to a specific stage rather than to the
    request as a whole.

  * I want to apply policy to individual stages, so that an interaction with an
    external service and an interaction with an in-cluster backend can be
    governed differently within the same logical request.

* As a **Gateway API implementer**:

  * I want a standard way for a workload to describe that its requests span
    multiple backend interactions, so that I can support these workloads
    generically instead of integrating with each project's bespoke mechanism.

  * I want the boundary between what the routing layer decides and what the
    workload decides to be explicit, so that I can implement the routing side
    without encoding any workload's semantics.

## Goals

* Provide a declarative way to express that a logical request is fulfilled by
  multiple backend interactions, including which stages exist, in what order
  they may execute, and what each targets.

* Allow each stage to carry its own networking configuration, so that stages
  with different performance and failure characteristics can be configured
  differently.

* Establish a clear separation between what is declared in configuration — the
  boundaries of the workflow — and the workload-specific decisions taken within
  those boundaries while a request is served.

* Reuse existing Kubernetes routing semantics for each stage, so that
  capabilities such as retries, timeouts and failure behavior do not have to be
  reimplemented.

* Remain workload agnostic. The abstraction must be usable for service
  composition and fan-out, and must not require any AI-specific understanding
  to implement.

* Make the stages of a logical request observable as related parts of one
  exchange.

# Relationship to Payload Processing

The [Payload Processing] proposal has advanced to [GEP-5091], which introduces
a `PayloadProcessor` resource: an ordered list of processing steps that read
and mutate a request or response, either inline through CEL expressions or by
calling an external processor.

The two are complementary, and the division of responsibility is
straightforward:

* **This proposal** defines which stages a logical request consists of, the
  order in which they may execute, and the networking configuration of each.

* **Payload processing** defines what happens to request and response content,
  including between stages. Whether a payload processor runs inline or as an external
  processor is a payload processing concern, and this proposal does not restate it.

Multi-stage routing doesn't introduce a mechanism of its own for
inspecting, rewriting or rejecting payloads. Gateway API should have a single
declarative vocabulary for that, and it is payload processing's.

Composing the two needs work on both sides. [GEP-5091] predates this proposal, so
it has no notion of a stage boundary to attach processing to, and its
expression context cannot yet reference the result of an earlier stage — which
is exactly what deriving one stage's input from a previous one requires. Both
look like natural extensions rather than conflicts, and should be worked out
together with the payload processing authors.

[Payload Processing]:/proposals/7-payload-processing.md
[GEP-5091]:https://github.com/kubernetes-sigs/gateway-api/pull/5092

# How?

> **This should be left blank until the "What?" and "Why?" are agreed upon,
> as defining "How?" the goals are accomplished is not important unless we can
> first even agree on what the problem is, and why we want to solve it.

# Relevant Links

* [Payload Processing proposal](/proposals/7-payload-processing.md) — addresses
  processing the full payload of a request or response, which is complementary
  to, and distinct from, spanning a request across multiple backends.
* [GEP-5091: PayloadProcessor Resource](https://github.com/kubernetes-sigs/gateway-api/pull/5092)
  ([issue #5091](https://github.com/kubernetes-sigs/gateway-api/issues/5091)) —
  the Gateway API GEP incubated from the Payload Processing proposal.
* [Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension)
  — establishes inference-aware endpoint selection for a single backend pool.
