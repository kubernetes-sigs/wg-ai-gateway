---
title: 'AI Gateway Working Group'
description: 'Proposals and prototypes for AI Gateway capabilities in Kubernetes'
---

{{< blocks/cover title="AI Gateway Working Group" image_anchor="center" height="full" >}}

<div class="px-3">

The AI Gateway Working Group develops proposals for AI Gateway capabilities in Kubernetes, including AI protocol awareness, egress gateway support, and payload processing.

<div class="mt-5">
{{< blocks/link-down color="info" >}}
</div>

</div>

{{< /blocks/cover >}}

{{% blocks/lead color="primary" %}}

## What is an AI Gateway?

The term **AI Gateway** in a Kubernetes context refers to a network Gateway (including proxies, load-balancers, etc.) which implements the [Gateway API](https://gateway-api.sigs.k8s.io/) specification and has capabilities to support networking for AI workloads.

This includes:
**AI Protocol Awareness** - Understanding and routing AI-specific protocols

**Egress Gateway Support** - Secure egress for AI workloads

**Payload Processing** - Token-based rate limiting, access controls, routing, caching, and guardrails

{{% /blocks/lead %}}

{{% blocks/section color="white" %}}

## About Our Working Group

As a Kubernetes working group, we do not _directly_ own projects or code. Our purpose is to make proposals that will be developed into proposals for specific Kubernetes Special Interest Groups (SIGs) and their sub-projects.

**Any code contained in this repo is a prototype only and NOT suitable for production use.**

### Our Goals

- Develop proposals for AI Gateway capabilities in the Kubernetes ecosystem
- Build consensus around AI networking requirements
- Work with relevant SIGs to implement standardized solutions
- Enable policy enforcement on AI traffic including token-based rate limiting and fine-grained access controls

{{% /blocks/section %}}

{{% blocks/section color="light" type="row" class="text-right" %}}

## Get Involved

We welcome contributions from the community! Here's how you can get involved:

{{% blocks/feature icon="fab fa-github" title="Contributions Welcome" url="https://github.com/kubernetes-sigs/wg-ai-gateway" %}}
Submit pull requests, explore our proposals, and help shape the future of AI Gateways in Kubernetes.
{{% /blocks/feature %}}

{{% blocks/feature icon="fab fa-slack" title="Connect With Us" url="https://kubernetes.slack.com/messages/wg-ai-gateway" %}}
Chat with other contributors in the `#wg-ai-gateway` channel.
{{% /blocks/feature %}}

{{% blocks/feature icon="fas fa-envelope" title="Join the Mailing Group" url="https://groups.google.com/a/kubernetes.io/g/wg-ai-gateway" %}}
Stay updated with our discussions and announcements.
{{% /blocks/feature %}}

<!-- {{% blocks/feature icon="fas fa-calendar" title="Weekly Meetings" url="https://docs.google.com/document/d/1nRRkRK2e82mxkT8zdLoAtuhkom2X6dEhtYOJ9UtfZKs" %}}
Join us every Thursday at 2PM EST. [Convert to your timezone](https://dateful.com/time-zone-converter?t=2PM&tz=ET%20%28Eastern%20Time%29).
{{% /blocks/feature %}} -->

{{% /blocks/section %}}

{{% blocks/section color="white" class="text-right" %}}

## Quick Links

- [View Our Proposals](/proposals/) - Explore current and completed proposals
- [Prototypes](https://github.com/kubernetes-sigs/wg-ai-gateway/tree/main/prototypes) - See working prototypes
- [Charter](https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md) - Learn about our mission and goals
- [README](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/README.md) - General information about the working group

{{% /blocks/section %}}

{{% blocks/section color="light" %}}

## Community

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](https://github.com/kubernetes-sigs/wg-ai-gateway/blob/main/code-of-conduct.md).

Learn how to engage with the Kubernetes community on the [community page](https://kubernetes.io/community/).

{{% /blocks/section %}}
