# GEP-XXXX: CEL Vocabulary

* Issue: [#XXXX](https://github.com/kubernetes-sigs/gateway-api/issues/XXXX)
  * Incubated by the [AI Gateway Working Group](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/proposals/fill-it-in.md)
* Status: Provisional

## TL;DR

This GEP proposes standard [Common Expression Language](https://cel.dev/) vocabulary for
evaluation of request and response attributes in Gateway policies. Presently, Gateway API
standardized access to HTTP attributes, such as header values, authority, URI path or query
parameters — but does not provide vendor agnostic way to access attributes of
protocols encapsulated in HTTP requests, such as [MCP](https://modelcontextprotocol.io/).
Operators of networks for modern workloads, require policies expressed using
protocol specific attributes, such as JSON-RPC method for security, compliance or service selection.
A lack of standard way of expressing such policies is error prone as different implementation
can evaluate the same CEL expression differently, making it particularly problematic for security
policies where precision is paramount.

The proposed CEL vocabulary covers TCP, TLS, HTTP, MCP and OpenAI protocols with strictly defined semantics
for each attribute. This proposal also defines a way for vendors to extend the standard vocabulary in
a way that makes portability less error prone.

## Goals

* Establish CEL vocabulary for accessing request and response attributes of specific protocols.
  TCP, TLS, HTTP, MCP and OpenAI are covered initially.
* Ensure that semantics of each attribute is unambiguous. For example, define attribute value when a protocol
  can carry multiple attributes with the same name.
* Provide an unambiguous way for vendors to extend the standard vocabulary.

## Overview

Traditional HTTP workloads provided all attributes necessary for evaluation of network policies, such as
service selection and RBAC, in HTTP headers. Modern workloads, particularly AI agents, encapsulate domain
specific protocols, such as MCP, in the HTTP body and do not use HTTP headers. This presents a challenge for
network operators that need to express policies using attributes native to workloads. For example it is not
presently possible to express in a portable way a policy for selection of inference service based on the
model name in the inference request.

Networking for agentic workloads is rapidly evolving, necessitating a flexible way to include new attributes
or protocols into the policy engine. For this reason it is advantageous to surface attributes of new protocols
through a CEL vocabulary, since CEL is already a [well established mechanism](https://kubernetes.io/docs/reference/using-api/cel/)
for declaring policies in Kubernetes and extending CEL vocabularies is relatively easy.

## User Stories

### As a Security Engineer

"I want to create an RBAC policy that allows a principal access to specific list of MCP tools on specific MCP
server. MCP requests to all other MCP tools are denied."

### As in AI Infrastructure Engineer

"I want to create a policy that selects a specific service based on a model name in the inference request."

### As a Developer of AI Agent

"I want to create a policy that allows specific agents access to an experimental protocol feature supported
by a specific dataplane vendor."

## Proposal

This proposal addresses the following requirements:

1. CEL vocabulary that provides access to attributes of widely used protocols. This proposal includes
   attributes for TCP, TLS, HTTP, MCP amd OpenAI protocols.
1. Well defined attribute naming and a way for vendors to extend standard vocabulary.
1. Well defined behavior for standard protocol attributes.

### Attribute Naming

Variables in the CEL vocabulary are named according to the following convention:

`namespace.[protocol.]attribute-identifier.attribute-identifier...attribute-identifier`.

Variable names always begin with the namespace that distinguishes standard and vendor specific variables. All
standard variables have the `k8s` prefix.

Namespace prefix can be followed by an optional protocol identifier if the variable corresponds to a protocol
specific attribute. Standard vocabulary defines the following protocol identifiers:

| Identifier | Protocol(s) |
|------------|-------------|
| tcp        | TCP/IP |
| tls        | TLS (all versions) |
| http       | HTTP (all versions) |
| mcp        | MCP |
| llm        | Inference |

The protocol identifier is followed by one or more identifiers that fully qualify the protocol attribute.

Dataplane vendors may extend the standard dictionary by defining attribute names outside of the reserved `k8s` namespace.
In the absence of a registry that can be used to reserve vendor namespaces, each vendor MUST choose a namespace
that does not collide with other vendors. An error SHOULD be emitted by the API controller if a CEL expression uses
unsupported attribute names.

### Attribute Semantics

In addition to the value names in the standard CEL vocabulary, the proposal also defines semantics for each
variable. Well defined semantics are necessary to address ambiguity in cases there protocols allow multiple attributes
with the same name. For example HTTP protocol allows multiple headers with the same name, likewise JSON messages
are allowed to have multiple values with the same name.

In cases where underlying protocol already established constraints on specific attributes, these constraints are
reflected in the CEL values corresponding to these attributes. For example a valid HTTP request can have only one
authority attribute and corresponding CEL identifier is a single value string.

Multivalue attributes are represented as a list of values in the same order as they were observed by the dataplane.
In cases where multivalue attributes can be interchangeably represented by a concatenated single value, CEL vocabulary
contains identifiers for both representations.

## CEL Vocabulary

### TCP Vocabulary.

| Identifier                           | Type   | Description |
|--------------------------------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| k8s.tcp.source.address               | string | Client connection remote IP address. IPv4 address is in the dot-decimal notation. IPv6 address is without square brackets and can be in compressed format. |
| k8s.tcp.source.port                  | int    | Client connection remote port. |

### TLS Vocabulary

| Identifier                                    | Type   | Description
|-----------------------------------------------|--------|------------------------------------------------------------------------------------------------|
| k8s.tls.source.is_tls                         | bool   | Indicates whether TLS is applied to the client connection. If this value is false all other TLS attributes are empty values. |
| k8s.tls.source.is_mtls                        | bool   | Indicates whether TLS is applied to the client connection and the peer certificate is presented. If this value is false all peer certificate values are empty. |
| k8s.tls.source.requested_server_name          | string | Requested server name in the client TLS connection. |
| k8s.tls.source.tls_version                    | string | TLS version of the client TLS connection. Version string is in the major.minor format. For example `1.3` |
| k8s.tls.source.subject_certificate            | string | The subject field of the peer certificate in the client TLS connection. |
| k8s.tls.source.san_certificate                | list< SAN > | List of Subject Alternative Names of the peer certificate in the client TLS connection. The SAN is a message type with two string values `{type, value}`. Supported types are case-insensitive, with the following reserved values `DNS`, `EMAIL`, `URI`, `IP` and `UPN`. If the type is not one of reserved values it is consiered a SAN OID or OID friendly name. |
| k8s.tls.source.peer_certificate               | string | PEM-encoded peer certificate in the client TLS connection if present. |

### HTTP Vocabulary

| Identifier                           | Type                        | Description |
|--------------------------------------|-----------------------------|------------------------------------------------------------------------------------------------|
| k8s.http.request.path                | string                      | The path portion of the URL, without query |
| k8s.http.request.query               | string                      | The query portion of the URL |
| k8s.http.request.path_and_query      | string                      | The path portion of the URL including the query string |
| k8s.http.request.authority           | string                      | The authority portion of the URL. It can include the port or the user info if present. |
| k8s.http.request.scheme              | string                      | The scheme portion of the URL e.g. "http" |
| k8s.http.request.method              | string                      | Request method e.g. "GET" |
| k8s.http.request.protocol            | string                      | "Request protocol ('"HTTP/1.0'", '"HTTP/1.1'", '"HTTP/2'", or '"HTTP/3'")" |
| k8s.http.request.headers             | map<string | string >       | All request headers indexed by the header name. If there are multiple headers with the same name, thier values are concatenated according to [RFC 9110, section 5.2](https://datatracker.ietf.org/doc/html/rfc9110#name-field-lines-and-combined-fi). |
| k8s.http.request.raw_headers         | list< Header >              | All request headers in the order observed by the dataplane. Multiple header with the same name are not concatenated. The Header is a message type with two string values `{name, value}`. |
| k8s.http.response.status.code        | int                         | HTTP response status code. |
| k8s.http.response.headers            | map<string | string >       | All response headers indexed by the header name, with concatenated values. |
| k8s.http.response.raw_headers        | list< Header >              | All response headers in the order observed by the dataplane. The Header is a message type with two string values `{name, value}`. |

### MCP Vocabulary

| Identifier                           | Type                        | Description |
|--------------------------------------|-----------------------------|-----------------------------------|
| k8s.mcp.request.id                   | string                      | MCP request ID. |
| k8s.mcp.request.method               | string                      | MCP request method. |
| k8s.mcp.request.tool_name            | string                      | Tool name for MCP tools/call method. |
| k8s.mcp.request.resource_uri         | string                      | Tool name for MCP resources/read method. |
| k8s.mcp.response.is_error            | bool                        | True if the response is an error response. |
| k8s.mcp.response.error.code          | int                         | MCP error code. |
| k8s.mcp.response.error.message       | string                      | MCP error message. |

### OpenAI  Vocabulary

| Identifier                           | Type                        | Description
|--------------------------------------|-----------------------------|--------------------------------|
| k8s.openai.model                     | string                      | Model name for the OpenAI request. |
