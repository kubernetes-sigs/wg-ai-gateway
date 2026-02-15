---
title: "Introducing the AI Gateway Working Group"
date: 2026-02-28T10:00:00-08:00
canonicalUrl: https://www.kubernetes.dev/blog/2026/02/28/introducing-ai-gateway-wg/
slug: introducing-ai-gateway-wg
author: >
  [Keith Mattix](https://github.com/keithmattix)
---
The community around Kubernetes includes a number of Special Interest Groups (SIGs) and Working Groups (WGs) facilitating discussions on important topics between interested contributors. Today, we're excited to announce the formation of the [AI Gateway Working Group](https://github.com/kubernetes-sigs/wg-ai-gateway), a new initiative focused on developing standards and best practices for networking infrastructure that supports AI workloads in Kubernetes environments.
## What is an AI Gateway?
In a Kubernetes context, an *AI Gateway* refers to network gateway infrastructure (including proxy servers, load-balancers, etc.) that generally implements the [Gateway API](https://gateway-api.sigs.k8s.io/) specification with enhanced capabilities for AI workloads. Rather than defining a distinct product category, AI Gateways describe infrastructure designed to enforce policy on AI traffic, including:
- Token-based rate limiting for AI APIs
- Fine-grained access controls for inference APIs
- Payload inspection enabling intelligent routing, caching, and guardrails
- Support for AI-specific protocols and routing patterns
## Our Charter and Mission
The AI Gateway Working Group operates under a clear [charter](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md) with the mission to develop proposals for Kubernetes Special Interest Groups (SIGs) and their sub-projects. Our primary goals include:
- **Standards Development**: Create declarative APIs, standards, and guidance for AI workload networking in Kubernetes
- **Community Collaboration**: Foster discussions and build consensus around best practices for AI infrastructure
- **Extensible Architecture**: Ensure composability, pluggability, and ordered processing for AI-specific gateway extensions
- **Standards-Based Approach**: Build on established networking foundations, layering AI-specific capabilities on top of proven standards
## Active Proposals
We currently have several active proposals addressing key challenges in AI workload networking:
### Payload Processing (Proposal 7)
Our [payload processing proposal](https://github.com/kubernetes-sigs/wg-ai-gateway/tree/main/proposals/7-payload-processing.md) addresses the critical need for AI workloads to inspect and transform full HTTP request and response payloads. This enables:
**AI Inference Security:**
- Guard against malicious prompts and prompt injection attacks
- Content filtering for AI responses
- Signature-based detection and anomaly detection for AI traffic
**AI Inference Optimization:**
- Semantic routing based on request content
- Intelligent caching to reduce inference costs and improve response times
- RAG (Retrieval-Augmented Generation) system integration for context enhancement
The proposal defines standards for declarative payload processor configuration, ordered processing pipelines, and configurable failure modes - all essential for production AI workload deployments.
### Egress Gateways (Proposal 10)
Modern AI applications increasingly depend on external inference services, whether for specialized models, failover scenarios, or cost optimization. Our [egress gateways proposal](https://github.com/kubernetes-sigs/wg-ai-gateway/tree/main/proposals/10-egress-gateways.md) provides standards for securely routing traffic outside the cluster.
Key features include:
**External AI Service Integration:**
- Secure access to cloud-based AI services (OpenAI, Vertex AI, Bedrock, etc.)
- Managed authentication and token injection for third-party AI APIs
- Regional compliance and failover capabilities
**Advanced Traffic Management:**
- Backend resource definitions for external FQDNs and services
- TLS policy management and certificate authority control
- Cross-cluster routing for centralized AI infrastructure
**User Stories We're Addressing:**
- Platform operators providing managed access to external AI services
- Developers requiring inference failover across multiple cloud providers
- Compliance engineers enforcing regional restrictions on AI traffic
- Organizations centralizing AI workloads on dedicated clusters
## Community and Upcoming Events
The AI Gateway Working Group meets weekly on Thursdays at 2PM EST. You can find us on [Slack (#wg-ai-gateway)](https://kubernetes.slack.com/messages/wg-ai-gateway), our [mailing list](https://groups.google.com/a/kubernetes.io/g/wg-ai-gateway), and our [discussion board](https://github.com/kubernetes-sigs/wg-ai-gateway/discussions).
### KubeCon + CloudNativeCon Europe 2026
We're thrilled that members of our working group will be presenting at [KubeCon + CloudNativeCon Europe](https://events.linuxfoundation.org/kubecon-cloudnativecon-europe/) in Amsterdam (March 23-26). Don't miss the [Agentics Day: MCP + Agents](https://events.linuxfoundation.org/kubecon-cloudnativecon-europe/co-located-events/agentics-day-mcp-agents/) co-located event, where AI Gateway Working Group members will be discussing the intersection of AI gateways with Model Context Protocol and agent networking patterns.
This event will showcase how our proposals enable the infrastructure needed for next-generation AI agent deployments and multi-agent communication patterns.
## Looking Forward
The AI Gateway Working Group represents the Kubernetes community's commitment to standardizing AI workload networking. As AI becomes increasingly integral to modern applications, we need robust, standardized infrastructure that can support the unique requirements of inference workloads while maintaining the security, observability, and reliability standards that Kubernetes users expect.
Our proposals are currently in active development, with implementations beginning across various gateway projects. We're working closely with SIG Network on Gateway API enhancements and collaborating with the broader cloud-native community to ensure our standards meet real-world production needs.
## Get Involved
Whether you're a gateway implementer, platform operator, AI application developer, or simply interested in the intersection of Kubernetes and AI, we'd love your input. The working group follows an open contribution model - you can review our proposals, join our weekly meetings, or start discussions on our GitHub repository.
To learn more:
- Visit our [GitHub repository](https://github.com/kubernetes-sigs/wg-ai-gateway)
- Read our [charter](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md)
- Join our [weekly meetings](https://docs.google.com/document/d/1nRRkRK2e82mxkT8zdLoAtuhkom2X6dEhtYOJ9UtfZKs)
- Connect with us on [Slack](https://kubernetes.slack.com/messages/wg-ai-gateway)
The future of AI infrastructure in Kubernetes is being built today, and we invite you to help shape it with us.  