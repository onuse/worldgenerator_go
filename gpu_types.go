package main

// GPUVoxelMaterial is a GPU-compatible version of VoxelMaterial
type GPUVoxelMaterial struct {
	Type        uint32
	Density     float32
	Temperature float32
	Pressure    float32
	VelTheta    float32
	VelPhi      float32
	VelR        float32
	Age         float32
}

// ConvertToGPUVoxel converts a VoxelMaterial to GPU format
func ConvertToGPUVoxel(v *VoxelMaterial) GPUVoxelMaterial {
	return GPUVoxelMaterial{
		Type:        uint32(v.Type),
		Density:     v.Density,
		Temperature: v.Temperature,
		Pressure:    v.Pressure,
		VelTheta:    v.VelTheta,
		VelPhi:      v.VelPhi,
		VelR:        v.VelR,
		Age:         v.Age,
	}
}