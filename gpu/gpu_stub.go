//go:build !darwin
// +build !darwin

package gpu

import (
	"fmt"
	"worldgenerator/core"
)

// MetalCompute stub for non-macOS platforms
type MetalCompute struct{}

func NewMetalCompute(planet *core.VoxelPlanet) (*MetalCompute, error) {
	return nil, fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) InitializeForPlanet(planet *core.VoxelPlanet) error {
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

func (mc *MetalCompute) uploadPlanetData(planet *core.VoxelPlanet) error {
	return fmt.Errorf("Metal GPU acceleration is only available on macOS")
}

func (mc *MetalCompute) downloadPlanetData(planet *core.VoxelPlanet) error {
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
