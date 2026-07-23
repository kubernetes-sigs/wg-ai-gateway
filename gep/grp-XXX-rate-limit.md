# GEP-XXXX: Rate Limit API

* Issue: [#XXXX](https://github.com/kubernetes-sigs/gateway-api/issues/XXXX)
  * Incubated by the [AI Gateway Working Group](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/proposals/fill-it-in.md)
* Status: Provisional

## TL;DR

This GEP proposed APIs for configuring rate limit policies for requests flowing through Gateway. Gateway operators can use
these policies to protect their services from overload, divide capacity among different traffic classes or provide tenants
with different tiers of service.

The rate limit policy is defined as a *volume* of traffic over a rolling period of time. The volume can be expressed in bytes or
requests, including an arbitrary computation of the cost of the unit of volume. The volume can be debited from the limit on request,
response, or both traffic directions. This can allow simple policies expressed in bytes per second for forwarding TCP bytestreams or requests
per second for HTTP or gRPC requests and complex policies for limiting the number of tokens per minute for inference requests.

The scope of the rate limit policy is determined by the attachment point as well as any traffic matchers defined by the policy. In a
simple case rate limit policy attached to Gateway's [HTTPRoute](https://gateway-api.sigs.k8s.io/reference/api-types/httproute/)
is applied to all requests that match the route. A more complicated policy can additionally define rate limit buckets through
traffic matchers that apply rate limits to specific requests based on request attributes. Traffic matchers can be used to define
static rate limit buckets, for example for authenticated and un-authenticated requests, or to define a dynamically created
set of rate limit buckets, for example for equally sharing capacity among all tenants.

## Goals

* Establish API for defining rate limit policies.
* Provide dynamic way of assigning different classes of traffic to their rate limits.
* Provide flexibility for defining policies using vendor specific traffic attributes.

## Overview

Rate limiting is well established mechanism for protecting services from overload, enforcing fair sharing of capacity or
controlling costs of serving requests. Operators can express limits in bytes or requests per time unit, depending
on whether the gateway is forwarding at the network or application layer of the [OSI model](https://en.wikipedia.org/wiki/OSI_model).

In some cases all requests have the same cost and debit the limit the same amount. In other cases, requests may have different costs
and operators need to provide an expression for computing the amount debited from the rate limit (often called request cost).
Often the service reports the actual cost of processing the request in response attributes, such as custom utilization metrics
in HTTP response headers.

Modern workloads often encapsulate domain specific messages in HTTP requests. For such workloads, rate limit policies have to be
defined using attributes of domain specific messages, rather than HTTP requests that provide the transport. For example rate limits
for inference requests have to be expressed in tokens to be of practical use.

## Details

A rate limit policy defines classification criteria and rate limits for each class of incoming traffic. In the simplest case there is
no classification criteria and the rate limit is uniformly applied to all requests flowing through the Policy Enforcement Point
with the attached policy (such as Gateway, HTTPRoute or Service). In a more complex scenario operator assigns different rate limits to
authenticated and un-authenticated requests, by providing static classification criteria that distinguishes between authenticated
and un-authenticated traffic. In most complex cases Gateway is handling traffic for a multitenant service, with dynamically changing
tenants and multiple service tiers with different rate limits.

To accommodate these use cases the rate limit policy is structured as a set of rate limit buckets for fine grained traffic classification
and rate limits corresponding to these buckets. A bucket may have no classification criteria, in which case the rate limit is applied to all
traffic. Static traffic matchers allow operators to create a fixed set of buckets, in cases where traffic classes are known in advance
(i.e. authenticated vs un-authenticated). The policy also provides a way for dataplane to create rate limit buckets dynamically based on
request attributes. This allows operators to assign rate limits to a dynamic set of traffic classes. For example operators may define
a policy that creates rate limit buckets dynamically based on workload identity of request, such as SAN value of client certificate.
In this case PEP will automatically create new rate limit bucket the first time a distinct value of the request attribute is observed,
with all subsequent requests with the same attribute value debiting the limit in the created bucket.

### Global vs Local Rate Limits

A fleet of Gateways can enforce rate limit policy independently of each other, so called local rate limiting, or cooperatively by
sharing the state of rate limit buckets across the entire fleet, so called global rate limiting. Local rate limiting is typically used
to protect gateways themselves, while global rate limiting is for other use cases. This proposal defines the global rate limit policy
with the state shared among all Gateways.

### Rate Limit Enforcement

Rate limit policy is enforced on request path only. HTTP requests are rejected with the 429 HTTP status code and byte based lmits cause
throttling of incoming data from the network connection. The rate limit can however be debited on both request and response paths. This
allows rate limiting of services which provide the cost of request processing in the response attributes. Note that in such systems
Gateways can exceed the limits due the inherent lag between the time of enforcement and time of debiting the limit.

### TODO: rolling limit vs discreet refills

## User Stories

### As an Infrastructure Engineer

"I want to define a rate limit policy that prevents service backends from overloading."

### As an Security Engineer

"I want to define a policy that limits unauthenticated clients from issuing more than a set number of requests per second."

### As an IA Infrastructure Engineer

"I want to create a policy that fairly shares an overall token budget across multiple tenants."

## API Definition

```go
// XRateLimitPolicy specifies rate limit policy.
//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[-1:].type`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=direct"
type XRateLimitPolicy struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`
  Spec              RateLimitPolicySpec `json:"spec,omitempty"`
  // Status defines the status details of the XRateLimitPolicy.
  Status XRateLimitPolicyStatus `json:"status,omitempty"`
}

// RateLimitPolicySpec specifies rate limiting rules and the scope of their application.
type RateLimitPolicySpec struct {
  // TargetRefs are the resources this XRateLimitPolicy is being attached to.
  //
  // +optional
  // +kubebuilder:validation:MaxItems=16 (TODO: maybe remove this limit)
  TargetRefs []gwapiv1a2.LocalPolicyTargetReference `json:"targetRefs,omitempty"`
  // `rules` is an ordered list of rate limits together with optional bucket definitions.
  // `rules` are matched sequentially. Request cost is debited from all matching rules.
  // If any matching rate limit rule is above the limit, request is throttled.
  //
  // +kubebuilder:validation:MaxItems=128 (TODO: is this too low ?)
  // +optional
  Rules []RateLimitRule `json:"rules,omitempty"`
  // EmitRateLimitResponseHeaders controls whether X-RateLimit response headers are emitted for this rate
  // limit policy.
  //
  // +optional
  EmitRateLimitResponseHeaders *ResponseHeadersMode `json: emitRateLimitResponseHeaders,omitempty`
}

// RateLimitRule defines the policy for assigning requests to rate limit buckets
// and setting limits for them.
type RateLimitRule struct {
  // Buckets holds the list of static matchers for assigning requests to buckets
  // and a list of attributes whose values will be used for creating buckets dynamically.
  // This allows rate limits buckets to be created dynamically based on the traffic
  // observed by the dataplane. For example requests from distinct workloads as identified by
  // their client certificate can be assigned a dedicated rate limit bucket.
  //
  // All individual static match conditions must hold True for this rule
  // and its limit to be applied.
  //
  // If no bucket definitions are specified, the rule applies to all traffic of
  // the targeted scope.
  //
  // If the policy targets a Gateway, the rule applies to each Route of the Gateway.
  // Please note that each Route has its own rate limit counters. For example,
  // if a Gateway has two Routes, and the policy has a rule with limit 10rps,
  // each Route will have its own 10rps limit, unless the rule has the Shared field
  // set to "true".
  //
  // +optional
  // +kubebuilder:validation:MaxItems=128
  Buckets []RateLimitBucket `json:"buckets,omitempty"`
  // The rate limit assigned to each bucket.
  // The limit is enforced and the request is ratelimited, i.e. a response with
  // 429 HTTP status code is sent back to the client when the selected requests have
  // reached the limit.
  Limit RateLimit `json:"limit"`
  // Cost specifies the cost of requests and responses for the rule.
  //
  // This is optional and if not specified, the default behavior is to debut the rate limit by 1 on
  // the request path and do not debit the rate limit counters on the response path.
  //
  // +optional
  Cost *RateLimitCost `json:"cost,omitempty"`
  // Shared determines whether this rate limit rule applies across all the policy targets.
  // If set to true, the rule is treated as a common bucket and is shared across all policy targets.
  // Default: false.
  //
  // +optional
  Shared *bool `json:"shared,omitempty"`
  // ShadowMode indicates whether this rate-limit rule runs in shadow mode.
  // When enabled, all rate-limiting operations are performed (cache lookups,
  // counter updates, telemetry generation), but the outcome is never enforced.
  // The request always succeeds, even if the configured limit is exceeded.
  //
  // +optional
  ShadowMode *bool `json:"shadowMode,omitempty"`
}

// RateLimitBucket defines criteria for assigning requests to their rate limits.
// Rate limit buckets can be defined statically in the configuration or created dynamically
// by the dataplane using values of request attributes.
type RateLimitBucket struct {
  // A set of static matchers that determine requests that belong to the rate limit bucket.
  // All matchers must match for the request to be assigned to this bucket.
  //
  // +optional
  // +kubebuilder:validation:MaxItems=64
  StaticMatchers []RateLimitStaticMatcher `json:"staticMatchers,omitempty"`
  // A set of request attributes, whose values will be used to dynamically define rate limit
  // buckets.
  // Each distinct set of values produced from attributes produces a ratelimit bucket.
  // Note, the request attributes must be trusted and validated, to prevent unconstrained growth
  // of buckets, for example a verified identity from a client certificate or JWT, or
  // a HTTP header appended by a trusted intermediary.
  //
  // +optional
  // +kubebuilder:validation:MaxItems=64
  DynamicAttributes []RateLimitDynamicAttributes `json:"dynamicAttributes,omitempty"`
}

// RateLimitStaticMatcher defines matching criteria that assigns incoming requests to rate limit
// buckets.
type RateLimitStaticMatcher {
  // Headers is a list of request headers to match. Multiple header values are ANDed together,
  // meaning, a request must match all the specified headers.
  //
  // +optional
  // +kubebuilder:validation:MaxItems=64
  Headers []HeaderMatch `json:"headers,omitempty"`
  // CEL expression that evaluates to a `bool` value to indicate request match.
  //
  // +optional
  CelExpression *string `json:"celExpression"`
}

type RateLimitDynamicAttributes {
  // Headers is a list of request headers to match. Multiple header values are ANDed together,
  // meaning, a request MUST match all the specified headers.
  //
  // +optional
  // +kubebuilder:validation:MaxItems=64
  HeaderNames []string `json:"headerNames,omitempty"`
  // CEL expression providing dynamic value based on request attributes.
  //
  // +optional
  CelExpression string `json:"celExpression"`
}

// HeaderMatch defines the match attributes within the HTTP Headers of the request.
type HeaderMatch struct {
  // Type specifies how to match mutivalue value headers.
  // TODO: do we need this complexity? Maybe start with concatenation? In this case Values does not
  // need to be a list and can be a single value.
  //
  // +optional
  // +kubebuilder:default=Concatenated
  MultiValueMatchType *MutiValueHeaderMatchType `json:"multiValueMatchType,omitempty"`
  // Type specifies how to match against the value of the header.
  //
  // +optional
  // +kubebuilder:default=Concatenated
  ValueMatchType *HeaderValueMatchType `json:"valueMatchType,omitempty"`
  // Name of the HTTP header.
  // The header name is case-insensitive.
  // For example, "Foo" and "foo" are considered the same header.
  //
  // +kubebuilder:validation:MinLength=1
  // +kubebuilder:validation:MaxLength=256
  Name string `json:"name"`
  // Value within the HTTP header.
  //
  // +kubebuilder:validation:MinItems=1
  // +kubebuilder:validation:MaxItems=1024
  Values []string `json:"values,omitempty"`
  // Invert specifies whether the value match result will be inverted.
  //
  // +optional
  // +kubebuilder:default=false
  Invert *bool `json:"invert,omitempty"`
}

// MutiValueHeaderMatchType specifies how HTTP headers with multiple values should be compared.
// Valid MutiValueHeaderMatchType values are "Concatenated", "DistinctAll", "DistinctSubset"
// and "DistinctIntersection".
//
// +kubebuilder:validation:Enum=Concatenated;DistinctAll;DistinctSubset,DistinctIntersection
type MutiValueHeaderMatchType string

// MutiValueHeaderMatchType constants.
const (
  // MutiValueHeaderMatchConcatenated concatenates multiple value headers into a single value
  // according to RFC 9110, section 5.2 https://datatracker.ietf.org/doc/html/rfc9110#name-field-lines-and-combined-fi
  // The values are concatenated in the order in which they were observed by the dataplane.
  // For order independent matching use the "DistinctAll" match type.
  MutiValueHeaderMatchConcatenated MutiValueHeaderMatchType = "Concatenated"
  // MutiValueHeaderMatchDistinctAll matches multiple headers values individually.
  // The exact number of values and their content, must match configured Values in any order.
  MutiValueHeaderMatchDistinctAll MutiValueHeaderMatchType = "DistinctAll"
  // MutiValueHeaderMatchDistinctSubset matches multiple headers values individually.
  // A subset of header values, must match all configured Values in any order.
  MutiValueHeaderMatchDistinctSubset MutiValueHeaderMatchType = "DistinctSubset"
  // MutiValueHeaderMatchDistinctIntersection matches multiple headers values individually.
  // A subset of header values, must match at least one configured element in the Values field in any order.
  MutiValueHeaderMatchDistinctIntersection MutiValueHeaderMatchType = "DistinctIntersection"
)

type HeaderValueMatchType string

// HeaderValueMatchType constants.
// TODO: this may need to be part of the Values list.
const (
  // HeaderValueMatchExact matches the exact value of the Values field against the value of
  // the specified HTTP Header.
  HeaderValueMatchExact HeaderValueMatchType = "Exact"
  // HeaderValueMatchRegularExpression matches a regular expression against the value of the
  // specified HTTP Header. The regex string must adhere to the syntax documented in
  // https://github.com/google/re2/wiki/Syntax.
  HeaderValueMatchRegularExpression HeaderValueMatchType = "RegularExpression"
)

// RateLimitValue defines the limits for rate limiting in volume per duration.
type RateLimit struct {
  Volume uint32        `json:"volume"`
  VolumeUnit RateLimitVolumeUnit `json:"volumeUnit"`
  Duration uint32        `json:"duration"`
  TimeUnit RateLimitTimeUnit `json:"timeUnit"`
}

// RateLimitCost defines computation of values that are debited from the rate limit
// on request or response paths.
type RateLimitCost struct {
  // Cost specifier on request path. The default value is 1 if the cost specifier is ommited.
  //
  // +optional
  Request *RateLimitCostSpecifier `json:"request,omitempty"`
  // Cost specifier on response path. The default value is 0 if the cost specifier is ommited.
  //
  // +optional
  Response *RateLimitCostSpecifier `json:"response,omitempty"`
}

// RateLimitCostSpecifier specifies an expression for computing the value debited from the rate limit.
type RateLimitCostSpecifier struct {
  // CEL expression that evaluates to a positive integer value or 0. If expression if evaluate to a negative or
  // non numeric type, the result is 0.
  //
  // +optional
  CelExpression *string `json:"celExpression"`
}

// ResponseHeadersMode controls whether X-RateLimit response headers are sent for a rate limit rule.
// Valid values are "Disabled" and "DraftVersion03".
//
// +kubebuilder:validation:Enum=Disabled;DraftVersion03
type ResponseHeadersMode string

const (
  // No rate limit response headers are emitted.
  ResponseHeadersModeDisabled     ResponseHeadersMode = "Disabled"
  // Emit response header according to https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-03
  ResponseHeadersModeDraft3       ResponseHeadersMode = "DraftVersion03"
)

// RateLimitVolumeUnit specifies the traffic volume units of rate limits.
// Valid RateLimitVolumeUnit values are "Request", "Byte", "Token".
//
// +kubebuilder:validation:Enum=Request;Byte;Token
type RateLimitVolumeUnit string

// RateLimitVolumeUnit constants.
const (
  // RateLimitVolumeUnitRequest specifies the rate limit volume in requests.
  RateLimitVolumeUnitRequest RateLimitVolumeUnit = "Request"

  // RateLimitVolumeUnitByte specifies the rate limit volume in bytes.
  RateLimitVolumeUnitByte RateLimitVolumeUnit = "Byte"

  // RateLimitVolumeUnitToken specifies the rate limit volume in tokens.
  RateLimitVolumeUnitToken RateLimitVolumeUnit = "Token"
)

// RateLimitTimeUnit specifies the intervals for setting rate limits.
// Valid RateLimitTimeUnit values are "Second", "Minute", "Hour", "Day", "Month" and "Year".
//
// +kubebuilder:validation:Enum=Second;Minute;Hour;Day;Month;Year
type RateLimitTimeUnit string

// RateLimitTimeUnit constants.
const (
  // RateLimitTimeUnitSecond specifies the rate limit interval to be 1 second.
  RateLimitTimeUnitSecond RateLimitTimeUnit = "Second"

  // RateLimitTimeUnitMinute specifies the rate limit interval to be 1 minute.
  RateLimitTimeUnitMinute RateLimitTimeUnit = "Minute"

  // RateLimitTimeUnitHour specifies the rate limit interval to be 1 hour.
  RateLimitTimeUnitHour RateLimitTimeUnit = "Hour"

  // RateLimitTimeUnitDay specifies the rate limit interval to be 1 day.
  RateLimitTimeUnitDay RateLimitTimeUnit = "Day"

  // RateLimitTimeUnitMonth specifies the rate limit interval to be 1 month.
  RateLimitTimeUnitMonth RateLimitTimeUnit = "Month"

  // RateLimitTimeUnitYear specifies the rate limit interval to be 1 year.
  RateLimitTimeUnitYear RateLimitTimeUnit = "Year"
)
```

## Example Configurations for Use Stories

### As an Infrastructure Engineer

"I want to define a rate limit policy that prevents service backends from overloading."

This policy applies a 1000 QPS limit to all requests flowing to the
load-sensitive-http-service.

```yaml
apiVersion: gateway.networking.x-k8s.io/v1alpha1
kind: XRateLimitPolicy
metadata:
  name: backend-protection-rate-limit-policy
  namespace: default
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Service
    name: load-sensitive-http-service

  rules:
  - limit:
      volumeUnit: REQUESTS
      volume: 1000
      timeUnit: SECONDS
      duration: 1
```

### As an Security Engineer

"I want to define a policy that limits unauthenticated clients from issuing more than a set number of requests per second."

This policy applies a 1000 QPS limit to all requests with valid Authorization header and 1 QPS to all other requests. The Gateway validates the token presented by the client before the rate limit policy is applied.

```yaml
apiVersion: gateway.networking.x-k8s.io/v1alpha1
kind: XRateLimitPolicy
metadata:
  name: backend-protection-rate-limit-policy
  namespace: default
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: http-route

  rules:
  - limit:
      volumeUnit: REQUESTS
      volume: 1000
      timeUnit: SECONDS
      duration: 1
    buckets:
      - staticMatchers:
        - headers:
          valueMatchType: RegularExpression
          name: "authorization"
          values:
            - ".*"
  - limit:
      volumeUnit: REQUESTS
      volume: 1
      timeUnit: SECONDS
      duration: 1
    buckets:
      - staticMatchers:
        - headers:
          valueMatchType: RegularExpression
          name: "authorization"
          values:
            - ".*"
          invert: true
```


### As an IA Infrastructure Engineer

"I want to create a policy that fairly shares an overall token budget across multiple tenants."

The Gateway enforces a 100 QPS rate limit for all distinct tenants. Tenant ID comes from validated JWT "sub" claim.

// TODO: this relies on CEL vocabulary supporting access to JWT claims

```yaml
apiVersion: gateway.networking.x-k8s.io/v1alpha1
kind: XRateLimitPolicy
metadata:
  name: backend-protection-rate-limit-policy
  namespace: default
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: http-route
  rules:
  - limit:
      volumeUnit: REQUESTS
      volume: 100
      timeUnit: SECONDS
      duration: 1
    buckets:
      - dynamicAttributes:
        - celExpression: "k8s.authorization.request.jwt.claims.sub"

  emitRateLimitResponseHeaders: DraftVersion03
```
