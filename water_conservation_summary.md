# Water Conservation System

## Overview

Implemented a global water volume tracking system that maintains constant water volume on the planet. This creates realistic sea level changes as continents move, mountains rise, and ocean basins deepen.

## Key Features

### 1. **Volume Tracking**
- `TotalWaterVolume`: Stores the planet's total water volume in m³
- `SeaLevel`: Current global sea level elevation (meters above/below reference)
- Calculated considering spherical geometry and actual voxel volumes

### 2. **Dynamic Sea Level**
- Binary search algorithm finds the sea level that maintains constant water volume
- Automatically adjusts when:
  - Mountains rise (sea level drops slightly)
  - Ocean basins deepen (sea level drops)
  - Land subsides (sea level rises)
  - Continents collide and thicken (sea level rises)

### 3. **Sea Level Effects**
- **Flooding**: Land below new sea level becomes ocean
- **Exposure**: Ocean floor above new sea level becomes exposed sediment
- **Coastal Changes**: Dynamic coastlines based on elevation vs sea level

### 4. **Conservation Benefits**
- **Realistic Transgressions**: Sea can advance inland during subsidence
- **Realistic Regressions**: Sea retreats when basins deepen or mountains rise
- **Ice Ages**: When implemented, ice formation will lower sea level
- **Lake Formation**: Isolated basins can have their own water levels
- **Accurate Coastal Evolution**: Coastlines change naturally with tectonics

## Implementation Details

### Volume Calculation
```go
// Considers spherical geometry
volume = r²Δr × ΔθΔφ × cos(latitude)
// Adjusted for actual water depth vs shell thickness
```

### Sea Level Algorithm
1. Calculate current water volume at given sea level
2. Binary search to find level that matches target volume
3. Apply changes to flood/expose land accordingly

### Integration Points
- Called after plate movement in `advectSurfacePlates()`
- Updates after shell-to-shell movement
- Applies after coastal erosion

## Future Enhancements

1. **Regional Water Bodies**
   - Track separate volumes for isolated lakes
   - Different water levels for enclosed seas
   - Endorheic basins that can dry up

2. **Water Cycle**
   - Evaporation and precipitation
   - Ice cap formation/melting affects sea level
   - River systems that transport sediment

3. **Climate Effects**
   - Temperature-dependent ice volume
   - Thermal expansion of oceans
   - Realistic ice age cycles

## Usage

The system runs automatically during simulation. Key outputs:
- Console reports sea level changes > 10m
- Elevation mode (8) shows flooding/exposure
- Coastlines dynamically adjust

## Physical Realism

This conservation system adds significant realism:
- Mountain building causes slight sea level rise (displaced water)
- Subduction creates deep trenches, lowering sea level
- Continental collision thickens crust, raising sea level
- No water is created or destroyed - only redistributed

This creates emergent behaviors like:
- Shallow seas flooding continents during subsidence
- Land bridges appearing during low sea level
- Realistic continental shelf exposure
- Natural coastal evolution