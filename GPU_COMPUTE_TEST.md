# GPU Compute Test Instructions

## Current Status
- GPU compute shaders are implemented but not fully integrated
- The code compiles successfully
- Threading issues prevent full GPU physics

## How to Test

### 1. Run with GPU Compute Flag
```bash
voxel_planet.exe -gpu compute
```

### 2. Expected Output
Look for these messages in console:
- "✅ Using GPU compute shaders for physics" - If compute shaders initialized
- "⚠️ GPU plate tectonics not yet integrated" - Expected warning
- "✅ Physics engine using GPU compute shaders" - If physics is using GPU

### 3. Check GPU Usage
- Open Task Manager > Performance > GPU
- Look for "Compute_0" or "Compute_1" usage
- Currently may show 0% due to threading issues

## Known Issues

### Threading Problem
The compute shaders need GPU buffers (SSBOs) to be bound, but:
1. Physics runs in a background thread
2. GPU buffers are managed in the main/render thread
3. OpenGL contexts can't be shared between threads easily

### Current Limitations
- Temperature diffusion shader exists but may not access data correctly
- Convection shader exists but velocities are overridden by CPU code
- Advection shader is not implemented

## Next Steps

To fully enable GPU compute:
1. Move physics back to main thread OR
2. Create a command queue system OR
3. Use persistent mapped buffers with proper synchronization

## Quick CPU vs GPU Test

1. Run with CPU: `voxel_planet.exe -gpu cpu`
2. Note the FPS and physics time
3. Run with compute: `voxel_planet.exe -gpu compute`
4. Compare performance (may be similar due to threading issues)