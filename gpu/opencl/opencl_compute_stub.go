// +build darwin

package opencl

// Stub for OpenCLCompute on macOS
func NewOpenCLCompute(planet *core.VoxelPlanet) (GPUCompute, error) {
	// On macOS, we use Metal instead
	return NewMetalCompute(planet)
}