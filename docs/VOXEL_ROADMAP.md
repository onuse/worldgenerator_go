# Voxel Planet Evolution - Development Roadmap

## Overview
Native voxel-based planet simulator with GPU-accelerated physics and direct volume rendering, enabling true 3D geological processes at planetary scale. Uses enhanced grid system with selective virtual voxels for complex features.

## Current Status
- ✅ Voxel physics simulation running on GPU (Metal/Compute Shaders)
- ✅ Native OpenGL 4.3 renderer with planet visualization
- ✅ Virtual voxel system prototyped and tested
- ✅ Surface and volume rendering implemented
- ✅ GPU compute shaders for physics (OpenGL 4.3)
- ✅ Zero-copy GPU buffer sharing (persistent mapped buffers)
- 🚧 Implementing enhanced grid with proper plate tectonics

## Architecture Decision
After extensive prototyping, we've chosen a **hybrid architecture**:
- **Enhanced Grid System** as the primary simulation layer (performance, compatibility)
- **Selective Virtual Voxels** for complex features (earthquakes, volcanoes, visible deformation)
- **Sub-cell positioning** to eliminate grid movement artifacts
- **Temporal resolution splitting** for optimal performance

## Development Phases

### Phase 1: Enhanced Grid System 🚧 IMMEDIATE PRIORITY

#### 1.1 Sub-cell Movement System
- [ ] Add sub-position tracking to VoxelMaterial struct
- [ ] Implement smooth movement accumulation
- [ ] Handle cell boundary transitions without snapping
- [ ] Maintain material continuity during transitions
- [ ] Test with existing plate tectonics system

#### 1.2 Vertical Layer Transitions
- [ ] Implement shell-to-shell movement for subduction/rising
- [ ] Add vertical velocity component to plate motion
- [ ] Handle material transformations during vertical movement
  - [ ] Continental crust → Magma (when subducting deep)
  - [ ] Magma → Oceanic crust (when rising to surface)
  - [ ] Pressure/temperature based phase changes
- [ ] Maintain mass conservation during transitions

#### 1.3 Temporal Resolution Optimization
- [ ] Separate update frequencies for different systems:
  - [ ] Rendering: 60 FPS (always smooth)
  - [ ] Physics integration: 30 FPS (critical calculations)
  - [ ] Plate movement: 10 FPS (slow process)
  - [ ] Geological processes: 1 FPS (very slow)
- [ ] Implement interpolation between physics frames
- [ ] Add configurable time scales for each system

### Phase 2: Plate Boundary Detection & Classification

#### 2.1 Boundary Detection System
- [ ] Implement efficient neighbor checking for plate IDs
- [ ] Calculate relative velocities between plates
- [ ] Classify boundary types:
  - [ ] Divergent (spreading apart)
  - [ ] Convergent (colliding)
  - [ ] Transform (sliding past)
- [ ] Track boundary stress accumulation

#### 2.2 Boundary-Specific Behaviors
- [ ] **Divergent Boundaries**:
  - [ ] Create new oceanic crust
  - [ ] Implement seafloor spreading
  - [ ] Add mid-ocean ridge volcanism
- [ ] **Convergent Boundaries**:
  - [ ] Oceanic-Continental subduction
  - [ ] Continental-Continental collision (mountain building)
  - [ ] Volcanic arc formation above subduction zones
- [ ] **Transform Boundaries**:
  - [ ] Lateral movement without creation/destruction
  - [ ] Stress accumulation for earthquakes

### Phase 3: Selective Virtual Voxel Integration

#### 3.1 Feature Detection for Virtual Voxels
- [ ] Identify where virtual voxels add value:
  - [ ] High-stress plate boundaries
  - [ ] Active volcanic regions
  - [ ] Visible mountain peaks
  - [ ] Areas near camera (LOD based)
- [ ] Implement conversion thresholds
- [ ] Add hysteresis to prevent oscillation

#### 3.2 Virtual Voxel Zones
- [ ] Create specialized virtual voxel systems:
  - [ ] `VirtualFaultSystem` for earthquake zones
  - [ ] `VirtualVolcanoSystem` for eruptions
  - [ ] `VirtualMountainSystem` for visible peaks
