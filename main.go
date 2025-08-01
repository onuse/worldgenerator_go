package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
)

func main() {
	runtime.LockOSThread()
	
	// Parse command line flags
	var (
		radius        = flag.Float64("radius", 6371000, "Planet radius in meters")
		shellCount    = flag.Int("shells", 20, "Number of spherical shells")
		gpuType       = flag.String("gpu", "metal", "GPU compute backend (metal, opencl, cuda)")
		width         = flag.Int("width", 1280, "Window width")
		height        = flag.Int("height", 720, "Window height")
	)
	flag.Parse()
	
	fmt.Println("=== Voxel Planet Evolution Simulator (Native Renderer) ===")
	fmt.Printf("Planet radius: %.0f m\n", *radius)
	fmt.Printf("Shell count: %d\n", *shellCount)
	fmt.Printf("GPU backend: %s\n", *gpuType)
	fmt.Printf("Window: %dx%d\n", *width, *height)
	
	// Debug continents
	// DebugContinentalness()
	
	// Create voxel planet
	planet := CreateVoxelPlanet(*radius, *shellCount)
	
	// Initialize GPU compute
	var gpuCompute GPUCompute
	var err error
	
	switch *gpuType {
	case "metal":
		if runtime.GOOS != "darwin" {
			log.Fatal("Metal is only available on macOS")
		}
		gpuCompute, err = NewMetalCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize Metal compute: %v", err)
		}
	case "opencl":
		gpuCompute, err = NewOpenCLCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize OpenCL compute: %v", err)
		}
	case "cuda":
		log.Fatal("CUDA support not yet implemented")
	default:
		log.Fatalf("Unknown GPU backend: %s", *gpuType)
	}
	defer gpuCompute.Cleanup()
	
	// Create native OpenGL renderer
	renderer, err := NewVoxelRenderer(*width, *height)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Terminate()
	
	// Create shared buffer manager
	sharedBuffers := NewSharedGPUBuffers(planet)
	sharedBuffers.UpdateFromPlanet(planet)
	
	// Create OpenGL buffers
	renderer.CreateBuffers(sharedBuffers)
	
	// Initialize voxel textures
	renderer.UpdateVoxelTextures(planet)
	
	// Simulation parameters
	simSpeed := 1000000.0 // 1 million years per second
	lastTime := time.Now()
	frameCount := 0
	lastFPSTime := time.Now()
	
	fmt.Println("\nControls:")
	fmt.Println("  1-4: Change visualization (Material/Temperature/Velocity/Age)")
	fmt.Println("  X/Y/Z: Toggle cross-section view")
	fmt.Println("  Mouse: Click and drag to rotate")
	fmt.Println("  Scroll: Zoom in/out")
	fmt.Println("  ESC: Exit")
	fmt.Println("\nStarting simulation...")
	
	// Main loop
	for !renderer.ShouldClose() {
		renderer.PollEvents()
		
		// Calculate delta time
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now
		
		// Update simulation
		UpdateVoxelPhysics(planet, dt*simSpeed, gpuCompute)
		planet.Time += dt * simSpeed
		
		// Update shared buffers from planet data
		sharedBuffers.UpdateFromPlanet(planet)
		renderer.UpdateBuffers(sharedBuffers)
		
		// Also update voxel textures
		renderer.UpdateVoxelTextures(planet)
		
		// Render
		renderer.Render()
		
		// FPS counter
		frameCount++
		if now.Sub(lastFPSTime).Seconds() >= 1.0 {
			fps := float64(frameCount) / now.Sub(lastFPSTime).Seconds()
			fmt.Printf("\rFPS: %.1f | Sim Time: %.1f My", fps, planet.Time/1000000)
			frameCount = 0
			lastFPSTime = now
		}
	}
	
	fmt.Println("\nShutting down...")
}