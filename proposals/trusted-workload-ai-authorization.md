# Trusted Workload Identity for AI Gateway Authorization

* Authors: @divyaan23
* Status: Proposed

# What?

Explore common, implementation-neutral semantics for using trusted Kubernetes workload identity in AI gateway authorization decisions.

AI applications running on Kubernetes may access multiple AI providers and models through a shared gateway. While existing gateway implementations can authenticate clients, control network destinations, and apply AI-specific policies, the authenticated client identity does not necessarily represent the Kubernetes workload that originated the request.

This proposal investigates how a gateway can reliably associate an AI request with a Kubernetes workload identity and use that identity together with AI provider and model attributes to evaluate authorization at runtime.

The proposal focuses on three related areas:

1. **Trusted workload context:** What Kubernetes identity information can be used as a reliable authorization input?
2. **AI-specific authorization context:** How can provider and model identities be associated with that workload context?
3. **Common authorization semantics:** Can existing Gateway API authorization mechanisms and standardized attributes represent these relationships consistently?

The initial objective is to identify common semantics, trust requirements, and potential interoperability gaps.

This proposal does not introduce a new gateway, identity system, authorization engine, or Kubernetes API resource.

# Why?

## Shared AI gateways introduce workload-specific authorization requirements

Kubernetes applications increasingly access AI services through shared gateway infrastructure.

Multiple workloads may communicate with the same AI gateway while requiring different permissions to consume AI providers or models.

For example, consider three workloads accessing a common AI gateway:

| Kubernetes workload | Namespace | Permitted AI models |
|---|---|---|
| Finance Agent | finance | Model A |
| HR Agent | hr | Model B |
| Developer Agent | engineering | Model A, Model B |

All three workloads may use the same gateway endpoint.

Network connectivity to the gateway does not necessarily imply authorization to consume every AI provider or model accessible through it.

A gateway therefore needs sufficient trusted context to distinguish requests originating from different workloads when evaluating AI-specific authorization.

## Gateway authentication does not always establish originating workload identity

An AI gateway may authenticate requests using API keys, bearer tokens, mTLS identities, or other supported authentication mechanisms.

However, the identity authenticated at the gateway may represent:

* The originating Kubernetes workload.
* A Kubernetes ServiceAccount.
* An intermediate proxy or gateway.
* A shared application credential.
* A tenant or API consumer.

These identities are not necessarily interchangeable.

For example, several Kubernetes workloads may share the same provider API key or ServiceAccount.

A gateway cannot safely assume that a client-supplied namespace or workload header establishes the originating Kubernetes workload.

The proposal therefore explores what constitutes trusted workload identity for AI authorization and how its provenance should be understood.

## AI provider access and AI model authorization are distinct

An AI provider may expose multiple models through a single endpoint or gateway.

A workload may be authorized to access a provider while being restricted to a subset of its models.

For example:

* Finance Agent may access Provider A but only Model A.
* Developer Agent may access Provider A and use both Model A and Model B.
* An experimental workload may access a test model but not a production-approved model.

These requirements introduce authorization attributes beyond the network destination.

The relevant decision context may include:

* Authenticated Kubernetes workload identity.
* Requested AI provider or service.
* Requested AI model or model class.
* Requested operation, where relevant.

This proposal explores the common meaning of these attributes rather than defining a new authorization policy language.

## AI model selection may occur at runtime

AI provider or model selection may happen after Kubernetes workload admission and may change between requests.

For example, an application may use a shared AI gateway endpoint and specify its requested model through a request path, model alias, header, or protocol-specific payload.

A workload admission policy cannot necessarily determine which model will be requested throughout the workload's lifetime.

Consequently, AI-specific authorization may need to be evaluated using request-time context.

The proposal does not require standardized request-body inspection. Model-level authorization is considered only where the relevant model identity is reliably available.

# Definitions

* **Originating Workload:** The Kubernetes workload responsible for initiating an AI request. Its identity may not be directly observable by every gateway.

* **Authenticated Principal:** The identity established through an authentication mechanism. This may represent a ServiceAccount, application, gateway, or another principal and is not necessarily equivalent to an individual Kubernetes workload.

* **Trusted Workload Context:** Kubernetes identity or metadata attributes whose origin and integrity are established through an explicitly trusted mechanism.

* **AI Provider Identity:** An identifier representing the AI provider or service being accessed, established from trusted configuration or validated request context.

* **Requested Model Identity:** The model identifier or alias requested by the client, when reliably identifiable.

* **Resolved Model Identity:** The backend model selected by a gateway or routing implementation, when known.

