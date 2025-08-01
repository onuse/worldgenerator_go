package main

import (
	"fmt"
	"math"
	"unsafe"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// VoxelTextureData manages voxel data as 3D textures for GPU access
type VoxelTextureData struct {
	MaterialTexture     uint32
	TemperatureTexture  uint32
	VelocityTexture     uint32
	ShellInfoTexture    uint32
	
	textureSize         int32
	maxShells          int32
}

// NewVoxelTextureData creates texture storage for voxel data
func NewVoxelTextureData(maxShells int) *VoxelTextureData {
	vtd := &VoxelTextureData{
		maxShells: int32(maxShells),
		textureSize: 512, // Balance between quality and performance
	}
	
	// Create textures
	gl.GenTextures(1, &vtd.MaterialTexture)
	gl.GenTextures(1, &vtd.TemperatureTexture)
	gl.GenTextures(1, &vtd.VelocityTexture)
	gl.GenTextures(1, &vtd.ShellInfoTexture)
	
	// Initialize material texture (2D texture array for shells)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.MaterialTexture)
	gl.TexImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.R32F, vtd.textureSize, vtd.textureSize, vtd.maxShells, 
		0, gl.RED, gl.FLOAT, nil)
	// Use nearest filtering for material texture to avoid interpolation between different materials
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	
	// Initialize temperature texture
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.TemperatureTexture)
	gl.TexImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.R32F, vtd.textureSize, vtd.textureSize, vtd.maxShells,
		0, gl.RED, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	
	// Initialize velocity texture (RG for theta/phi components)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.VelocityTexture)
	gl.TexImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.RG32F, vtd.textureSize, vtd.textureSize, vtd.maxShells,
		0, gl.RG, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	
	// Shell info texture (1D texture with shell metadata)
	gl.BindTexture(gl.TEXTURE_1D, vtd.ShellInfoTexture)
	gl.TexImage1D(gl.TEXTURE_1D, 0, gl.RGBA32F, vtd.maxShells, 0, gl.RGBA, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_1D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_1D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	
	return vtd
}

var debugOnce = true

// sampleVoxelAtLocation samples voxel data at a specific lat/lon position
// It handles the non-uniform longitude distribution by finding the correct voxel
func sampleVoxelAtLocation(shell *SphericalShell, lat, lon float64) VoxelMaterial {
	// Find the latitude band
	latBandF := (lat + 90.0) / 180.0 * float64(shell.LatBands)
	latBand := int(math.Floor(latBandF))
	
	// Clamp latitude band
	if latBand >= shell.LatBands {
		latBand = shell.LatBands - 1
	}
	if latBand < 0 {
		latBand = 0
	}
	
	// For the exact latitude band, find the appropriate longitude voxel
	// The key is to use the actual voxel count for this specific latitude band
	voxelCount := len(shell.Voxels[latBand])
	if voxelCount == 0 {
		return VoxelMaterial{Type: MatAir}
	}
	
	// Convert longitude to voxel index for this latitude band
	lonNorm := (lon + 180.0) / 360.0 // 0 to 1
	lonIndexF := lonNorm * float64(voxelCount)
	lonIndex := int(math.Floor(lonIndexF))
	
	// Wrap around
	lonIndex = lonIndex % voxelCount
	if lonIndex < 0 {
		lonIndex += voxelCount
	}
	
	// Return the voxel data
	return shell.Voxels[latBand][lonIndex]
}

