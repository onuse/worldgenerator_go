# DESIGN.md: Voxel Planet Evolution Simulator

This document outlines the architecture and design of the voxel-based planet evolution simulator.

## Core Architecture

The simulator is built around a voxel-based representation of a planet, enabling true 3D geological processes and material flow.

### Key Components

1. **Voxel Planet (`VoxelPlanet`)**: The core data structure representing the planet as a series of spherical shells, each containing voxels in a spherical coordinate system.

2. **Physics Engine (`VoxelPhysics`)**: Handles all physical simulations including:
   - Temperature diffusion and heat flow
   - Pressure calculation
   - Phase transitions (melting/solidification)
   - Material properties

3. **Advection System (`VoxelAdvection`)**: Manages material movement:
   - Convection cell formation
   - Buoyancy-driven flow
   - Material transport

4. **Server (`server.go`)**: WebSocket server that:
   - Runs the simulation loop
   - Converts voxel data to mesh format for rendering
   - Handles client connections and speed controls

5. **Web Frontend**: Three.js-based renderer that displays the planet

## Voxel Structure

The planet uses a spherical coordinate system with:
- **Shells**: Exponentially-spaced from core to surface for better surface resolution
- **Equal-area latitude bands**: Avoids pole singularities
- **Adaptive longitude divisions**: More points near equator, fewer at poles

Each voxel stores:
```go
type VoxelMaterial struct {
    Type        MaterialType  // Granite, Basalt, Water, etc.
    Density     float32      // kg/m³
    Temperature float32      // Kelvin
    Pressure    float32      // Pascals
    VelR, VelNorth, VelEast float32  // Velocity components
    Age         float32      // Material age
    Stress      float32      // Mechanical stress
    Composition float32      // 0=felsic, 1=mafic
}
```

## Simulation Pipeline

1. **Temperature Evolution**: Heat diffusion between voxels, surface solar heating/cooling, radioactive decay in core
2. **Pressure Calculation**: Weight of overlying material
3. **Phase Transitions**: Temperature/pressure-dependent melting and solidification
4. **Convection**: Temperature-driven buoyancy forces create convection cells
5. **Material Advection**: Movement of material based on velocity field
6. **Surface Extraction**: Convert voxel data to triangle mesh for rendering

## Physics Implementation

### Heat Diffusion
Uses finite difference approximation of the heat equation:
```
∂T/∂t = α∇²T
```

### Convection
Rayleigh-Bénard convection driven by temperature gradients:
- Hot material rises (positive buoyancy)
- Cold material sinks (negative buoyancy)
- Critical Rayleigh number determines convection onset

### Material Properties
Each material type has physical properties:
- Density
- Melting point
- Thermal conductivity
- Specific heat capacity
- Viscosity (temperature/pressure dependent)

## Performance Optimizations

- **Sparse Active Cells**: Only update voxels that need computation
- **Hierarchical Time Steps**: Different update rates for different processes
- **Simplified Surface Mesh**: Generate mesh only from visible surface voxels
- **Concurrent Physics**: Simulation runs in separate goroutine from server

## Future Enhancements

See `VOXEL_ROADMAP.md` for detailed development phases including:
- GPU acceleration for physics calculations
- Adaptive voxel resolution (octree subdivision)
- Advanced plate tectonics from emergent convection
- Surface processes (erosion, sedimentation)
- Climate and biosphere simulation