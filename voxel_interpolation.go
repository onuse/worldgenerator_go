package main

// InterpolateVoxelFromInnerShell samples voxel data from an inner shell using proper spherical interpolation
func InterpolateVoxelFromInnerShell(innerShell *SphericalShell, lat, lon float64) *VoxelMaterial {
	// Find the 4 nearest voxels in the inner shell
	latNorm := (lat + 90.0) / 180.0 // 0 to 1
	latIndexF := latNorm * float64(innerShell.LatBands)
	latIndex := int(latIndexF)
	latFrac := latIndexF - float64(latIndex)
	
	// Clamp latitude index
	if latIndex >= innerShell.LatBands-1 {
		latIndex = innerShell.LatBands - 2
		latFrac = 1.0
	}
	if latIndex < 0 {
		latIndex = 0
		latFrac = 0.0
	}
	
	// Sample two latitude bands
	result1 := interpolateLongitude(innerShell, latIndex, lon)
	result2 := interpolateLongitude(innerShell, latIndex+1, lon)
	
	// Interpolate between latitude bands
	return blendVoxels(result1, result2, float32(latFrac))
}

// interpolateLongitude interpolates voxel data at a specific longitude within a latitude band
func interpolateLongitude(shell *SphericalShell, latIndex int, lon float64) *VoxelMaterial {
	if latIndex >= len(shell.Voxels) {
		return &VoxelMaterial{Type: MatAir}
	}
	
	lonCount := len(shell.Voxels[latIndex])
	if lonCount == 0 {
		return &VoxelMaterial{Type: MatAir}
	}
	
	// Normalize longitude to 0-1
	lonNorm := (lon + 180.0) / 360.0
	lonIndexF := lonNorm * float64(lonCount)
	lonIndex := int(lonIndexF)
	lonFrac := lonIndexF - float64(lonIndex)
	
	// Handle wrapping
	lonIndex = lonIndex % lonCount
	lonIndexNext := (lonIndex + 1) % lonCount
	
	// Get the two voxels to interpolate between
	voxel1 := &shell.Voxels[latIndex][lonIndex]
	voxel2 := &shell.Voxels[latIndex][lonIndexNext]
	
	// Blend between them
	return blendVoxels(voxel1, voxel2, float32(lonFrac))
}

// blendVoxels blends two voxel materials based on a fraction
func blendVoxels(v1, v2 *VoxelMaterial, frac float32) *VoxelMaterial {
	// For material type, use nearest neighbor (don't blend material types)
	materialType := v1.Type
	if frac > 0.5 {
		materialType = v2.Type
	}
	
	// But we can store the blend factor for smoother rendering
	result := &VoxelMaterial{
		Type:        materialType,
		Density:     v1.Density*(1-frac) + v2.Density*frac,
		Temperature: v1.Temperature*(1-frac) + v2.Temperature*frac,
		Pressure:    v1.Pressure*(1-frac) + v2.Pressure*frac,
		VelTheta:    v1.VelTheta*(1-frac) + v2.VelTheta*frac,
		VelPhi:      v1.VelPhi*(1-frac) + v2.VelPhi*frac,
		VelR:        v1.VelR*(1-frac) + v2.VelR*frac,
		Age:         v1.Age*(1-frac) + v2.Age*frac,
		Stress:      v1.Stress*(1-frac) + v2.Stress*frac,
	}
	
	// Store the material blend for rendering (hack: use unused field)
	// This allows the renderer to show smooth transitions
	if v1.Type != v2.Type {
		// Store blend info in the VelR field (temporarily)
		result.VelR = float32(v1.Type) + frac
	}
	
	return result
}



// getLongitudeFromIndex returns the longitude in degrees for a given index
func getLongitudeFromIndex(lonIndex, totalLons int) float64 {
	return -180.0 + (float64(lonIndex)+0.5)*360.0/float64(totalLons)
}