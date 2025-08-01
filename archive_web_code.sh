#!/bin/bash

echo "Archiving web-based code..."

# Create archive directory structure
echo "Creating archive directories..."
mkdir -p archive/web-legacy
mkdir -p archive/web-legacy/web/static

# Archive web server files
echo "Archiving web server files..."
mv -f server.go archive/web-legacy/ 2>/dev/null || true
mv -f server_texture.go archive/web-legacy/ 2>/dev/null || true
mv -f voxel_texture.go archive/web-legacy/ 2>/dev/null || true
mv -f voxel_texture_smooth.go archive/web-legacy/ 2>/dev/null || true
mv -f voxel_render.go archive/web-legacy/ 2>/dev/null || true
mv -f main.go archive/web-legacy/ 2>/dev/null || true  # Old web-based main

# Archive web frontend
echo "Archiving web frontend..."
mv -f web/index.html archive/web-legacy/web/ 2>/dev/null || true
mv -f web/index_texture.html archive/web-legacy/web/ 2>/dev/null || true
mv -f web/static/terra.js archive/web-legacy/web/static/ 2>/dev/null || true
mv -f web/static/terra_texture.js archive/web-legacy/web/static/ 2>/dev/null || true

# Remove empty web directory
rmdir web/static 2>/dev/null || true
rmdir web 2>/dev/null || true

# Archive log files
echo "Archiving log files..."
mv -f server.log archive/web-legacy/ 2>/dev/null || true
mv -f server_debug.log archive/web-legacy/ 2>/dev/null || true

# Archive old binary
echo "Archiving old web binary..."
mv -f worldgenerator archive/web-legacy/ 2>/dev/null || true

# Create a README in archive
cat > archive/web-legacy/README.md << 'EOF'
# Web-Based Legacy Code

This directory contains the archived web-based implementation of the planet simulator.

## Contents

- `server.go` - HTTP server implementation
- `server_texture.go` - Texture-based rendering endpoint
- `voxel_texture.go` - Voxel to texture conversion
- `voxel_texture_smooth.go` - Bilinear interpolation for textures
- `voxel_render.go` - Mesh generation for Three.js
- `main.go` - Original main entry point with web server
- `web/` - Frontend HTML and JavaScript files
- Log files and old binaries

## Why Archived

The project transitioned from a web-based architecture to a native OpenGL renderer for:
- Better performance with direct GPU access
- Zero-copy buffer sharing between compute and render
- More sophisticated voxel visualization capabilities
- Elimination of mesh conversion overhead

## Date Archived

$(date)
EOF

echo ""
echo "Archive complete! Legacy code moved to: archive/web-legacy/"
echo ""
echo "Current project structure is now focused on:"
echo "- Native voxel renderer (renderer_gl.go, main_native.go)"
echo "- GPU compute (gpu_*.go)"
echo "- Voxel physics simulation (voxel_*.go)"
echo "- Project documentation (*.md)"
echo ""
echo "The legacy web code is preserved in archive/web-legacy/ for reference."