package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
	
	"worldgenerator/core"
	"worldgenerator/gpu"
	"worldgenerator/gpu/opencl"
	"worldgenerator/physics"
	"worldgenerator/rendering/opengl"
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
	planet := core.CreateVoxelPlanet(*radius, *shellCount)
	
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
	var gpuCompute gpu.GPUCompute
	var err error
	
	switch *gpuType {
	case "metal":
		if runtime.GOOS != "darwin" {
			log.Fatal("Metal is only available on macOS")
		}
		// Metal compute
		mc, err := gpu.NewMetalCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize Metal compute: %v", err)
		}
		gpuCompute = mc
	case "opencl":
		gpuCompute, err = opencl.NewOpenCLCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize OpenCL compute: %v", err)
		}
	case "cuda":
		log.Fatal("CUDA support not yet implemented")
	case "cpu", "compute":
		// For compute shaders, we still need a fallback CPU compute for initialization
		gpuCompute, err = gpu.NewCPUCompute(planet)
		if err != nil {
			log.Fatalf("Failed to initialize CPU compute: %v", err)
		}
	default:
		log.Fatalf("Unknown GPU backend: %s", *gpuType)
	}
	defer gpuCompute.Cleanup()
	
	// Create native OpenGL renderer
	renderer, err := opengl.NewVoxelRenderer(*width, *height)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Terminate()
	
	// Set planet reference for mouse picking
	renderer.PlanetRef = planet
	
	// Try to create GPU compute physics (OpenGL 4.3 compute shaders)
	// TODO: Implement ComputePhysics when needed
	// var computePhysics *physics.ComputePhysics
	// useGPUPhysics := false
	// if *gpuType == "compute" || (*gpuType == "cpu" && renderer.HasComputeShaderSupport()) {
	// 	cp, err := physics.NewComputePhysics(planet)
	// 	if err == nil {
	// 		computePhysics = cp
	// 		useGPUPhysics = true
	// 		defer computePhysics.Release()
	// 		fmt.Println("✅ Using GPU compute shaders for physics")
			
	// 		// Initialize plate tectonics if available
	// 		// TODO: Fix this when physics package is properly integrated
	// 		// if planet.Physics != nil && planet.Physics.plates != nil {
	// 		if false {
	// 			// if err := computePhysics.InitializePlateTectonics(planet.Physics.plates); err == nil {
	// 			if false {
	// 				fmt.Println("✅ GPU plate tectonics initialized")
	// 			} else {
	// 				fmt.Printf("⚠️  Plate tectonics not available: %v\n", err)
	// 			}
	// 		}
	// 	} else {
	// 		fmt.Printf("⚠️  Compute shader physics not available: %v\n", err)
	// 	}
	// }
	
	// Try to create optimized GPU buffer manager
	var gpuBufferMgr *gpu.WindowsGPUBufferManager
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		if mgr, err := gpu.NewWindowsGPUBufferManager(planet); err == nil {
			gpuBufferMgr = mgr
			defer gpuBufferMgr.Release()
			fmt.Println("✅ Using optimized GPU buffer sharing")
			if mgr.UsePersistent {
				fmt.Println("✅ Using persistent mapped buffers (zero-copy)")
			} else {
				fmt.Println("⚠️  Using standard buffers (requires copy)")
			}
		} else {
			fmt.Printf("❌ GPU buffer optimization not available: %v\n", err)
		}
	}
	
	// Create shared buffer manager (fallback)
	var sharedBuffers *gpu.SharedGPUBuffers
	if gpuBufferMgr == nil {
		sharedBuffers = gpu.NewSharedGPUBuffers(planet)
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
	
	// Create threaded physics engine
	physicsEngine := physics.NewThreadedPhysicsInterface(planet, gpuCompute, simSpeed)
	defer physicsEngine.Stop()
	
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
		
		// Check if physics thread has new data
		physicsUpdated := false
		if updatedPlanet, hasUpdate := physicsEngine.Update(); hasUpdate {
			// Use the updated planet data from physics thread
			planet = updatedPlanet
			physicsUpdated = true
			renderer.PlanetRef = planet // Update renderer's reference
		}
		planet.Time += dt * simSpeed
		
		// Update GPU data only when physics updated
		if physicsUpdated {
			if gpuBufferMgr != nil {
				// Only include plate data when in plate visualization mode
				// TODO: Fix this when physics package is properly integrated
				// if renderer.RenderMode == 4 {
				// 	if vp, ok := planet.Physics.(*physics.VoxelPhysics); ok && vp.plates != nil {
				// 		gpuBufferMgr.UpdateFromPlanetWithPlates(planet, vp.plates)
				// 	} else {
				// 		gpuBufferMgr.UpdateFromPlanet(planet)
				// 	}
				// } else {
				gpuBufferMgr.UpdateFromPlanet(planet)
				// }
			} else {
				// Fallback path - copy through shared buffers
				// TODO: Fix this when physics package is properly integrated
				// if renderer.RenderMode == 4 {
				// 	if vp, ok := planet.Physics.(*physics.VoxelPhysics); ok && vp.plates != nil {
				// 		simulation.UpdateSharedBuffersWithPlates(sharedBuffers, planet, vp.plates)
				// 	} else {
				// 		sharedBuffers.UpdateFromPlanet(planet)
				// 	}
				// } else {
				sharedBuffers.UpdateFromPlanet(planet)
				// }
				renderer.UpdateBuffers(sharedBuffers)
			}
			
			// Also update voxel textures (skip if using SSBO or optimized buffers)
			if gpuBufferMgr == nil && !renderer.UseSSBO {
				renderer.UpdateVoxelTextures(planet)
			}
		}
		
		// Render
		renderer.Render()
		
		// FPS counter and performance report
		frameCount++
		totalFrameCount++
		
		// Update stats overlay and console output
		if now.Sub(lastFPSTime).Seconds() >= 0.5 { // Update every 0.5 seconds
			fps := float64(frameCount) / now.Sub(lastFPSTime).Seconds()
			renderer.UpdateStats(fps)
			
			// Also print to console if not quiet
			if !*quiet {
				// Calculate zoom level
				cameraDistance := renderer.GetCameraDistance()
				zoomLevel := float64(*radius) * 3.0 / float64(cameraDistance)
				// Get physics performance
				physicsTime := physicsEngine.GetPhysicsFrameTime() * 1000 // Convert to ms
				// Output with zoom info
				fmt.Printf("\rFPS: %.1f | Physics: %.1fms | Zoom: %.3f | Distance: %.0f km | Sim Time: %.1f My    ", 
					fps, physicsTime, zoomLevel, cameraDistance/1000.0, planet.Time/1000000)
			}
			frameCount = 0
			lastFPSTime = now
		}
	}
	
	fmt.Println("\nShutting down...")
}