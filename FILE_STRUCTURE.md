# Voxel Planet Project File Structure

## Core Application Files
- `main.go` - Entry point, simulation loop
- `types.go` - Basic type definitions
- `settings.go` / `settings.json` - Configuration

## Voxel Planet System
- `voxel_planet.go` - Planet structure and initialization
- `voxel_types.go` - Voxel material types and properties
- `voxel_physics.go` - Physics simulation (CPU)
- `voxel_physics_gpu.go` - GPU physics dispatch
- `voxel_mechanics.go` - Tectonics and mechanical processes
- `voxel_advection.go` - Advection calculations
- `voxel_interpolation.go` - Interpolation utilities
- `sphere_geometry.go` - Spherical grid generation

## GPU Compute
- `gpu_interface.go` - GPUCompute interface definition
- `gpu_types.go` - GPU data structures
- `gpu_buffer_share.go` - Shared buffer management
- `gpu_cpu.go` - CPU fallback implementation
- `gpu_metal.go` / `gpu_metal.m` - Metal implementation (macOS)
- `gpu_metal_interface.go` - Metal Go interface
- `gpu_metal_interface_stub.go` - Metal stub for non-macOS
- `gpu_metal_kernel_methods.go` - Metal kernel implementations
- `gpu_metal_methods.go` - Metal utility methods
- `gpu_stub.go` - Stub implementations
- `opencl_compute.go` / `opencl_compute_stub.go` - OpenCL (not implemented)

## Rendering System
- `renderer_gl.go` - Main OpenGL renderer
- `renderer_gl_raymarch.go` - Ray marching shader (primary renderer)
- `renderer_gl_splat.go` - Splat rendering shader (alternative)
- `renderer_gl_voxels.go` - Voxel shader (legacy/unused)
- `renderer_gl_direct.go` - Direct rendering utilities
- `voxel_texture_data.go` - Texture management for GPU

## Build Scripts
- `build.bat` - Windows build script
- `build_fast.bat` - Fast build with caching
- `go.mod` / `go.sum` - Go module files

## Documentation
- `README.md` - Project overview
- `CLAUDE.md` - Claude AI assistant documentation
- `DESIGN.md` - System design
- `VOXEL_DESIGN.md` - Voxel system design
- `VOXEL_ROADMAP.md` - Development roadmap

## Files to Remove (Debug/Test)
- `debug_voxels.go` - Debug tool for voxel inspection
- `debug_continents.go` - Continent generation debug
- `debug_*.txt` - Debug outputs
- `renderer_gl_tests.go` - Basic rendering tests
- `renderer_gl_raymarch_tests.go` - Ray march tests  
- `renderer_gl_raymarch_volume_tests.go` - Volume tests
- `renderer_gl_material_blend_test.go` - Material blend tests
- `renderer_gl_raymarch_compare_test.go` - Comparison tests
- `renderer_gl_raymarch_debug_test.go` - Debug tests
- All Python path scripts (`*python*.ps1`)
- Old build/run scripts

## Archive Folder
Contains old web-based implementation (archived, not used)