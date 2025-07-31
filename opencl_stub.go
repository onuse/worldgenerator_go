//go:build nometal
// +build nometal

package main

// Stub implementation when OpenCL is not available

type OpenCLGPU struct {
	enabled bool
}

var openclGPU = OpenCLGPU{enabled: false}

func updateTectonicsOpenCL(planet Planet, deltaYears float64) Planet {
	return updateTectonics(planet, deltaYears)
}