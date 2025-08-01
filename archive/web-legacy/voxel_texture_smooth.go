package main

import (
	"math"
)

// sampleVoxelSmooth samples voxel data with bilinear interpolation
func sampleVoxelSmooth(shell *SphericalShell, lat, lon float64) (materialType MaterialType, height float32, age float32, velPhi float32) {
	// Convert lat/lon to continuous voxel coordinates
	latCoord := (lat + 90.0) * float64(shell.LatBands) / 180.0
	
	// Get integer indices and fractional parts
	latIdx0 := int(math.Floor(latCoord))
	latIdx1 := latIdx0 + 1
	latFrac := latCoord - float64(latIdx0)
	
	// Clamp latitude indices
	if latIdx0 < 0 { latIdx0 = 0 }
	if latIdx0 >= shell.LatBands { latIdx0 = shell.LatBands - 1 }
	if latIdx1 < 0 { latIdx1 = 0 }
	if latIdx1 >= shell.LatBands { latIdx1 = shell.LatBands - 1 }
	
	// Sample at both latitude bands
	mat0, h0, a0, v0 := sampleLatBand(shell, latIdx0, lon)
	mat1, h1, a1, v1 := sampleLatBand(shell, latIdx1, lon)
	
	// For material type, use nearest neighbor (can't interpolate discrete values)
	if latFrac < 0.5 {
		materialType = mat0
	} else {
		materialType = mat1
	}
	
	// Interpolate continuous values
	height = float32(h0*(1-latFrac) + h1*latFrac)
	age = float32(a0*(1-latFrac) + a1*latFrac)
	velPhi = float32(v0*(1-latFrac) + v1*latFrac)
	
	return
}

// sampleLatBand samples within a latitude band with longitude interpolation
func sampleLatBand(shell *SphericalShell, latIdx int, lon float64) (MaterialType, float64, float64, float64) {
	lonCount := shell.LonCounts[latIdx]
	lonCoord := (lon + 180.0) * float64(lonCount) / 360.0
	
	// Get integer indices and fractional part
	lonIdx0 := int(math.Floor(lonCoord))
	lonIdx1 := lonIdx0 + 1
	lonFrac := lonCoord - float64(lonIdx0)
	
	// Wrap longitude
	lonIdx0 = ((lonIdx0 % lonCount) + lonCount) % lonCount
	lonIdx1 = ((lonIdx1 % lonCount) + lonCount) % lonCount
	
	// Get voxels
	voxel0 := &shell.Voxels[latIdx][lonIdx0]
	voxel1 := &shell.Voxels[latIdx][lonIdx1]
	
	// Material type - use most common in neighborhood
	var materialType MaterialType
	if lonFrac < 0.5 {
		materialType = voxel0.Type
	} else {
		materialType = voxel1.Type
	}
	
	// Interpolate continuous values
	height := 0.5 // Default
	if voxel0.Type == MatGranite {
		height = 0.52
	} else if voxel0.Type == MatBasalt {
		height = 0.48
	}
	
	age := float64(voxel0.Age)*(1-lonFrac) + float64(voxel1.Age)*lonFrac
	velPhi := float64(voxel0.VelPhi)*(1-lonFrac) + float64(voxel1.VelPhi)*lonFrac
	
	return materialType, height, age, velPhi
}