- [ ] Each system optimized for its specific physics
- [ ] Efficient GPU kernels for each type

#### 3.3 Grid-Virtual Interface
- [ ] Seamless data exchange between grid and virtual
- [ ] Virtual voxels read grid state as boundary conditions
- [ ] Grid incorporates virtual voxel results
- [ ] No visible seams or transitions

### Phase 4: Advanced Geological Features

#### 4.1 Volcanic System
- [ ] Magma chamber pressure tracking
- [ ] Eruption triggering based on pressure/stress
- [ ] Virtual voxel particle system for lava
- [ ] Ash cloud generation (grid-based atmosphere)
- [ ] New land formation from cooled lava

#### 4.2 Earthquake System
- [ ] Stress accumulation along faults
- [ ] Sudden release mechanics
- [ ] Virtual voxel spring breaking for realistic motion
- [ ] Seismic wave propagation through grid
- [ ] Surface deformation and scarring

#### 4.3 Erosion & Sedimentation
- [ ] Height-based erosion on grid
- [ ] River formation using flow accumulation
- [ ] Sediment transport and deposition
- [ ] Delta formation at river mouths
- [ ] Integration with plate movement

#### 4.4 Climate & Atmosphere
- [ ] Grid-based atmospheric simulation
- [ ] Temperature distribution
- [ ] Precipitation patterns
- [ ] Cloud formation and movement
- [ ] Ice cap growth/retreat
- [ ] Weathering effects on rock

### Phase 5: Performance & Optimization

#### 5.1 LOD System
- [ ] Distance-based detail reduction
- [ ] Fewer physics updates for distant regions
- [ ] Simplified rendering for far areas
- [ ] Smooth LOD transitions

#### 5.2 GPU Optimization
- [ ] Optimize compute shader work groups
- [ ] Implement GPU-based culling
- [ ] Reduce memory bandwidth usage
- [ ] Profile and eliminate bottlenecks

#### 5.3 Memory Management
- [ ] Efficient buffer allocation
- [ ] Streaming for very large planets
- [ ] Compress inactive regions
- [ ] Smart caching strategies

### Phase 6: User Interface & Visualization

#### 6.1 Visualization Modes
- [ ] Enhanced material view with textures
- [ ] Temperature with heat flow arrows
- [ ] Stress visualization for tectonics
- [ ] Age-based coloring for crust
- [ ] Velocity field streamlines

#### 6.2 Analysis Tools
- [ ] Cross-section views at any angle
- [ ] Time-lapse recording
- [ ] Statistical overlays
- [ ] Measurement tools

#### 6.3 Simulation Control
- [ ] Time speed control with interpolation
- [ ] Save/load simulation states
- [ ] Parameter tuning interface
- [ ] Preset scenarios

## Implementation Priority

### Week 1: Enhanced Grid Foundation
1. Sub-cell positioning system
2. Smooth movement interpolation
3. Cell transition handling
4. Basic vertical movement

### Week 2: Plate Dynamics
1. Boundary detection
2. Subduction mechanics
3. Seafloor spreading
4. Mountain building basics

### Week 3: Virtual Voxel Zones
1. Fault line detection
2. Selective virtual voxel creation
3. Basic earthquake mechanics
4. Volcanic vent system

### Week 4: Integration & Polish
1. Performance optimization
2. Visual improvements
3. UI controls
4. Testing & debugging

## Success Metrics
- 15+ FPS with full geological simulation
- Smooth continental drift without artifacts
- Realistic plate interactions
- Stable performance as features are added
- Clean, maintainable code architecture

## Architecture Benefits
- **Performance**: Grid handles 99% efficiently
- **Quality**: Virtual voxels for complex deformation
- **Scalability**: Easy to add new features
- **Maintainability**: Clear separation of concerns
- **Flexibility**: Can adjust virtual/grid ratio as needed

## Next Immediate Steps
1. Create enhanced grid voxel structure with sub-positions
2. Implement smooth movement accumulation
3. Add vertical shell transitions
4. Test with existing plate system
5. Profile performance improvements