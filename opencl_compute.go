// +build !darwin

package main

import "fmt"

// OpenCLCompute placeholder for non-Darwin platforms
type OpenCLCompute struct{}

func NewOpenCLCompute(planet *VoxelPlanet) (GPUCompute, error) {
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