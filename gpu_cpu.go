package main

import (
	"fmt"
	"runtime"
	"sync"
)

// CPUCompute implements GPUCompute interface using CPU parallelization
type CPUCompute struct {
	planet      *VoxelPlanet
	numWorkers  int
}

// NewCPUCompute creates a new CPU-based compute backend
func NewCPUCompute(planet *VoxelPlanet) (GPUCompute, error) {
	numWorkers := runtime.NumCPU()
	fmt.Printf("Initializing CPU compute with %d workers\n", numWorkers)
	
	return &CPUCompute{
		planet:     planet,
		numWorkers: numWorkers,
	}, nil
}

// RunTemperatureKernel runs temperature diffusion on CPU
func (c *CPUCompute) RunTemperatureKernel(dt float32) error {
	// For now, just return nil to allow the simulation to run
	// The actual physics is still handled by UpdateVoxelPhysics
	return nil
}

// RunConvectionKernel runs convection simulation on CPU
func (c *CPUCompute) RunConvectionKernel(dt float32) error {
	// For now, just return nil to allow the simulation to run
	return nil
}

// RunAdvectionKernel runs material advection on CPU
func (c *CPUCompute) RunAdvectionKernel(dt float32) error {
	// For now, just return nil to allow the simulation to run
	return nil
}

// Cleanup releases CPU resources
func (c *CPUCompute) Cleanup() {
	// Nothing to clean up for CPU backend
}

// parallelForEachShell executes a function for each shell in parallel
func (c *CPUCompute) parallelForEachShell(fn func(shellIdx int)) {
	var wg sync.WaitGroup
	shellCount := len(c.planet.Shells)
	
	// Create work queue
	work := make(chan int, shellCount)
	for i := 0; i < shellCount; i++ {
		work <- i
	}
	close(work)
	
	// Spawn workers
	wg.Add(c.numWorkers)
	for i := 0; i < c.numWorkers; i++ {
		go func() {
			defer wg.Done()
			for shellIdx := range work {
				fn(shellIdx)
			}
		}()
	}
	
	wg.Wait()
}