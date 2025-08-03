# GPU Compute Status

## Current Situation
- **Zero GPU utilization** because default mode is `cpu`
- Physics runs on CPU by default

## Available GPU Backends

### 1. CPU (Default)
- **Status**: Working
- **Usage**: `./voxel_planet` or `./voxel_planet -gpu cpu`
- **Performance**: Slow but functional

### 2. Metal (macOS only)
- **Status**: Implemented
- **Usage**: `./voxel_planet -gpu metal`
- **Requirement**: macOS system

### 3. OpenCL
- **Status**: NOT IMPLEMENTED (stub only)
- **Usage**: Would be `./voxel_planet -gpu opencl`
- **Current**: Returns error "OpenCL compute not yet implemented"

### 4. CUDA
- **Status**: NOT IMPLEMENTED
- **Usage**: Would be `./voxel_planet -gpu cuda`
- **Current**: Fatal error "CUDA support not yet implemented"

### 5. Compute Shaders
- **Status**: Unclear - might be partially implemented
- **Usage**: `./voxel_planet -gpu compute`
- **Note**: Falls back to CPU compute

## Recommendations

### For Windows Users
Currently only CPU mode works. To get GPU acceleration:
1. OpenCL implementation would be most universal
2. CUDA would work for NVIDIA GPUs
3. Compute shaders (OpenGL 4.3) might already work

### For macOS Users
Use Metal: `./voxel_planet -gpu metal`

### Quick Test
Try: `./voxel_planet -gpu compute`
- If it works, you'll see GPU usage
- If not, it falls back to CPU

## Why This Matters
- CPU physics is SLOW for a full planet
- GPU would enable:
  - Real-time plate tectonics
  - Larger planet resolution
  - More complex physics
  - Smoother framerates

## Next Steps
1. Test if compute shaders work (`-gpu compute`)
2. If not, OpenCL implementation would be most valuable
3. The rendering is already on GPU (OpenGL)
4. Only physics needs GPU acceleration