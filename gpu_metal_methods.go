// +build darwin

package main

// Cleanup releases Metal resources
func (mc *MetalCompute) Cleanup() {
	mc.Release()
}