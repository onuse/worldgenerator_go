//go:build !darwin || !cgo || nometal
// +build !darwin !cgo nometal

package main

// Stub implementation when Metal is not available

type SimpleMetalGPU struct{}

func initSimpleMetalGPU() *SimpleMetalGPU {
	return nil
}

func (gpu *SimpleMetalGPU) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	return updateTectonics(planet, deltaYears)
}

func updateTectonicsSimpleMetal(planet Planet, deltaYears float64) Planet {
	return updateTectonics(planet, deltaYears)
}

var simpleMetalGPU *SimpleMetalGPU