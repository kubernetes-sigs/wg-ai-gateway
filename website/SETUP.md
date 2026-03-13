# AI Gateway Working Group Website - Setup Summary

## Overview

This document summarizes the setup of the AI Gateway Working Group website at [ai-gateway.sigs.k8s.io](https://ai-gateway.sigs.k8s.io/).

## What Was Created

### Directory Structure

```
website/
├── content/                    # Markdown content files
│   ├── _index.md              # Landing page
│  
├── i18n/                       # Internationalization files
│   └── en.yaml                # English translations
├── static/                     # Static assets
│   └── images/                # Images
├── layouts/                    # Custom layouts (if needed)
├── themes/                     # Hugo themes
│   └── docsy/                 # Docsy theme (git submodule)
├── hugo.toml                   # Hugo configuration
├── package.json                # NPM dependencies
├── README.md                   # Website documentation
└── SETUP.md                    # This file
```

### Configuration Files

1. **hugo.toml** - Main Hugo configuration
   - Site metadata and baseURL
   - Docsy theme configuration
   - Community links (GitHub, Slack, Mailing List, Meeting Notes)
   - UI customization

2. **package.json** - NPM dependencies
   - Bootstrap 5
   - Font Awesome
   - PostCSS and Autoprefixer

3. **.github/workflows/hugo.yml** - GitHub Actions workflow
   - Automated build and deployment to GitHub Pages
   - Runs on pushes to `main` branch
   - Includes Node.js and Hugo setup

### Content Pages

1. **Landing Page** (`content/_index.md`)
   - Hero section with project description
   - "What is an AI Gateway?" section
   - About the Working Group
   - Get Involved section with community links
   - Quick Links to proposals and resources

2. **Proposals Section** (`content/proposals/`)
   - Proposals listing page
   - Individual proposal pages:
     - Payload Processing
     - Egress Gateways

### Theme

- **Docsy v0.14.3** - Kubernetes-standard documentation theme
  - Modified `hugo.yaml` to disable Hugo module imports for Bootstrap/Font Awesome
  - Using NPM packages instead for dependencies
  - i18n files moved to site root to avoid Hugo compatibility issues

## Building the Site

### Prerequisites

- Hugo 0.157.0+ (extended version)
- Node.js 20+
- npm

### Local Development

```bash
cd website
npm install
hugo server
```

Open browser to `http://localhost:1313`

### Production Build

```bash
cd website
npm install
hugo --gc --minify
```

Output will be in `public/` directory.

## Deployment

The website is configured for deployment to GitHub Pages via GitHub Actions:

1. Push changes to `main` branch
2. GitHub Actions workflow automatically builds and deploys
3. Site is available at the configured baseURL

### Manual Deployment Setup

To enable GitHub Pages deployment:

1. Go to repository Settings > Pages
2. Set Source to "GitHub Actions"
3. The workflow will handle the rest

## Key Design Decisions

1. **Theme Choice**: Docsy was selected because:
   - It's the standard for Kubernetes SIG projects
   - Excellent documentation-focused layout
   - Active maintenance and community support
   - Similar to agent-sandbox.sigs.k8s.io

2. **Dependency Management**: 
   - Using NPM packages instead of Hugo modules for Bootstrap/Font Awesome
   - More reliable and easier to maintain
   - Avoids Hugo module compatibility issues

3. **Content Structure**:
   - Simple, flat structure for easy maintenance
   - Proposals as individual markdown files
   - Links back to full proposals in main repo

## Maintenance

### Adding New Proposals

1. Create new markdown file in `content/proposals/`
2. Add front matter with title, description, authors, status
3. Write condensed proposal content
4. Link to full proposal in main repo

### Updating Content

- Edit markdown files in `content/`
- Run `hugo server` to preview changes
- Commit and push to trigger deployment

### Theme Updates

To update Docsy:

```bash
cd website/themes/docsy
git fetch --tags
git checkout <new-version-tag>
cd ../../
hugo --gc --minify
```

Test thoroughly after theme updates!

## Troubleshooting

### Hugo Module Errors

If you see errors about missing Hugo modules, the theme's `hugo.yaml` has been modified to use NPM packages instead. Ensure you've run `npm install`.

### i18n Errors

The i18n files are in the site root (`website/i18n/`) rather than in the theme to avoid Hugo compatibility issues.

### Build Failures

Ensure you have the correct versions:
- Hugo 0.157.0+ (extended)
- Node.js 20+

Run `hugo version` and `node --version` to check.

## Resources

- [Hugo Documentation](https://gohugo.io/documentation/)
- [Docsy Documentation](https://www.docsy.dev/docs/)
- [GitHub Pages Documentation](https://docs.github.com/en/pages)
- [Kubernetes Website Guide](https://kubernetes.io/docs/contribute/style/page-content-types/)


## Background Image

The hero section on the landing page uses a custom background image.

### Current Implementation

- **File**: `static/images/hero-bg.svg`
- **Style**: Kubernetes-themed SVG with network nodes and gradient background
- **CSS**: Defined in `assets/scss/_styles_project.scss`

### Customizing the Background

To use your own background image:

1. Place your image in `static/images/`
2. Edit `assets/scss/_styles_project.scss`
3. Update the `background-image` URL
4. Rebuild: `hugo --gc --minify`

See `static/images/README.md` for detailed instructions.
