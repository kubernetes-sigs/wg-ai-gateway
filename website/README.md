# AI Gateway Working Group Website

This directory contains the source code for the AI Gateway Working Group website, deployed at [ai-gateway.sigs.k8s.io](https://ai-gateway.sigs.k8s.io/).

## Technology Stack

- **Hugo**: Static site generator
- **Docsy Theme**: Kubernetes-standard documentation theme
- **GitHub Pages**: Hosting platform

## Quick Start

### Prerequisites

- [Hugo](https://gohugo.io/installation/) (extended version)
- Git

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/kubernetes-sigs/wg-ai-gateway.git
cd wg-ai-gateway/website
```

2. Initialize and update the Docsy theme submodule:
```bash
git submodule update --init --recursive
```

3. Start the Hugo development server:
```bash
hugo server
```

4. Open your browser to `http://localhost:1313`

## Building for Production

```bash
hugo --gc --minify
```

The built site will be in the `public/` directory.

## Directory Structure

```
website/
├── content/           # Markdown content files
│   ├── _index.md     # Landing page
│   ├── proposals/    # Proposal pages
│   └── community/    # Community information
├── static/           # Static assets (images, etc.)
├── layouts/          # Custom layouts (if needed)
├── hugo.toml         # Hugo configuration
└── README.md         # This file
```

## Adding Content

### New Proposals

1. Create a new markdown file in `content/proposals/`
2. Add front matter with title, description, authors, and status
3. Write the proposal content using Markdown
4. The proposal will automatically appear on the proposals listing page

Example front matter:
```yaml
---
title: 'My Proposal'
description: 'Brief description'
authors:
  - '@github-username'
status: 'Proposed'
weight: 3
---
```

### Updating the Landing Page

Edit `content/_index.md` to update the landing page content.

## Deployment

The website is automatically deployed to GitHub Pages when changes are merged to the `main` branch. The deployment workflow is configured in `.github/workflows/hugo.yml`.

## Theme Customization

This site uses the [Docsy](https://www.docsy.dev/) theme. For theme customization options, see the [Docsy documentation](https://www.docsy.dev/docs/).

## Contributing

Contributions are welcome! Please see the main repository [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## Contact

- Slack: [#wg-ai-gateway](https://kubernetes.slack.com/messages/wg-ai-gateway)
- Mailing List: [wg-ai-gateway@kubernetes.io](https://groups.google.com/a/kubernetes.io/g/wg-ai-gateway)
- Meetings: Thursdays at 2PM EST
