#!/bin/bash

# Build script for native voxel renderer
echo "Building native voxel renderer..."

# Build the native renderer
go build -o voxel_planet .

if [ $? -eq 0 ]; then
    echo "Build successful! Run with: ./voxel_planet"
    echo "Options:"
    echo "  -radius float    Planet radius in meters (default 6371000)"
    echo "  -shells int      Number of spherical shells (default 20)"
    echo "  -gpu string      GPU compute backend: metal, opencl, cuda (default metal)"
    echo "  -width int       Window width (default 1280)"
    echo "  -height int      Window height (default 720)"
else
    echo "Build failed!"
    exit 1
fi