package gpu

import (
	"worldgenerator/core"
)

// For now, we'll use a simpler approach without CGO struct issues
// The renderer will copy data from Metal buffers during each frame

// SharedGPUBuffers manages data transfer between compute and render
type SharedGPUBuffers struct {
	VoxelData []GPUVoxelMaterial
	ShellData []SphericalShellMetadata
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
func NewSharedGPUBuffers(planet *core.VoxelPlanet) *SharedGPUBuffers {
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
		VoxelData: make([]GPUVoxelMaterial, totalVoxels),
		ShellData: shellMeta,
	}
}

// UpdateFromPlanet copies voxel data from planet
func (s *SharedGPUBuffers) UpdateFromPlanet(planet *core.VoxelPlanet) {
	idx := 0
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if idx < len(s.VoxelData) {
					s.VoxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
					idx++
				}
			}
		}
	}
}