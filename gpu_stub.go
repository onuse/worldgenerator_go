// +build !darwin

package main

import "fmt"

// MetalCompute stub for non-macOS platforms
type MetalCompute struct{}

func NewMetalCompute(planet *VoxelPlanet) (*MetalCompute, error) {
	return nil, fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) InitializeForPlanet(planet *VoxelPlanet) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) UpdateTemperature(dt float64) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) UpdateConvection(dt float64) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) UpdateAdvection(dt float64) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) uploadPlanetData(planet *VoxelPlanet) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) downloadPlanetData(planet *VoxelPlanet) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) Release() {}

// GPUCompute interface methods
func (mc *MetalCompute) RunTemperatureKernel(dt float32) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) RunConvectionKernel(dt float32) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) RunAdvectionKernel(dt float32) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) Cleanup() {
	// No-op for stub
}