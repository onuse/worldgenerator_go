// +build darwin

package main

import "unsafe"

// GetVoxelBuffer returns the Metal voxel buffer pointer for OpenGL sharing
func (m *MetalCompute) GetVoxelBuffer() unsafe.Pointer {
	if m.ctx != nil {
		return m.voxelBuffer
	}
	return nil
}

// GetShellBuffer returns the Metal shell buffer pointer for OpenGL sharing
func (m *MetalCompute) GetShellBuffer() unsafe.Pointer {
	if m.ctx != nil {
		return m.shellBuffer
	}
	return nil
}