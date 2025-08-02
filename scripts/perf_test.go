package main

import (
	"fmt"
	"runtime"
	"time"
	"worldgenerator/core"
	"worldgenerator/scripts"
)

func main() {
	runtime.LockOSThread()

	fmt.Println("=== Performance Test ===")

	// Test 1: Planet creation
	start := time.Now()
	planet := core.CreateVoxelPlanet(6371000, 20)
	fmt.Printf("Planet creation: %.3fs\n", time.Since(start).Seconds())

	// Count total voxels
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, latBand := range shell.Voxels {
			totalVoxels += len(latBand)
		}
	}
	fmt.Printf("Total voxels: %d\n", totalVoxels)

	// Test 2: Physics update (if enabled)
	start = time.Now()
	scripts.UpdateVoxelPhysicsCPU(planet, 1000.0) // 1000 seconds
	fmt.Printf("Physics update: %.3fs\n", time.Since(start).Seconds())

	// Test 3: Buffer creation
	start = time.Now()
	buffers := scripts.NewSharedGPUBuffers(planet)
	buffers.UpdateFromPlanet(planet)
	fmt.Printf("Buffer update: %.3fs\n", time.Since(start).Seconds())

	// Test 4: Texture data creation
	start = time.Now()
	texData := scripts.NewVoxelTextureData(30)
	fmt.Printf("Texture creation: %.3fs\n", time.Since(start).Seconds())

	// Test 5: Texture update
	start = time.Now()
	texData.UpdateFromPlanet(planet)
	fmt.Printf("Texture update: %.3fs\n", time.Since(start).Seconds())

	// Test 6: With plate data
	if planet.physics != nil && planet.physics.plates != nil {
		start = time.Now()
		scripts.UpdateSharedBuffersWithPlates(buffers, planet, planet.physics.plates)
		fmt.Printf("Buffer update with plates: %.3fs\n", time.Since(start).Seconds())
	}

	fmt.Println("\n=== Test Complete ===")
}
