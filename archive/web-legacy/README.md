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
