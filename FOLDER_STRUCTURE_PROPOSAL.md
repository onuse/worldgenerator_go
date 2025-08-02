# Proposed Folder Structure

## Current Issues
- Too many files in root directory
- Mixed concerns (rendering, physics, GPU, planet data)
- Some folders exist but are underutilized

## Proposed Structure

```
worldgenerator_go/
├── cmd/                    # Application entry points
│   └── voxel_planet/
│       └── main.go
│
├── core/                   # Core voxel planet data structures
│   ├── planet.go          # VoxelPlanet struct and methods
│   ├── types.go           # Core types (VoxelMaterial, etc)
│   └── sphere_geometry.go # Spherical geometry helpers
│
├── physics/               # Physics simulation
│   ├── advection.go       # Fluid advection
│   ├── interpolation.go   # Field interpolation
│   ├── mechanics.go       # Material mechanics
│   ├── physics.go         # Main physics interface
│   ├── cpu/              # CPU implementations
│   │   └── physics_cpu.go
│   └── gpu/              # GPU physics
│       ├── physics_gpu.go
│       └── compute_plates.go
│
├── rendering/            # All rendering code
│   ├── opengl/          # OpenGL renderer
│   │   ├── renderer.go   # Main renderer
│   │   ├── shaders/     # Shader programs
│   │   │   ├── raymarch.go
│   │   │   ├── volume.go
│   │   │   └── ssbo.go
│   │   └── overlay/     # UI overlay
│   │       ├── fullscreen_overlay.go
│   │       ├── stats.go
│   │       └── bitmap_font.go
│   └── textures/
│       └── voxel_texture_data.go
│
├── gpu/                  # GPU abstraction layer
│   ├── interface.go      # GPUCompute interface
│   ├── buffer_manager.go # Shared buffer management
│   ├── metal/           # Metal implementation
│   │   ├── compute.go
│   │   ├── kernel_methods.go
│   │   └── metal.m
│   ├── opencl/          # OpenCL implementation
│   │   └── compute.go
│   └── cuda/            # CUDA implementation (future)
│       └── compute.go
│
├── simulation/          # High-level simulation
│   ├── plates.go        # Plate tectonics
│   └── evolution.go     # Planet evolution (future)
│
├── ui/                  # UI components (already exists)
│   ├── overlay.go
│   └── text_renderer.go
│
├── config/              # Configuration
│   ├── settings.go
│   └── settings.json
│
├── docs/                # Documentation
│   ├── DESIGN.md
│   ├── VOXEL_DESIGN.md
│   └── VOXEL_ROADMAP.md
│
├── scripts/             # Build scripts
│   ├── build.bat
│   ├── build.sh
│   └── build_fast.bat
│
└── archive/             # Legacy code (already exists)
```

## Benefits
1. Clear separation of concerns
2. Easy to find related code
3. Better for team collaboration
4. Easier to add new features
5. Cleaner root directory

## Migration Steps
1. Create new folder structure
2. Move files in logical groups
3. Update import paths
4. Test build
5. Update documentation