# GEP-XXXX: Rate Limit API

* Issue: [#XXXX](https://github.com/kubernetes-sigs/gateway-api/issues/XXXX)
  * Incubated by the [AI Gateway Working Group](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/proposals/fill-it-in.md)
* Status: Provisional

## TL;DR

This GEP proposed APIs for configuring rate limit policies for requests flowing through Gateway. Gateway operators can use
these policies to protect their services from overload, divide capacity among different traffic classes or control costs
of serving requests.

The rate limit policy is defined as a *volume* of traffic per unit of time. The volume can be debited from the limit on request,
response, or both traffic directions. The volume can be expressed in bytes or requests, including an arbitrary computation of the cost
of the unit of volume. This can allow simple policies expressed in bytes per second for forwarding TCP bytestreams or requests
per second for HTTP or gRPC requests and complex policies for limiting the number of tokens per minute for inference requests.

The scope of the rate limit policy is determined by the attachment point as well as any traffic matchers defined by the policy. In a
simple case rate limit policy attached to Gateway's [HTTPRoute](https://gateway-api.sigs.k8s.io/reference/api-types/httproute/)
is applied to all requests that match the route. A more complicated policy can additionally define rate limit buckets through
traffic matchers that apply rate limits to specific requests based on request attributes. Traffic matchers can be used to define
static rate limit buckets, for example for authenticated and un-authenticated requests, or to define a dynamically created
set of rate limit buckets, for example for equally sharing capacity among all tenants.

## Goals

* Establish API for defining rate limit policies.
* Provide flexibility for defining policies using vendor specific traffic attributes.

## Overview

Rate limiting is well established mechanism for protecting services from overload, enforcing fare sharing of capacity or
controlling costs of serving requests. Operators can express limits in bytes or requests per time unit, depending
on whether the gateway is forwarding at the network or application layer of the [OSI model](https://en.wikipedia.org/wiki/OSI_model).

In some cases all requests have the same cost and debit the limit the same amount. In other cases, requests may have different costs
and operators need to provide an expression for computing the amount debited from the rate limit (often called request cost).
Often the service reports the actual cost of processing the request in response attributes, such as custom utilization metrics
in HTTP response headers.

Modern workloads often encapsulate domain specific messages in HTTP requests. For such workloads, rate limit policies have to be
defined using attributes of domain specific messages, rather than HTTP requests that provide the transport. For example rate limits
for inference requests have to be expressed in tokens to be of practical use.

## User Stories

### As an Infrastructure Engineer

"I want to define a rate limit policy that prevents service backends from overloading."

### As an Security Engineer

"I want to define a policy that limits unauthenticated clients from issuing more than a set number of requests per second."

### As an IA Infrastructure Engineer

"I want to create a policy that fairly shares an overall token budget across multiple tenants."

### API Definition

```yaml
apiVersion: gateway.networking.x-k8s.io/v1alpha1
kind: XRateLimitPolicy
metadata:
  name: example-rate-limit-policy
  namespace: default
spec:
  # targetRef identifies the HTTPRoute, Service or Backend this policy applies to.
  # Follows the standard policy attachment pattern (GEP-713).
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute          # or Service, or Backend (maybe Gateway)
    name: my-http-route

  # `rules` is an ordered list of rate limits together with optional traffic matchers.
  # `rules` are matched sequentially. Request cost is debited from all matching rules.
  # If any matching request buckets are above the limit, request is throttled.
  rules:
  - name: rule-name                  # unique user defined name within this resource
    limit:
      volumeUnits: REQUESTS          # REQUESTS, BYTES or TOKENS
      volume: 1024                   # bytes or requests are exclusive
      timeUnit: SECONDS              # MINUTES, HOURS, DAYS, WEEKS, MONTHS, ? YEARS
      timeValue: 10

    trafficDirection: REQUEST        # REQUEST, RESPONSE or BOTH (ignored when volume units are TOKENS)
    costExpression: 'k8s.http.response.headers["x-server-cpu-cost"]' # CEL expression for cost computation (TODO: may need different computation on request and response paths)
    shadowMode: false                # set to true to prevent request rejection (i.e. to test policy rollout)
    shared: false                    # set to true to share the bucket defined by this rule across all attached targets.
    trafficMatcher:
      celMatcher: 'k8s.llm.request.model == "deepseek-v3"'
      httpMatch:                     # same schema as Gateway's [HTTPRouteMatch](https://gateway-api.sigs.k8s.io/reference/api-spec/main/spec/#httproutematch)
        headers:
        - type: Exact
          name: 'x-some-header'
          value: 'some-value'

  matchFailureMode: FailClosed          # FailClosed (default) or FailOpen
  emitRateLimitResponseHeaders: DRAFT-3         # or DISABLED (DRAFT-3 is https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-03) (TODO: does this need to be per-rule?)
```
