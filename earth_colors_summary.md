# Earth-like Colors and Shell-to-Shell Movement

## Completed Features

### 1. Earth-like Color Scheme
Updated the elevation visualization (Mode 8) with realistic Earth colors:

**Ocean Depths:**
- Deep Ocean (< -4000m): Dark blue `rgb(0.05, 0.1, 0.3)`
- Mid Ocean (-4000 to -2000m): Medium blue gradient
- Shallow Ocean (-2000 to -200m): Light blue gradient
- Continental Shelf (-200 to 0m): Very light blue

**Land Elevations:**
- Beaches (0-50m): Sandy tan `rgb(0.76, 0.7, 0.5)`
- Plains (50-500m): Grass green
- Hills (500-1500m): Dark green with brown hints
- Mountains (1500-3000m): Grey-brown rock
- High Mountains (3000-4500m): Darker grey
- Snow Caps (> 4500m): White with blue tint `rgb(0.95, 0.96, 0.98)`

**Material Colors:**
- Water: Realistic ocean blue `rgb(0.15, 0.4, 0.7)`
- Basalt: Dark grey volcanic rock
- Granite: Earth-like continental green

### 2. Shell-to-Shell Movement System

Implemented vertical movement between shells for realistic plate tectonics:

**Subduction (Downward):**
- Tracks `SubPosR < -1.0` for movement to lower shell
- Oceanic crust (basalt) transforms when subducting
- Density increases 10% under pressure
- Partial melting occurs at temperatures > 1400K
- Surface gaps filled with ocean water

**Rising Material (Upward):**
- Tracks `SubPosR > 1.0` for movement to upper shell  
- Magma cools 50K per shell while rising
- Solidifies to basalt < 1200K near surface
- Continental crust gets elevation boost for mountains
- Mantle material fills gaps from below

**Material Transformations:**
- Added `MeltFraction` field to track partial melting
- Handles basalt → high-pressure phases
- Magma → basalt solidification
- Temperature and composition mixing

### 3. Integration with Existing Systems

- Works with sub-position tracking system
- Integrates with elevation tracking
- Preserves plate IDs and properties
- Compatible with all visualization modes

## Usage

1. Build and run: `go build . && ./voxel_planet.exe`
2. Press **"8"** to activate Earth-like elevation visualization
3. Speed up with **Shift+3** (1000x) to see geological changes
4. Watch for:
   - Ocean trenches forming (dark blue)
   - Mountain ranges rising (grey → white peaks)
   - Volcanic islands emerging (basalt)
   - Continental collision zones

## Files Modified

- `/physics/voxel_advection.go`: Added `handleShellToShellMovement()` function
- `/core/voxel_types.go`: Added `MeltFraction` field
- `/rendering/opengl/shaders/renderer_gl_raymarch.go`: Updated color schemes

The rendering pipeline is standard - no special startup needed. Just press "8" for the Earth-like view!