# Background Images

This directory contains background images for the website.

## Hero Background

The file `hero-bg.svg` is used as the background image for the hero/cover section on the landing page.

### Current Image

The current background is an SVG with:
- Dark blue gradient background (Kubernetes-style colors)
- Abstract network nodes and connections
- Subtle hexagon pattern (referencing Kubernetes logo)

### Replacing the Background Image

To use your own background image:

1. **Add your image file** to this directory (e.g., `my-background.jpg`)

2. **Update the CSS** in `/assets/scss/_styles_project.scss`:
   ```scss
   .td-cover-block--height-full {
     background-image: linear-gradient(rgba(0, 0, 0, 0.5), rgba(0, 0, 0, 0.5)), url('/images/my-background.jpg');
     background-size: cover;
     background-position: center;
     background-repeat: no-repeat;
   }
   ```

3. **Rebuild the site**:
   ```bash
   hugo --gc --minify
   ```

### Recommended Image Specifications

- **Format**: SVG (for graphics), JPG (for photos), or PNG (for transparency)
- **Size**: 1920x1080 pixels or larger
- **File Size**: Keep under 500KB for fast loading
- **Style**: Darker images work better as the text is white
- **Overlay**: The CSS adds a dark overlay (rgba) to ensure text readability

### Alternative: Solid Color

To use a solid color instead of an image:

```scss
.td-cover-block--height-full {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}
```

### Testing

After making changes, always test:
1. Run `hugo server` for local preview
2. Check on different screen sizes (mobile, tablet, desktop)
3. Ensure text is readable on the background
