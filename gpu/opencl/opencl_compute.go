//go:build !darwin
// +build !darwin

package opencl

import (
	"fmt"
	"worldgenerator/core"
	"worldgenerator/gpu"
)

// OpenCLCompute placeholder for non-Darwin platforms
type OpenCLCompute struct{}

func NewOpenCLCompute(planet *core.VoxelPlanet) (gpu.GPUCompute, error) {
	return &OpenCLCompute{}, fmt.Errorf("OpenCL compute not yet implemented")
}

func (o *OpenCLCompute) RunTemperatureKernel(dt float32) error {
	return fmt.Errorf("OpenCL not implemented")
}

func (o *OpenCLCompute) RunConvectionKernel(dt float32) error {
	return fmt.Errorf("OpenCL not implemented")
}

func (o *OpenCLCompute) RunAdvectionKernel(dt float32) error {
	return fmt.Errorf("OpenCL not implemented")
}

func (o *OpenCLCompute) Cleanup() {}
