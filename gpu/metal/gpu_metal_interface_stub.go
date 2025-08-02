//go:build !darwin
// +build !darwin

package metal

import (
	"unsafe"
	"worldgenerator/gpu"
)

// MetalCompute is a local type alias for gpu.MetalCompute to allow method definitions.
type MetalCompute gpu.MetalCompute

// GetVoxelBuffer stub for non-Darwin platforms
func (m *MetalCompute) GetVoxelBuffer() unsafe.Pointer {
	return nil
}

// GetShellBuffer stub for non-Darwin platforms
func (m *MetalCompute) GetShellBuffer() unsafe.Pointer {
	return nil
}
