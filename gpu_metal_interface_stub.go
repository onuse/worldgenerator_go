// +build !darwin

package main

import "unsafe"

// GetVoxelBuffer stub for non-Darwin platforms
func (m *MetalCompute) GetVoxelBuffer() unsafe.Pointer {
	return nil
}

// GetShellBuffer stub for non-Darwin platforms  
func (m *MetalCompute) GetShellBuffer() unsafe.Pointer {
	return nil
}