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
		gpuType       = flag.String("gpu", "cpu", "GPU compute backend (metal, opencl, cuda, compute, cpu)")
		width         = flag.Int("width", 1280, "Window width")
		height        = flag.Int("height", 720, "Window height")
		quiet         = flag.Bool("quiet", false, "Disable console output for smooth rendering")
	)
	flag.Parse()
	
	fmt.Println("=== Voxel Planet Evolution Simulator (Native Renderer) ===")
	fmt.Printf("Planet radius: %.0f m\n", *radius)
	fmt.Printf("Shell count: %d\n", *shellCount)
	fmt.Printf("GPU backend: %s\n", *gpuType)
	fmt.Printf("Window: %dx%d\n", *width, *height)
	
	// Create voxel planet
	planet := CreateVoxelPlanet(*radius, *shellCount)
	
	// Count voxels
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, latBand := range shell.Voxels {
			totalVoxels += len(latBand)
		}
	}
	fmt.Printf("Total voxels: %d (%.1f million)\n", totalVoxels, float64(totalVoxels)/1000000)
	fmt.Printf("Data size: %.1f MB per frame\n", float64(totalVoxels*64)/(1024*1024))
	
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
	case "cpu", "compute":
		// For compute shaders, we still need a fallback CPU compute for initialization
		gpuCompute, err = NewCPUCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize CPU compute: %v", err)
		}
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
	
	// Set planet reference for mouse picking
	renderer.planetRef = planet
	
	// Try to create GPU compute physics (OpenGL 4.3 compute shaders)
	var computePhysics *ComputePhysics
	useGPUPhysics := false
	if *gpuType == "compute" || (*gpuType == "cpu" && renderer.HasComputeShaderSupport()) {
		cp, err := NewComputePhysics(planet)
		if err == nil {
			computePhysics = cp
			useGPUPhysics = true
			defer computePhysics.Release()
			fmt.Println("✅ Using GPU compute shaders for physics")
			
			// Initialize plate tectonics if available
			if planet.physics != nil && planet.physics.plates != nil {
				if err := computePhysics.InitializePlateTectonics(planet.physics.plates); err == nil {
					fmt.Println("✅ GPU plate tectonics initialized")
				} else {
					fmt.Printf("⚠️  Plate tectonics not available: %v\n", err)
				}
			}
		} else {
			fmt.Printf("⚠️  Compute shader physics not available: %v\n", err)
		}
	}
	
	// Try to create optimized GPU buffer manager
	var gpuBufferMgr *WindowsGPUBufferManager
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		if mgr, err := NewWindowsGPUBufferManager(planet); err == nil {
			gpuBufferMgr = mgr
			defer gpuBufferMgr.Release()
			fmt.Println("✅ Using optimized GPU buffer sharing")
			if mgr.usePersistent {
				fmt.Println("✅ Using persistent mapped buffers (zero-copy)")
			} else {
				fmt.Println("⚠️  Using standard buffers (requires copy)")
			}
		} else {
			fmt.Printf("❌ GPU buffer optimization not available: %v\n", err)
		}
	}
	
	// Create shared buffer manager (fallback)
	var sharedBuffers *SharedGPUBuffers
	if gpuBufferMgr == nil {
		sharedBuffers = NewSharedGPUBuffers(planet)
		sharedBuffers.UpdateFromPlanet(planet)
		// Create OpenGL buffers
		renderer.CreateBuffers(sharedBuffers)
	} else {
		// Use optimized buffers
		gpuBufferMgr.UpdateFromPlanet(planet)
		renderer.SetOptimizedBuffers(gpuBufferMgr)
	}
	
	// Initialize voxel textures
	renderer.UpdateVoxelTextures(planet)
	
	// Simulation parameters
	simSpeed := 1000000.0 // 1 million years per second
	lastTime := time.Now()
	frameCount := 0
	totalFrameCount := 0 // Never reset this one
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
		
		// Update simulation only occasionally
		physicsUpdated := false
		if totalFrameCount > 0 && totalFrameCount % 300 == 0 { // Update physics every 300 frames, skip frame 0
			if useGPUPhysics && computePhysics != nil && gpuBufferMgr != nil {
				// Run physics on GPU using compute shaders
				gpuBufferMgr.SyncToGPU() // Ensure GPU has latest data
				computePhysics.RunPhysicsStep(float32(dt*simSpeed*300), float32(*radius), 9.8)
				// Sync back to CPU for visualization and plate updates
				gpuBufferMgr.UpdateToPlanet(planet)
				
				// Update plate boundaries after GPU physics modified velocities
				if planet.physics != nil && planet.physics.plates != nil {
					// Re-identify boundaries since voxels may have moved
					for _, plate := range planet.physics.plates.Plates {
						planet.physics.plates.identifyPlateBoundaries(plate)
					}
				}
			} else {
				// Fallback to CPU physics
				UpdateVoxelPhysicsWrapper(planet, dt*simSpeed*300, gpuCompute)
			}
			physicsUpdated = true
		}
		planet.Time += dt * simSpeed
		
		// Update GPU data only when physics updated
		if physicsUpdated {
			if gpuBufferMgr != nil {
				// Only include plate data when in plate visualization mode
				if renderer.renderMode == 4 && planet.physics != nil && planet.physics.plates != nil {
					gpuBufferMgr.UpdateFromPlanetWithPlates(planet, planet.physics.plates)
				} else {
					gpuBufferMgr.UpdateFromPlanet(planet)
				}
			} else {
				// Fallback path - copy through shared buffers
				if renderer.renderMode == 4 && planet.physics != nil && planet.physics.plates != nil {
					UpdateSharedBuffersWithPlates(sharedBuffers, planet, planet.physics.plates)
				} else {
					sharedBuffers.UpdateFromPlanet(planet)
				}
				renderer.UpdateBuffers(sharedBuffers)
			}
			
			// Also update voxel textures (skip if using SSBO or optimized buffers)
			if gpuBufferMgr == nil && !renderer.useSSBO {
				renderer.UpdateVoxelTextures(planet)
			}
		}
		
		// Render
		renderer.Render()
		
		// FPS counter and performance report
		frameCount++
		totalFrameCount++
		
		// Print FPS counter less frequently to avoid hiccups
		if !*quiet && now.Sub(lastFPSTime).Seconds() >= 5.0 { // Every 5 seconds instead of 1
			fps := float64(frameCount) / now.Sub(lastFPSTime).Seconds()
			// Simple one-line output to minimize console overhead
			fmt.Printf("\rFPS: %.1f | Sim Time: %.1f My                    ", fps, planet.Time/1000000)
			frameCount = 0
			lastFPSTime = now
		}
	}
	
	fmt.Println("\nShutting down...")
}