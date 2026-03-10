---
title: 'Egress Gateways'
description: 'Provide standards in Kubernetes to route traffic outside of the cluster'
authors:
  - '@shaneutt'
  - '@usize'
  - '@keithmattix'
status: 'Proposed'
weight: 2
---

## What?

Provide standards in Kubernetes to route traffic outside of the cluster.

## Why?

Applications are increasingly utilizing inference as a part of their logic. This may be for chatbots, knowledgebases, or a variety of other potential use cases. The inference workloads to support this may not always be on the same cluster as the requesting workload.

## Resources

- [Full Proposal](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/proposals/10-egress-gateways.md)
