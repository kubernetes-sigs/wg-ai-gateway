---
title: 'Getting Started'
description: '
Welcome to the AI Gateway Working Group! This guide will help you get started with our community and understand our work.'
weight: 1
---

## What is an AI Gateway?

The term **AI Gateway** in a Kubernetes context refers to a network Gateway (including proxies, load-balancers, etc.) which implements the [Gateway API](https://gateway-api.sigs.k8s.io/) specification and has capabilities to support networking for AI workloads.

## Key Capabilities

### AI Protocol Awareness

AI Gateways understand and can route AI-specific protocols including:
- OpenAI-compatible APIs
- Model Context Protocol (MCP)
- gRPC-based inference protocols

### Egress Gateway Support

Secure and managed egress for AI workloads:
- External AI service access
- Token management and injection
- Failover and load balancing across providers

### Payload Processing

Advanced processing of request and response payloads:
- **Guardrails**: Prompt injection detection, PII filtering
- **Semantic Routing**: Route based on request content
- **Semantic Caching**: Cache responses to reduce inference costs
- **RAG Integration**: Augment requests with additional context

## Our Mission

The AI Gateway Working Group develops capabilities for AI workloads on Kubernetes, including AI protocol awareness, egress gateway support, and payload processing.

## Quick Links

- [Prototypes](https://github.com/kubernetes-sigs/wg-ai-gateway/tree/main/prototypes) - See working prototypes
- [Contribution Guidelines](/docs/contribution-guidelines/) - Learn how to contribute
- [Community](/community/) - Get involved with our community

## Prerequisites

To participate in our working group, you should have:
- Basic understanding of Kubernetes
- Familiarity with Gateway API concepts
- Interest in AI/ML workloads on Kubernetes

## Next Steps

1. **Join our Slack** - Introduce yourself in [#wg-ai-gateway](https://kubernetes.slack.com/messages/wg-ai-gateway)
2. **Attend a meeting** - Join us every Thursday at 2PM EST
3. **Start contributing** - Read our [contribution guidelines](/docs/contribution-guidelines/)

## External Resources

- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Kubernetes Networking](https://kubernetes.io/docs/concepts/services-networking/)
- [AI Gateway WG Charter](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md)
