package main

import (
	"fmt"
	"runtime"
)

type ComputeBackend interface {
	UpdateTectonics(planet Planet, deltaYears float64) Planet
	IsEnabled() bool
	Name() string
}

type CPUBackend struct{}

func (c *CPUBackend) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	return updateTectonics(planet, deltaYears)
}

func (c *CPUBackend) IsEnabled() bool { return true }
func (c *CPUBackend) Name() string { return "CPU" }

type MPSBackend struct {
	enabled bool
}

func (m *MPSBackend) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	if !m.enabled {
		return updateTectonics(planet, deltaYears)
	}
	return updateTectonicsSimpleMetal(planet, deltaYears)
}

func (m *MPSBackend) IsEnabled() bool { return m.enabled }
func (m *MPSBackend) Name() string { return "Metal Performance Shaders" }

type OpenCLBackend struct {
	enabled bool
}

func (o *OpenCLBackend) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	if !o.enabled {
		return updateTectonics(planet, deltaYears)
	}
	return updateTectonicsOpenCL(planet, deltaYears)
}

func (o *OpenCLBackend) IsEnabled() bool { return o.enabled }
func (o *OpenCLBackend) Name() string { return "OpenCL" }

var computeBackend ComputeBackend

// LazyMPSBackend delays Metal initialization check until first use
type LazyMPSBackend struct {
	checked bool
	enabled bool
}

func (m *LazyMPSBackend) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	if !m.checked {
		// Check Metal availability on first use
		m.checked = true
		m.enabled = simpleMetalGPU != nil && simpleMetalGPU.enabled
		fmt.Printf("Metal backend check: enabled=%v\n", m.enabled)
	}
	
	if !m.enabled {
		return updateTectonics(planet, deltaYears)
	}
	return updateTectonicsSimpleMetal(planet, deltaYears)
}

func (m *LazyMPSBackend) IsEnabled() bool {
	if !m.checked {
		m.checked = true
		m.enabled = simpleMetalGPU != nil && simpleMetalGPU.enabled
	}
	return m.enabled
}

func (m *LazyMPSBackend) Name() string { return "Metal Performance Shaders" }

func initComputeBackend() ComputeBackend {
	switch runtime.GOOS {
	case "darwin":
		// Check if Apple Silicon
		if runtime.GOARCH == "arm64" {
			fmt.Println("Detected Apple Silicon Mac - will check Metal on first use")
			return &LazyMPSBackend{}
		} else {
			fmt.Println("Detected Intel Mac - attempting OpenCL")
			return &OpenCLBackend{enabled: openclGPU.enabled}
		}
	case "linux", "windows":
		fmt.Println("Detected non-Apple system - attempting OpenCL/CUDA")
		return &OpenCLBackend{enabled: openclGPU.enabled}
	default:
		fmt.Println("Unknown system - using CPU fallback")
		return &CPUBackend{}
	}
}

func init() {
	computeBackend = initComputeBackend()
	fmt.Printf("Compute backend: %s\n", computeBackend.Name())
}