// UpdateFromPlanet updates textures with planet voxel data
func (vtd *VoxelTextureData) UpdateFromPlanet(planet *VoxelPlanet) {
	// Prepare data arrays
	materialData := make([]float32, vtd.textureSize*vtd.textureSize)
	tempData := make([]float32, vtd.textureSize*vtd.textureSize)
	velData := make([]float32, vtd.textureSize*vtd.textureSize*2) // 2 components
	
	// Update each shell
	for shellIdx, shell := range planet.Shells {
		if shellIdx >= int(vtd.maxShells) {
			break
		}
		
		// Clear arrays
		for i := range materialData {
			materialData[i] = 0
			tempData[i] = 0
		}
		for i := range velData {
			velData[i] = 0
		}
		
		// Debug: count non-air voxels
		nonAirCount := 0
		
		// Fill texture by recreating the continent data at texture resolution
		// This avoids the mismatch between voxel grid and texture grid
		for texY := 0; texY < int(vtd.textureSize); texY++ {
			for texX := 0; texX < int(vtd.textureSize); texX++ {
				// Convert texture coordinates to spherical coordinates
				u := float64(texX) / float64(vtd.textureSize) // 0 to 1
				v := float64(texY) / float64(vtd.textureSize) // 0 to 1
				
				lon := u * 360.0 - 180.0 // -180 to 180
				lat := v * 180.0 - 90.0   // -90 to 90
				
				idx := texY*int(vtd.textureSize) + texX
				
				// For surface shell, recreate continent logic
				if shellIdx == len(planet.Shells)-2 {
					// Simple continent generation - must match voxel_planet.go logic
					isLand := false
					
					// Eurasia
					if lat > 20 && lat < 75 && lon > -10 && lon < 140 {
						isLand = true
					}
					// Africa  
					if lat > -35 && lat < 35 && lon > -20 && lon < 50 {
						isLand = true
					}
					// Americas
					if lon > -170 && lon < -30 {
						if lat > -55 && lat < 70 {
							isLand = true
						}
					}
					// Australia
					if lat > -40 && lat < -10 && lon > 110 && lon < 155 {
						isLand = true
					}
					
					if isLand {
						materialData[idx] = float32(MatGranite)
					} else {
						materialData[idx] = float32(MatWater)
					}
					tempData[idx] = 288.15 - float32(math.Abs(lat)*0.5)
					nonAirCount++
				} else {
					// For non-surface shells, sample from voxel data
					voxel := sampleVoxelAtLocation(&shell, lat, lon)
					materialData[idx] = float32(voxel.Type)
					if voxel.Type != MatAir {
						nonAirCount++
					}
					tempData[idx] = voxel.Temperature
					velData[idx*2] = voxel.VelTheta
					velData[idx*2+1] = voxel.VelPhi
				}
			}
		}
		
		// Debug output for surface shell (only once)
		if debugOnce && shellIdx == len(planet.Shells)-2 {
			fmt.Printf("Surface shell %d (r=%.0f-%.0f km): %d non-air voxels out of %d texture pixels\n", 
				shellIdx, shell.InnerRadius/1000, shell.OuterRadius/1000, nonAirCount, vtd.textureSize*vtd.textureSize)
			
			// Check material distribution
			matCounts := make(map[MaterialType]int)
			for i := 0; i < len(materialData); i++ {
				mat := MaterialType(materialData[i])
				matCounts[mat]++
			}
			fmt.Printf("Material distribution in texture: %+v\n", matCounts)
			
			// Also check voxel distribution
			voxelMatCounts := make(map[MaterialType]int)
			totalVoxels := 0
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					mat := shell.Voxels[latIdx][lonIdx].Type
					voxelMatCounts[mat]++
					totalVoxels++
				}
			}
			fmt.Printf("Voxel distribution in shell: %+v (total: %d)\n", voxelMatCounts, totalVoxels)
			
			// Debug: Check specific texture positions
			fmt.Printf("\nSampling debug at specific texture positions:\n")
			testPositions := []struct{x, y int; desc string}{
				{256, 256, "center"},
				{128, 256, "left"},
				{384, 256, "right"},
				{256, 128, "top"},
				{256, 384, "bottom"},
			}
			for _, pos := range testPositions {
				if pos.x < int(vtd.textureSize) && pos.y < int(vtd.textureSize) {
					idx := pos.y*int(vtd.textureSize) + pos.x
					mat := MaterialType(materialData[idx])
					u := float64(pos.x) / float64(vtd.textureSize)
					v := float64(pos.y) / float64(vtd.textureSize)
					lon := u * 360.0 - 180.0
					lat := v * 180.0 - 90.0
					fmt.Printf("  Tex(%d,%d) -> lat=%.1f lon=%.1f -> material=%v\n", 
						pos.x, pos.y, lat, lon, mat)
				}
			}
			
			debugOnce = false
		}
		
		// Upload to GPU
		gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.MaterialTexture)
		gl.TexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, int32(shellIdx), 
			vtd.textureSize, vtd.textureSize, 1,
			gl.RED, gl.FLOAT, unsafe.Pointer(&materialData[0]))
		
		gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.TemperatureTexture)
		gl.TexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, int32(shellIdx),
			vtd.textureSize, vtd.textureSize, 1,
			gl.RED, gl.FLOAT, unsafe.Pointer(&tempData[0]))
		
		gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.VelocityTexture)
		gl.TexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, int32(shellIdx),
			vtd.textureSize, vtd.textureSize, 1,
			gl.RG, gl.FLOAT, unsafe.Pointer(&velData[0]))
	}
	
	// Update shell info
	shellInfo := make([]float32, vtd.maxShells*4) // RGBA = inner radius, outer radius, lat bands, reserved
	for i, shell := range planet.Shells {
		if i >= int(vtd.maxShells) {
			break
		}
		shellInfo[i*4] = float32(shell.InnerRadius)
		shellInfo[i*4+1] = float32(shell.OuterRadius)
		shellInfo[i*4+2] = float32(shell.LatBands)
		shellInfo[i*4+3] = 0 // Reserved
	}
	
	gl.BindTexture(gl.TEXTURE_1D, vtd.ShellInfoTexture)
	gl.TexSubImage1D(gl.TEXTURE_1D, 0, 0, vtd.maxShells, gl.RGBA, gl.FLOAT, unsafe.Pointer(&shellInfo[0]))
	
	// Generate mipmaps for temperature and velocity textures only
	// Material texture uses nearest filtering so no mipmaps needed
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.TemperatureTexture)
	gl.GenerateMipmap(gl.TEXTURE_2D_ARRAY)
	
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.VelocityTexture)
	gl.GenerateMipmap(gl.TEXTURE_2D_ARRAY)
}

// Bind binds all textures to their texture units
func (vtd *VoxelTextureData) Bind() {
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.MaterialTexture)
	
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.TemperatureTexture)
	
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, vtd.VelocityTexture)
	
	gl.ActiveTexture(gl.TEXTURE3)
	gl.BindTexture(gl.TEXTURE_1D, vtd.ShellInfoTexture)
}

// Cleanup releases texture resources
func (vtd *VoxelTextureData) Cleanup() {
	gl.DeleteTextures(1, &vtd.MaterialTexture)
	gl.DeleteTextures(1, &vtd.TemperatureTexture)
	gl.DeleteTextures(1, &vtd.VelocityTexture)
	gl.DeleteTextures(1, &vtd.ShellInfoTexture)
}