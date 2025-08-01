// +build !native

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	
	// Add command line flags
	testMode := flag.Bool("test", false, "Run in test mode")
	textureMode := flag.Bool("texture", false, "Run with texture-based rendering")
	flag.Parse()
	
	if *testMode {
		// Run test
		testVoxelSystem()
		return
	}
	
	if *textureMode {
		fmt.Println("Starting voxel planet server with texture rendering...")
		startTextureServer()
	} else {
		fmt.Println("Starting voxel planet server...")
		startVoxelServer()
	}
}

func testVoxelSystem() {
	fmt.Println("=== Testing Voxel Planet System ===")
	
	// Create a small test planet
	planet := CreateVoxelPlanet(6371000, 8) // 8 shells for testing
	
	fmt.Printf("\nPlanet created successfully!\n")
	fmt.Printf("Surface mesh extraction would create ~%d vertices\n", 40000)
	
	// Test voxel access
	testCoord := VoxelCoord{Shell: 7, Lat: 45, Lon: 90}
	voxel := planet.GetVoxel(testCoord)
	if voxel != nil {
		fmt.Printf("\nSample voxel at %+v:\n", testCoord)
		fmt.Printf("  Material: %d\n", voxel.Type)
		fmt.Printf("  Temperature: %.1f°C\n", voxel.Temperature-273.15)
		fmt.Printf("  Pressure: %.0f Pa\n", voxel.Pressure)
	}
}