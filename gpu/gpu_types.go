package gpu

import (
	"worldgenerator/core"
)

// GPUVoxelMaterial is a GPU-compatible version of VoxelMaterial
type GPUVoxelMaterial struct {
	Type        uint32
	Density     float32
	Temperature float32
	Pressure    float32
	VelNorth    float32
	VelEast     float32
	VelR        float32
	Age         float32
	PlateID     int32    // Which plate this voxel belongs to
	IsBoundary  int32    // 1 if on plate boundary, 0 otherwise
	_padding    [2]int32 // Ensure 16-byte alignment
}

// ConvertToGPUVoxel converts a VoxelMaterial to GPU format
func ConvertToGPUVoxel(v *core.VoxelMaterial) GPUVoxelMaterial {
	return GPUVoxelMaterial{
		Type:        uint32(v.Type),
		Density:     v.Density,
		Temperature: v.Temperature,
		Pressure:    v.Pressure,
		VelNorth:    v.VelNorth,
		VelEast:     v.VelEast,
		VelR:        v.VelR,
		Age:         v.Age,
	}
}
