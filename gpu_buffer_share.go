package main

// For now, we'll use a simpler approach without CGO struct issues
// The renderer will copy data from Metal buffers during each frame

// SharedGPUBuffers manages data transfer between compute and render
type SharedGPUBuffers struct {
	voxelData []GPUVoxelMaterial
	shellData []SphericalShellMetadata
}

// SphericalShellMetadata is a simplified version for GPU transfer
type SphericalShellMetadata struct {
	InnerRadius  float32
	OuterRadius  float32
	LatBands     int32
	VoxelOffset  int32
}

// ExtendedShellMetadata includes longitude counts for proper voxel indexing
type ExtendedShellMetadata struct {
	InnerRadius  float32
	OuterRadius  float32
	LatBands     int32
	VoxelOffset  int32
	LonCounts    [360]int32  // Max 360 latitude bands
}

// NewSharedGPUBuffers creates a buffer manager
func NewSharedGPUBuffers(planet *VoxelPlanet) *SharedGPUBuffers {
	// Count total voxels
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
	}
	
	// Create metadata
	shellMeta := make([]SphericalShellMetadata, len(planet.Shells))
	voxelOffset := 0
	
	for i, shell := range planet.Shells {
		shellMeta[i] = SphericalShellMetadata{
			InnerRadius: float32(shell.InnerRadius),
			OuterRadius: float32(shell.OuterRadius),
			LatBands:    int32(shell.LatBands),
			VoxelOffset: int32(voxelOffset),
		}
		
		// Calculate voxels in this shell
		shellVoxels := 0
		for _, count := range shell.LonCounts {
			shellVoxels += count
		}
		voxelOffset += shellVoxels
	}
	
	return &SharedGPUBuffers{
		voxelData: make([]GPUVoxelMaterial, totalVoxels),
		shellData: shellMeta,
	}
}

// UpdateFromPlanet copies voxel data from planet
func (s *SharedGPUBuffers) UpdateFromPlanet(planet *VoxelPlanet) {
	idx := 0
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if idx < len(s.voxelData) {
					s.voxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
					idx++
				}
			}
		}
	}
}