* **Workload-Aware AI Authorization:** An authorization decision that evaluates trusted workload context together with relevant AI provider or model attributes.

# User Stories

## User Story 1: Trusted Workload Identity for AI Authorization

As a platform administrator, I want AI gateways to use authenticated Kubernetes workload identities and consistent authorization attributes so that access decisions can reliably distinguish workloads without trusting unverified client-supplied metadata.

**Example:**

Two Kubernetes applications access a shared AI gateway. The gateway authenticates their identities using an existing mechanism such as mTLS/SPIFFE or a validated ServiceAccount token.

The gateway should distinguish authenticated principals from untrusted namespace or ServiceAccount headers and expose the relevant trusted identity context for authorization.

This also enables different gateway implementations to evaluate equivalent authorization intent using common attribute semantics.

## User Story 2: Workload-Specific Provider and Model Authorization

As a platform administrator, I want to control which AI providers and models an authenticated Kubernetes workload can access, independently of network connectivity to the shared AI gateway.

**Example:**

Two workloads access the same gateway:

- `finance-agent` is authorized to use `provider-a/model-a`.
- `developer-agent` is authorized to use both `provider-a/model-a` and `provider-a/model-b`.

Although both workloads can reach Provider A, a request from `finance-agent` for Model B should be denied.

Authorization should evaluate the trusted workload identity together with reliably identified AI provider and model attributes.

## User Story 3: Request-Time AI Model Authorization

As a platform administrator, I want model-specific authorization to remain effective when AI model selection occurs dynamically at request time.

**Example:**

An application sends requests to a shared AI gateway endpoint and selects models through a request path, alias, or protocol-specific payload.

The gateway should evaluate the relevant authorization context for each request when the model identity is reliably available.

Where a model alias resolves to a different backend model, the authorization semantics should clarify whether the requested alias, resolved model, or both are evaluated.

# Goals

- **Trusted Workload Identity:** Define the minimum trusted Kubernetes workload identity attributes and their provenance required for AI authorization decisions.

- **AI Authorization Context:** Establish common semantics for AI provider, requested model, and resolved model attributes used alongside authenticated workload identity.

- **Authorization Interoperability:** Explore how existing Gateway API authorization mechanisms can consistently evaluate workload-to-provider/model permissions.

- **Standards Alignment:** Identify gaps in existing Gateway API capabilities, including GEP-5253, and determine whether a focused extension or shared guidance is needed.

# Non-Goals

This proposal does not aim to:

- **Introduce new authentication or authorization mechanisms**, policy engines, or Kubernetes API resources.
- **Define workload identity issuance or propagation**, relying instead on existing trusted identity mechanisms.
- **Standardize AI request parsing, payload inspection, or content safety** mechanisms.
- **Implement AI routing, model selection, fallback, or egress traffic management.**
- **Define token accounting, rate limiting, quotas, budgets, or billing.**

The focus remains on **common, trusted workload and AI provider/model authorization attributes** that existing authorization mechanisms can consume.

# How?

The proposal will explore how existing Gateway API authentication, authorization, and standardized attributes can support workload-aware AI authorization, without introducing new policy resources or enforcement mechanisms.

### 1. Establish trusted workload identity

Identify the Kubernetes workload context available through trusted authentication mechanisms, such as ServiceAccount identity or mTLS/SPIFFE.

Distinguish authenticated workload identity from untrusted request headers or metadata.

### 2. Associate AI provider and model context

Explore how a gateway can associate the authenticated workload with the requested AI provider and model.

Model identity may be obtained from trusted gateway configuration or implementation-specific request processing. The proposal does not standardize model extraction or payload inspection.

### 3. Evaluate authorization using existing mechanisms

Explore whether existing Gateway API authorization capabilities and standardized attributes can express workload-to-provider/model permissions.

For example, an illustrative authorization context could be:

```yaml
# Conceptual authorization attributes, not a Gateway API resource
workload:
  namespace: finance
  serviceAccount: finance-agent
  identityVerified: true

ai:
  provider: provider-a
  model: model-a
```

**Expected authorization behavior:**

| Workload | Provider | Model | Decision |
|---|---|---|---|
| finance-agent | provider-a | model-a | Allow |
| finance-agent | provider-a | model-b | Deny |
| finance-agent | provider-b | model-a | Deny |

These decisions assume a policy permitting only `provider-a/model-a` for the verified `finance-agent` identity.

A new authorization API will be considered only if existing mechanisms cannot express the required behavior.

# Relevant Links

* [AI Gateway Working Group Slack Discussion](https://kubernetes.slack.com/archives/C09EJTE0LV9/p1791975897834239)