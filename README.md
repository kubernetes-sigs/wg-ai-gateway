# AI Gateway Working Group

Proposals and discussions for the [AI Gateway Working Group].

[AI Gateway Working Group]:https://github.com/kubernetes/community/tree/master/wg-ai-gateway

## About

The term *AI Gateway* in a Kubernetes context refers to a network Gateway (including proxy servers, load-balancers, etc) which (generally) implements the [Gateway API] specification and has capabilities to support networking for AI workloads including AI protocol awareness, [egress gateway] support and [payload processing]. Rather than a distinct product category, it describes infrastructure for enforcing policy on AI traffic, such as token-based rate limiting, fine grained access controls for inference APIs, and payload inspection that enables routing, caching and guardrails.

For more general information about the working group, please see:

* Our [README] - This provides an overview of the community, contact and
  communication information.
* Our [Charter] - This describes what this WG intends to accomplish in
  detail, our general goals and areas of interest.

As a Kubernetes working group, we do not _directly_ own projects or code,
our purpose is to make proposals. As such, this repository is meant for
discussions and staging proposals which (generally) will be developed into
proposals for specific Kubernetes Special Interest Groups (SIGs) and their
sub-projects.

**Any code contained in this repo is a prototype only and NOT suitable for anything resembling production**

[README]:https://github.com/kubernetes/community/blob/master/wg-ai-gateway/README.md
[Charter]:https://github.com/kubernetes/community/blob/master/wg-ai-gateway/charter.md

### Proposals

Please see our [proposals directory](/proposals) to view the proposals we
have so far. If you'd like to submit a new proposal, please contact us in one of
our communication channels to get some feedback and identify potential
supporters for your proposal in the community who can help move it forward with you.

### Prototypes

You are welcome to create and share prototypes for any WG proposal under the [prototypes directory](/prototypes)
in `main` branch. This enables collaboration, experimentation, and knowledge sharing across contributors.

However, this Working Group (WG) does not own or maintain production code. All code in this repository is provided strictly for PoC, experimentation, and discussion purposes only. It should not be considered production-ready, officially supported, or suitable for direct reuse in other projects.

More details can be found in the [prototypes directory](/prototypes).

Long term, proposals and any mature implementations originating from this WG are expected to move into appropriate Kubernetes subprojects or other relevant upstream repositories, where formal ownership and maintenance can be established.

## Community, discussions, contributions, and support

Our community meetings are held weekly on Thursday 2PM EST ([Calendar](https://www.kubernetes.dev/resources/calendar/), [Meeting Notes](https://docs.google.com/document/d/1nRRkRK2e82mxkT8zdLoAtuhkom2X6dEhtYOJ9UtfZKs)). [Convert to your timezone](https://dateful.com/time-zone-converter?t=2PM&tz=ET%20%28Eastern%20Time%29).

You can reach out to members of the community using the following communication channels:
- [Slack channel wg-ai-gateway](https://kubernetes.slack.com/messages/wg-ai-gateway)
- [Mailing List](https://groups.google.com/a/kubernetes.io/g/wg-ai-gateway)
- [Discussion Board](https://github.com/kubernetes-sigs/wg-ai-gateway/discussions)

Contributions are readily welcomed!

Learn how to engage with the Kubernetes community on the [community page](https://kubernetes.io/community/).

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
