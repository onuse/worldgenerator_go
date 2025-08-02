#!/bin/bash
# Build script for Linux/WSL
set -e

echo "Building voxel planet..."
export CGO_ENABLED=1

# Build the executable
go build -o voxel_planet .

echo "Build complete: ./voxel_planet"