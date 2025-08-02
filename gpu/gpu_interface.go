package gpu

// GPUCompute interface for different GPU backends
type GPUCompute interface {
	RunTemperatureKernel(dt float32) error
	RunConvectionKernel(dt float32) error
	RunAdvectionKernel(dt float32) error
	Cleanup()
}