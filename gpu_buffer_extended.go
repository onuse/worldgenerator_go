package main

import (
	"unsafe"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// ExtendedBufferManager manages GPU buffers with full voxel indexing information
type ExtendedBufferManager struct {
	voxelSSBO       uint32
	shellMetaSSBO   uint32
	lonCountSSBO    uint32  // Per-shell longitude counts for each latitude band
	
	totalVoxels     int
	shellCount      int
}

// ShellMetadataGPU matches the GPU struct layout
type ShellMetadataGPU struct {
	InnerRadius    float32
	OuterRadius    float32
	LatBands       int32
	VoxelOffset    int32
	LonCountOffset int32  // Offset into longitude count array
	_padding       [3]int32 // Ensure 16-byte alignment
}

// CreateExtendedBuffers creates SSBOs with full indexing information
func (r *VoxelRenderer) CreateExtendedBuffers(planet *VoxelPlanet) {
	// Count total voxels and longitude counts
	totalVoxels := 0
	totalLonCounts := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
		totalLonCounts += len(shell.LonCounts)
	}
	
	// Prepare shell metadata
	shellMeta := make([]ShellMetadataGPU, len(planet.Shells))
	lonCounts := make([]int32, totalLonCounts)
	
	voxelOffset := 0
	lonCountOffset := 0
	
	for i, shell := range planet.Shells {
		shellMeta[i] = ShellMetadataGPU{
			InnerRadius:    float32(shell.InnerRadius),
			OuterRadius:    float32(shell.OuterRadius),
			LatBands:       int32(shell.LatBands),
			VoxelOffset:    int32(voxelOffset),
			LonCountOffset: int32(lonCountOffset),
		}
		
		// Copy longitude counts
		for j, count := range shell.LonCounts {
			lonCounts[lonCountOffset+j] = int32(count)
			voxelOffset += count
		}
		lonCountOffset += len(shell.LonCounts)
	}
	
	// Create voxel data SSBO
	if r.voxelSSBO == 0 {
		gl.GenBuffers(1, &r.voxelSSBO)
	}
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
	voxelSize := totalVoxels * int(unsafe.Sizeof(GPUVoxelMaterial{}))
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, gl.DYNAMIC_DRAW)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, r.voxelSSBO)
	
	// Create shell metadata SSBO
	if r.shellSSBO == 0 {
		gl.GenBuffers(1, &r.shellSSBO)
	}
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.shellSSBO)
	shellSize := len(shellMeta) * int(unsafe.Sizeof(ShellMetadataGPU{}))
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, shellSize, unsafe.Pointer(&shellMeta[0]), gl.STATIC_DRAW)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, r.shellSSBO)
	
	// Create longitude count SSBO
	var lonCountSSBO uint32
	gl.GenBuffers(1, &lonCountSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, lonCountSSBO)
	lonCountSize := len(lonCounts) * 4 // int32 = 4 bytes
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, lonCountSize, unsafe.Pointer(&lonCounts[0]), gl.STATIC_DRAW)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 2, lonCountSSBO)
	
	// Store for cleanup
	r.lonCountSSBO = lonCountSSBO
}

// UpdateExtendedBuffers updates voxel data in the SSBO
func (r *VoxelRenderer) UpdateExtendedBuffers(planet *VoxelPlanet) {
	// Convert voxels to GPU format
	var voxelData []GPUVoxelMaterial
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxelData = append(voxelData, ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx]))
			}
		}
	}
	
	// Update SSBO
	if len(voxelData) > 0 {
		gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
		voxelSize := len(voxelData) * int(unsafe.Sizeof(GPUVoxelMaterial{}))
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, voxelSize, unsafe.Pointer(&voxelData[0]))
	}
}
