// +build darwin

package main

// Stub for OpenCLCompute on macOS
func NewOpenCLCompute(planet *VoxelPlanet) (GPUCompute, error) {
	// On macOS, we use Metal instead
	return NewMetalCompute(planet)
}