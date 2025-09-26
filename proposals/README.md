# Proposals

This is where proposals for the [AI Gateway Working Group] live.

[AI Gateway Working Group]:https://github.com/kubernetes/community/tree/master/wg-ai-gateway

## Guidelines

Proposals you find here are meant to eventually find their way to the
Kubernetes Special Interest Groups (SIGs) and their sub-projects, which means
that the final proposal which stems from a proposal here may follow a format
or guidelines specific to that group. As such our guidelines are very high
level and mostly focused on building general consensus.

Please see our [proposal template](/proposals/template.md) which includes
more guidance on how proposals should be written.

## Process

As a general rule, we don't want to have too much process, so you may find our
process here kind of light, and that's intentional: we expect we can work
through most details through communication channels.

That said, to ensure we have enough structure to avoid confusion, we do
have a high-level process that should be followed (unless there are specific
reasons not to):

1. Communicate your proposal in the community before making a PR. Share the
   general idea you want to cover, and seek feedback. Look for opportunities
   to find co-authors or collaborators that can join the effort.
2. Create an initial PR. This should only include "What?" and "Why?". That is,
   what the proposal is trying to solve at a high level, and the motivation
   and goals to do it. Provide user stories, as these can help illustrate the
   need from various perspectives. For the first PR, there should be no "How?":
   stay away from implementation details, code and APIs at first as these can
   take a long time to build consensus on, and need specific focus. Let's be
   sure the community agrees on the "What?" and "Why?" first.
3. Later PRs can start introducing the "How?". This needs to start by
   identifying which SIG and sub-project(s) the proposal is suggesting to bring
   itself to, or if its proposing new sub-projects. It should include high
   level technical details as to how this can be accomplished. Extensive low
   level details _may_ be needed, but when in doubt less may be more at this
   stage. For any details you want to add, consider if those can be added as
   their own iteration and PR, so they can be reviewed in focus.
4. Repeat the cycle of iterations for the "What?", "Why?" and "How?". Prefer to
   have smaller more focused PRs over massive ones with many changes and
   themes.
5. Ultimately: Share the proposal with the SIG/sub-project(s) that its intended
   for, and start the process of moving this proposal to those destinations.
   Update the proposal here with links to that, and mark the proposal as
   `Succeeded` to indicate that we're done with the proposal here and it has
   moved on to these other places.

The WG leads may mark a proposal `WithDrawn` if it is unable to move forward
for some reason. Any PR that moves a proposal to `WithDrawn` will include a
written note explaining the rationale.

### Statuses

- `Proposed` - Early discussion, consensus is not built yet. Focus on the
  "What?" and "Why?" at this phase.
- `InProgress` - The proposal is generally accepted by the working group, and
  now its time to figure out the details, and fill in the "How?".
- `Succeeded` - We got far enough that the proposal was ready to be sent out
  to destination SIGs/sub-projects. It is concluded on this end.
- `WithDrawn` - For one reason or another, we're not moving forward with this
  one. Details on why will be noted in the proposal.
