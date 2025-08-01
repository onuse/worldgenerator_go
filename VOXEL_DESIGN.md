# Voxel-Based Planet Evolution Simulator - Architecture Design

## Core Concept
Instead of a fixed mesh with changing properties, we model the planet as a 3D grid of voxels where material can flow, transform, and interact based on physical laws.

## Coordinate System
We'll use a spherical voxel grid to avoid pole singularities and maintain roughly uniform voxel sizes:

```
- Radial shells: From core to atmosphere
- Latitude bands: Equal-area divisions
- Longitude divisions: Vary by latitude to maintain similar voxel volumes
```

## Data Structure

### Core Voxel Structure
```go
type VoxelMaterial struct {
    Type        MaterialType  // Enum: Air, Water, Basalt, Granite, Sediment, Magma, etc.
    Density     float32       // kg/m³
    Temperature float32       // Kelvin
    Pressure    float32       // Pascals
    
    // Material flow
    Velocity    Vector3       // m/s in spherical coordinates
    
    // Geological properties
    Age         float32       // Years since formation
    Composition Composition   // Mineral percentages
    
    // State
    Phase       PhaseType     // Solid, Liquid, Gas, Plasma
    Stress      float32       // For fracturing/earthquakes
}

type VoxelPlanet struct {
    // Primary voxel grid
    Shells      []SphericalShell  // Radial layers
    
    // Global properties
    Radius      float64
    Mass        float64
    Time        float64
    
    // Optimization structures
    ActiveCells map[VoxelCoord]bool  // Cells needing updates
    Boundaries  []BoundaryCell       // Inter-material boundaries
}

type SphericalShell struct {
    InnerRadius float64
    OuterRadius float64
    Voxels      [][]VoxelMaterial  // [latitude][longitude]
    Resolution  ShellResolution    // Can vary by depth
}
```

## Simulation Layers

### 1. Material Flow Layer
- Advection: Material moves based on velocity field
- Convection: Buoyancy-driven flow (hot rises, cold sinks)
- Plate motion: Surface velocity fields driven by mantle convection

### 2. Thermal Layer  
- Heat diffusion through materials
- Radioactive heating in core/mantle
- Surface cooling to space
- Phase changes (melting/solidifying)

### 3. Mechanical Layer
- Pressure from overlying material
- Stress accumulation and release (earthquakes)
- Elastic/plastic deformation
- Fracturing and faulting

### 4. Chemical Layer
- Melting/crystallization based on pressure-temperature
- Differentiation (heavy minerals sink)
- Metamorphism under heat/pressure
- Weathering at surface

## Resolution Strategy

### Adaptive Resolution
```go
type ShellResolution struct {
    LatitudeDivisions  int
    LongitudeFunc      func(latitude float64) int  // More cells at equator
}

// Example resolutions by depth:
// - Core: 20x20 (low res, slow processes)
// - Mantle: 50x50 (medium res)
// - Crust: 200x200 (high res, most activity)
// - Surface detail: 1000x1000 (where needed)
```

### Level of Detail (LOD)
- Coarse simulation for deep layers
- Fine simulation near surface and boundaries
- Dynamic subdivision where interesting things happen

## Key Algorithms

### 1. Material Advection
```go
func (p *VoxelPlanet) AdvectMaterial(dt float64) {
    // For each voxel with velocity
    // 1. Calculate destination position
    // 2. Distribute material to neighboring voxels
    // 3. Handle mass conservation
}
```

### 2. Thermal Diffusion
```go
func (p *VoxelPlanet) UpdateTemperature(dt float64) {
    // Heat equation in spherical coordinates
    // Account for material properties
    // Handle phase transitions
}
```

### 3. Plate Tectonics
```go
func (p *VoxelPlanet) UpdatePlateMotion() {
    // 1. Calculate mantle convection cells
    // 2. Derive surface velocity field
    // 3. Move crustal voxels
    // 4. Handle collisions/subduction
}
```

## Rendering Pipeline

### Surface Extraction
1. **Marching Cubes** on the crust/ocean/air boundaries
2. **Mesh optimization** to reduce triangle count
3. **Normal calculation** for smooth shading
4. **Texture coordinates** based on material properties

### Direct Volume Rendering (Optional)
- Ray marching through voxel grid
- Show internal structure (core, mantle, crust)
- Temperature/density visualization

### Hybrid Approach
```go
type RenderData struct {
    SurfaceMesh    TriangleMesh     // For terrain
    OceanMesh      TriangleMesh     // For water
    CloudParticles []Particle       // For atmosphere
    
    // Debug visualization
    VelocityField  []Arrow          // Show material flow
    TempField      []ColoredPoint   // Temperature distribution
}
```

## Implementation Phases

### Phase 1: Basic Voxel System
- Spherical voxel grid
- Simple material types (rock, water, air)
- Basic rendering (marching cubes)
- Manual velocity fields for testing

### Phase 2: Thermal Dynamics
- Temperature diffusion
- Melting/solidifying
- Convection cells
- Hot spots and cooling

### Phase 3: Mechanical Dynamics
- Pressure calculation
- Elastic deformation
- Fracturing
- Basic earthquakes

### Phase 4: Full Plate Tectonics
- Mantle convection driving surface motion
- Realistic subduction
- Mountain building
- Ocean floor spreading

### Phase 5: Surface Processes
- Erosion based on climate
- River systems carving valleys
- Sedimentation in oceans
- Glaciation cycles

## Performance Considerations

### GPU Acceleration
- Voxel updates are highly parallel
- Use compute shaders for:
  - Material advection
  - Thermal diffusion
  - Pressure calculation
  - Marching cubes

### Optimization Strategies
- Spatial hashing for active cells
- Temporal LOD (update distant things less often)
- Hierarchical grids for multi-scale phenomena
- Compression for voxel storage

## Advantages Over Mesh-Based System

1. **True 3D Geology**: Model the entire planet, not just surface
2. **Natural Physics**: Material flows, no artificial constraints
3. **Emergent Features**: Mountains, valleys, volcanoes form naturally
4. **Proper Subduction**: Oceanic crust can actually go under continental
5. **No Mesh Distortion**: Fixed grid, material flows through it
6. **Scalable Complexity**: Start simple, add features incrementally

## Next Steps

1. Implement basic spherical voxel grid
2. Add simple material types and rendering
3. Implement material advection
4. Test with simple convection patterns
5. Add plate tectonics layer by layer