#!/bin/bash

echo "Cleaning up web-based code..."

# Remove web server files
echo "Removing web server files..."
rm -f server.go
rm -f server_texture.go
rm -f voxel_texture.go
rm -f voxel_texture_smooth.go
rm -f voxel_render.go
rm -f main.go  # Old web-based main

# Remove web frontend
echo "Removing web frontend..."
rm -rf web/

# Remove log files
echo "Removing log files..."
rm -f server.log
rm -f server_debug.log

# Remove old binary
echo "Removing old web binary..."
rm -f worldgenerator

echo "Cleanup complete!"
echo ""
echo "Remaining files are focused on:"
echo "- Native voxel renderer (renderer_gl.go, main_native.go)"
echo "- GPU compute (gpu_*.go)"
echo "- Voxel physics simulation (voxel_*.go)"
echo "- Project documentation (*.md)"