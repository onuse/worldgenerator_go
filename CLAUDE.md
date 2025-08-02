# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a voxel-based planet evolution simulator written in Go that models realistic geological processes including mantle convection, plate tectonics, and surface evolution. The project uses GPU acceleration (Metal on macOS, OpenCL cross-platform) for physics simulation and native OpenGL for real-time 3D visualization.

## Build Commands

```bash
# Build the native renderer executable
./build.sh

# Or build manually
go build -o voxel_planet .

# Run with default parameters
./voxel_planet

# Run with custom parameters
./voxel_planet -radius 6371000 -shells 20 -gpu metal -width 1280 -height 720
```

## Architecture

### Core Components

- **Voxel Planet** (`voxel_planet.go`): Spherical voxel grid with exponentially-spaced shells from core to surface
- **Physics Simulation** (`voxel_physics*.go`): Temperature diffusion, material flow, phase transitions
- **GPU Compute** (`gpu_*.go`): Hardware-accelerated physics using Metal (macOS) or OpenCL
- **Native Renderer** (`renderer_gl*.go`): OpenGL 4.1 visualization with multiple rendering modes
- **Shared Buffers** (`gpu_buffer_share.go`): Zero-copy GPU buffer sharing between compute and render

### Key Data Structures

- `VoxelPlanet`: Main planet structure with spherical shells
- `VoxelMaterial`: Material properties (type, temperature, density, velocity)
- `GPUCompute`: Interface for GPU physics computation
- `VoxelRenderer`: OpenGL rendering pipeline
- `SharedGPUBuffers`: Manages GPU memory sharing

### Visualization Modes

1. Material type (rock, water, air, magma)
2. Temperature distribution
3. Velocity fields
4. Geological age

## Development Workflow

### Adding New Features

1. Check `VOXEL_ROADMAP.md` for current development phase and priorities
2. GPU compute kernels go in `gpu_metal_kernel_methods.go` (Metal) or OpenCL kernel files
3. Rendering shaders are embedded in `renderer_gl_*.go` files
4. Physics algorithms are implemented in `voxel_physics*.go`

### Current Development Focus

The project is in Phase 5.2: Voxel Data Visualization. Priority tasks:
- Fix GPU buffer structure for shader access
- Implement basic voxel sampling in shaders
- Add surface rendering with material-based coloring
- Enable visualization modes (temperature, velocity, age)

### Performance Considerations

- Target: 60+ FPS with full planet visualization
- Use GPU for all heavy computation
- Minimize CPU-GPU data transfers
- Profile with Metal/OpenGL debugging tools

## Important Notes

- The project transitioned from a web-based architecture (archived in `archive/web-legacy/`) to native rendering
- Windows builds require CGO_ENABLED=1 and MinGW-w64 (use build.bat or build_fast.bat)
- GPU acceleration: Metal works on macOS, CPU fallback on Windows/Linux (OpenCL/CUDA not implemented)
- Cross-sections work with X/Y/Z keys
- No automated tests currently - manual testing required

## Known Issues & Fixes

### Ocean/Continent Dithering (FIXED)
The rendering showed dithering artifacts at ocean/continent boundaries. Root causes:
1. **Texture resolution mismatch**: 512x512 texture for 360xN voxel grid caused aliasing
2. **Shell boundary sampling**: Floating-point precision issues when sampling exactly at shell boundaries
3. **Noise amplification**: Procedural noise color variation made aliasing more visible

**Solution**:
- Changed texture resolution to 360x360 to match voxel longitude resolution
- Sample at `radius * 0.999` instead of exact surface to avoid shell boundaries  
- Removed noise-based color variation in the shader

**Files affected**: `voxel_texture_data.go` (line 25), `renderer_gl_raymarch.go` (line 190)