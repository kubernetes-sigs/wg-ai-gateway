---
title: 'Payload Processing'
description: 'Define standards for declaratively adding processing steps to HTTP requests and responses'
authors:
  - '@shaneutt'
  - '@kflynn'
  - '@shachartal'
status: 'Proposed'
weight: 1
---

## What?

Define standards for declaratively adding processing steps to HTTP requests and responses in Kubernetes across the entire payload.

## Why?

Modern workloads require the ability to process the full payload of an HTTP request and response:

- **AI Inference Security**: Guard against bad prompts or misaligned responses
- **AI Inference Optimization**: Route requests based on semantics, enable caching
- **Web Application Security**: Enforce signature-based detection rules

## Resources

- [Full Proposal](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/proposals/7-payload-processing.md)
