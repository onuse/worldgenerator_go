// +build darwin

package metal

// Cleanup releases Metal resources
func (mc *MetalCompute) Cleanup() {
	mc.Release()
}