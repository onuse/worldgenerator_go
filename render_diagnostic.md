# Render Mode Diagnostic

## Issue
When pressing keys 1-8, only the atmosphere color changes (sometimes red, sometimes not), but elevation and material variations don't show.

## Expected Behavior
- Mode 1: Blue oceans, green continents
- Mode 8: Colorful topographic map (blue ocean depths → green lowlands → yellow/brown mountains → white peaks)

## Diagnostic Steps Added

1. **Render Mode Check**: Added debug output to verify renderMode uniform is found in shader
2. **Texture Check**: Added warning if voxelTextures is nil

## Potential Causes

1. **Texture Data Issue**
   - Elevation data IS being written (line 165: `tempData[idx*2+1] = voxel.Elevation`)
   - Initial elevations ARE varied (mountains 1.5-3.5km, oceans -1 to -4km)

2. **Shader Sampling Issue**
   - The shader might not be reading the texture data correctly
   - The texture coordinates might be wrong

3. **Texture Update Timing**
   - Textures might not be updated before first render
   - Call to `UpdateVoxelTextures()` might be missing or happening too late

## What to Check When Running

1. Look for console output:
   - "WARNING: renderMode uniform not found in shader!" 
   - "WARNING: voxelTextures is nil!"
   - "Switched to elevation visualization" when pressing 8

2. If you see material distribution debug output, check if it shows variety:
   - Should show Water, Granite, Air in different amounts
   - Not just all Air or all one type

## Quick Fix to Try

In main.go, ensure `UpdateVoxelTextures()` is called AFTER planet creation and BEFORE first render:
```go
// Initialize voxel textures
renderer.UpdateVoxelTextures(planet)
```

This should already be there around line 188, but verify it's being called.

## If Still Not Working

The issue might be:
1. OpenGL context/state issue
2. Shader compilation using wrong version
3. Texture format mismatch

Try adding after texture update:
```go
if err := gl.GetError(); err != gl.NO_ERROR {
    fmt.Printf("OpenGL error after texture update: 0x%x\n", err)
}
```