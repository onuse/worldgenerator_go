# Continent Border Hard Angles Issue

## Problem Description
The continents show a mix of nice smooth borders (from initial generation) and hard rectangular angles that appear once simulation starts.

## Root Causes Identified

### 1. Initial Rectangle Shapes Still Visible
- `CreateVoxelPlanet` creates simple rectangular continents (Eurasia, Africa, etc.)
- `CreateRandomizedPlanet` should overwrite these with smooth noise-based shapes
- But rectangular boundaries may persist in some areas

### 2. Grid-Based Movement Artifacts
The advection system moves voxels between discrete grid cells:
```go
// Phase 2: Handle cell boundary transitions
if voxel.SubPosLon >= 1.0 {
    intLonMove = int(voxel.SubPosLon)
    voxel.SubPosLon -= float32(intLonMove)
}
```
This causes:
- Voxels snap to grid positions when crossing boundaries
- Creates stair-step patterns along diagonal coastlines
- Especially visible on slowly moving continents

### 3. Gap Filling Creates Sharp Edges
The `fillPlateGaps` function fills empty cells with new material:
- Creates new voxels at grid positions
- No sub-position interpolation
- Results in blocky continent edges

### 4. Limited Boundary Smoothing
Current `interpolateBoundaryProperties` only smooths:
- Temperature
- Stress
- But NOT material types or positions

## Solutions

### 1. Improve Initial Generation
- Remove rectangular continent code from `initializePlanetComposition`
- Or ensure `generateRandomContinents` fully overwrites all surface voxels

### 2. Sub-Voxel Rendering
Instead of rendering voxels as full blocks:
- Use sub-position data to offset rendering
- Interpolate between adjacent voxels
- Create smooth transitions at material boundaries

### 3. Material Transition Zones
Add a "blend factor" to voxels:
```go
type VoxelMaterial struct {
    // ...existing fields...
    BlendFactor float32 // 0=fully this material, 1=transitioning
    BlendToType MaterialType // What material we're transitioning to
}
```

### 4. Coastline Smoothing Algorithm
Post-process coastlines after movement:
```go
func smoothCoastlines(shell *SphericalShell) {
    // For each land voxel adjacent to water
    // Calculate distance to nearest water
    // Create gradient of elevation/blend based on distance
}
```

### 5. Anti-Aliasing in Texture Generation
In `voxel_texture_data.go`, when sampling for textures:
- Sample multiple points per texture pixel
- Use bilinear filtering for material boundaries
- Create smooth gradients at continent edges

## Quick Fix

The easiest immediate fix is to ensure random continents fully replace rectangular ones:

```go
// In generateRandomContinents, explicitly set all voxels first
for latIdx := range shell.Voxels {
    for lonIdx := range shell.Voxels[latIdx] {
        // Default everything to ocean first
        shell.Voxels[latIdx][lonIdx].Type = MatWater
    }
}
// Then apply continent generation...
```

## Testing

1. Check initial continent shapes (pause immediately)
2. Run simulation and watch for:
   - When hard angles appear
   - Which directions show most artifacts
   - Whether it's movement or gap-filling causing issues