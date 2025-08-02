# Import Update Guide

Due to the folder reorganization, all import paths need to be updated. Since this is a single module project, we'll keep using a single go.mod at the root, but update package declarations.

## Package Structure

### Core Package Files (package core)
- core/planet.go (was voxel_planet.go)
- core/types.go
- core/voxel_types.go
- core/sphere_geometry.go

### Physics Package Files (package physics)
- physics/advection.go
- physics/interpolation.go
- physics/mechanics.go
- physics/physics.go
- physics/physics_cpu.go
- physics/physics_gpu.go

### GPU Package Files (package gpu)
- gpu/interface.go
- gpu/types.go
- gpu/cpu.go
- gpu/stub.go
- gpu/compute.go
- gpu/compute_plates.go
- gpu/buffer_*.go files

### Rendering Package Files (package rendering)
- rendering/opengl/renderer.go (was renderer_gl.go)
- rendering/opengl/picking.go
- rendering/opengl/shaders/*.go
- rendering/opengl/overlay/*.go
- rendering/textures/voxel_texture_data.go

### Simulation Package Files (package simulation)
- simulation/plates.go (was voxel_plates.go)

## Required Changes

1. Update package declarations in moved files
2. Update imports in main.go
3. Ensure all cross-package references are correct
4. Test build

## Note
Since Go modules use the module path for imports, and this is a single module, we don't need to change import paths, just package names and ensure files are in the right packages.