# Elevation and Collision Features

## Current Implementation

### 1. **Elevation Tracking**
- Added `Elevation` field to VoxelMaterial struct (in meters above/below mean radius)
- Elevation changes based on vertical velocity (`VelR`)
- Initial elevations set during planet generation:
  - Mountains: 1500-3500m (based on noise patterns)
  - Highlands: 700-1300m
  - Lowlands: 400-600m
  - Ocean depths: -1000 to -4000m

### 2. **Collision Detection**
- Different behaviors for different collision types:
  - **Oceanic-Continental**: Oceanic crust gets downward velocity (-0.001 m/s)
  - **Continental-Continental**: Both get upward velocity (0.0005 m/s) when stress > 5e7 Pa
- Stress accumulation tracks collision intensity
- Compression factors track plate deformation

### 3. **Elevation Visualization (Mode 8)**
Press "8" to activate elevation visualization with color coding:
- **Deep Blue**: Ocean trenches (< -4000m)
- **Cyan**: Shallow ocean (-4000 to -1000m)
- **Green**: Lowlands (-1000 to 1000m)
- **Light Green**: Hills (0 to 1000m)
- **Yellow**: Highlands (1000 to 3000m)
- **Orange/Red**: Mountains (3000 to 6000m)
- **White**: Extreme peaks (> 6000m)

### 4. **Vertical Movement**
- Vertical velocities accumulate elevation changes over time
- `SubPosR` tracks position within shell for future shell-to-shell movement
- Foundation laid for subduction and mountain building

## What's Working
- ✅ Elevation field tracking
- ✅ Initial elevation patterns
- ✅ Collision detection with stress
- ✅ Vertical velocity assignment
- ✅ Elevation visualization mode
- ✅ Elevation accumulation from velocity

## What's Still Needed
- ❌ Actual shell-to-shell movement (voxels changing shells)
- ❌ Mass conservation during vertical movement
- ❌ Material transformation (e.g., basalt → magma when subducting)
- ❌ Proper mountain range formation at collision zones
- ❌ Ocean trench formation at subduction zones

## Testing
1. Run `voxel_planet.exe`
2. Press "8" to switch to elevation visualization
3. Speed up time with Shift+3 (1000x) to see elevation changes
4. Look for:
   - Green lowlands on continents
   - Yellow/red mountain ranges
   - Blue ocean depths
   - Changes at collision zones over time