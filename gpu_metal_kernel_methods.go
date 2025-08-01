// +build darwin

package main

// RunAdvectionKernel implements the GPUCompute interface method
func (mc *MetalCompute) RunAdvectionKernel(dt float32) error {
	return mc.UpdateAdvection(float64(dt))
}

// RunConvectionKernel implements the GPUCompute interface method
func (mc *MetalCompute) RunConvectionKernel(dt float32) error {
	return mc.UpdateConvection(float64(dt))
}

// RunTemperatureKernel implements the GPUCompute interface method
func (mc *MetalCompute) RunTemperatureKernel(dt float32) error {
	return mc.UpdateTemperature(float64(dt))
}