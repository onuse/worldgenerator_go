# Velocity Debug Status

## Issue
Pressing "2" shows deep blue planet - velocities are zero or near-zero.

## Root Causes Found

### 1. Convection Overwriting Plate Velocities (FIXED)
- **Location**: `physics/voxel_advection.go` lines 131-132
- **Problem**: Convection was setting VelNorth and VelEast, overwriting plate motion
- **Fix**: Commented out horizontal velocity assignments in convection

### 2. Velocity Decay (FIXED)  
- **Location**: `physics/voxel_advection.go` lines 137-138
- **Problem**: Convection was decaying all velocities including plate motion
- **Fix**: Only decay VelR (radial), preserve horizontal velocities

### 3. Initial Velocities ARE Set
- **Location**: `core/voxel_planet.go` lines 223, 258
- **Value**: 3e-9 m/s (≈ 9.5 cm/year)
- **Status**: Correctly initialized

## Next Steps to Debug

1. **Run simulation and check console output**
   - Added velocity debug output in texture update
   - Should see: "Velocities: X voxels with velocity, max=Y m/s"

2. **Test plate visualization (mode 4)**
   - Should show different colored plates
   - If working, plates are identified

3. **Check if plate angular velocities build up**
   - Plate system might need initial angular velocities
   - Currently starts at 0 and builds from forces

## How to Test

1. Build and run: `go build . && ./voxel_planet`
2. Watch console for velocity output in first 5 updates
3. Press "2" to check velocity visualization
4. Press "4" to check plate visualization

## Expected Behavior

- Initial velocities: ~3e-9 m/s (9.5 cm/year)
- Should see yellow/orange colors in velocity mode
- Continents should drift eastward slowly