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
		seed          = flag.Int64("seed", 0, "Random seed for planet generation (0 = use current time)")
		continents    = flag.Int("continents", 7, "Number of initial continental masses")
		oceanFraction = flag.Float64("ocean", 0.7, "Fraction of surface covered by ocean (0.0-1.0)")
		virtualVoxels = flag.Bool("virtual", false, "Use virtual voxel system (experimental)")
	)
	flag.Parse()

	// Initialize random seed
	actualSeed := *seed
	if actualSeed == 0 {
		actualSeed = time.Now().Unix()
	}

	fmt.Println("=== Voxel Planet Evolution Simulator (Native Renderer) ===")
	fmt.Printf("Planet radius: %.0f m\n", *radius)
	fmt.Printf("Shell count: %d\n", *shellCount)
	fmt.Printf("GPU backend: %s\n", *gpuType)
	fmt.Printf("Window: %dx%d\n", *width, *height)
	fmt.Printf("Random seed: %d\n", actualSeed)
	fmt.Printf("Continents: %d masses\n", *continents)
	fmt.Printf("Ocean coverage: %.0f%%\n", *oceanFraction*100)

	// Create voxel planet with randomization
	genParams := core.PlanetGenerationParams{
		Seed:               actualSeed,
		ContinentCount:     *continents,
		OceanFraction:      *oceanFraction,
		MinContinentSize:   0.01, // 1% of surface minimum
		MaxContinentSize:   0.15, // 15% of surface maximum
		ContinentRoughness: 0.7,  // Moderately irregular shapes
	}
	planet := core.CreateRandomizedPlanet(*radius, *shellCount, genParams)
	
	// Initialize virtual voxel system if requested
	if *virtualVoxels {
		fmt.Println("Initializing virtual voxel system...")
		vvs := core.NewVirtualVoxelSystem(planet)
		vvs.ConvertToVirtualVoxels()
		fmt.Printf("Converting surface voxels to virtual voxels...\n")
		vvs.CreateBonds()
		planet.VirtualVoxelSystem = vvs
		planet.UseVirtualVoxels = true
		fmt.Printf("Created %d virtual voxels with %d bonds\n", len(vvs.VirtualVoxels), len(vvs.Bonds))
	}

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
	var computePhysics *gpu.ComputePhysics
	useGPUPhysics := false
	if *gpuType == "compute" {
		cp, err := gpu.NewComputePhysics(planet)
		if err == nil {
			computePhysics = cp
			useGPUPhysics = true
			defer computePhysics.Release()
			fmt.Println("✅ Using GPU compute shaders for physics")

			// Initialize plate tectonics if available
			// TODO: Fix this when physics package is properly integrated
			// For now, skip plate tectonics initialization
			fmt.Println("⚠️  GPU plate tectonics not yet integrated")
		} else {
			fmt.Printf("⚠️  Compute shader physics not available: %v\n", err)
		}
	}

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
		// Using texture mode
		fmt.Println("Using texture rendering mode with optimized GPU buffers")
	}

	// Initialize voxel textures
	renderer.UpdateVoxelTextures(planet)
	
	// Virtual voxel system removed - using standard grid

	// Simulation parameters
	simSpeed := 1000000.0 // 1 million years per second
	//speedMultiplier := 1.0 // Additional speed control
	// lastTime := time.Now() // Not needed with threaded physics
	frameCount := 0
	totalFrameCount := 0 // Never reset this one
	lastFPSTime := time.Now()

	// Create continental drift tracker (removed - not needed with new approach)
	// driftState := physics.NewContinentalDriftState(planet)

	// Create accelerated physics params (removed - not needed with new approach)
	// accelParams := physics.DefaultAcceleratedParams()

	// Create threaded physics engine
	// Use GPU compute physics if available, otherwise use the standard GPU compute interface
	var physicsCompute gpu.GPUCompute
	if useGPUPhysics && computePhysics != nil {
		// Cast ComputePhysics to GPUCompute interface
		physicsCompute = computePhysics
		fmt.Println("✅ Physics engine using GPU compute shaders")
	} else {
		physicsCompute = gpuCompute
	}
	physicsEngine := physics.NewThreadedPhysicsInterface(planet, physicsCompute, simSpeed)
	defer physicsEngine.Stop()

	// Track last GPU update time
	//var lastGPUUpdateTime float64 = -1

	fmt.Println("\nControls:")
	fmt.Println("  1-8: Change visualization (Material/Temp/Velocity/Age/Plates/Stress/SubPos/Elevation)")
	fmt.Println("  X/Y/Z: Toggle cross-section view")
	fmt.Println("  Mouse: Click and drag to rotate")
	fmt.Println("  Scroll: Zoom in/out")
	fmt.Println("  +/-: Speed up/slow down time (current: 1.0x)")
	fmt.Println("  Shift+1 to 5: Set speed to 10x, 100x, 1000x, 10000x, 100000x")
	fmt.Println("  0: Reset speed to 1x")
	fmt.Println("  P: Pause/unpause simulation")
	fmt.Println("  H: Create hotspot at cursor")
	fmt.Println("  ESC: Exit")
	fmt.Println("\nStarting simulation...")

	// Main loop
	for !renderer.ShouldClose() {
		renderer.PollEvents()

		// Calculate delta time
		now := time.Now()
		// dt := now.Sub(lastTime).Seconds() // Not used anymore
		// lastTime = now // Not needed anymore

		// Apply speed multiplier from renderer controls
		currentSpeed := simSpeed * float64(renderer.SpeedMultiplier)

		// Update physics engine with new speed
		physicsEngine.UpdateSimSpeed(currentSpeed)

		// Check if physics thread has new data (unless paused)
		physicsUpdated := false
		if !renderer.Paused {
			if updatedPlanet, hasUpdate := physicsEngine.Update(); hasUpdate {
				// Use the updated planet data from physics thread
				planet = updatedPlanet
				physicsUpdated = true
				renderer.PlanetRef = planet // Update renderer's reference

				// Debug output removed for cleaner display

				// Don't apply additional acceleration - let physics handle it
			}
			// Time is already updated in physics thread
		}

		// Update GPU data only when physics updated
		if physicsUpdated {
			// Updates tracked internally
			

			if gpuBufferMgr != nil {
				// Using optimized GPU buffer manager
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
				// Ensure buffers are synced to GPU
				gpuBufferMgr.BindBuffers()
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

			// Also update voxel textures when physics updated
			renderer.UpdateVoxelTextures(planet)
		}

		// Render
		renderer.Render()

		// FPS counter and performance report
		frameCount++
		totalFrameCount++

		// Update stats overlay and console output
		if now.Sub(lastFPSTime).Seconds() >= 5.0 { // Update every 5 seconds
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
				speedStr := ""
				if renderer.SpeedMultiplier != 1.0 {
					speedStr = fmt.Sprintf(" | Speed: %.0fx", renderer.SpeedMultiplier)
				}
				if renderer.Paused {
					speedStr = " | PAUSED"
				}
				fmt.Printf("\rFPS: %.1f | Physics: %.1fms | Zoom: %.3f | Distance: %.0f km | Sim Time: %.1f My%s    ",
					fps, physicsTime, zoomLevel, cameraDistance/1000.0, planet.Time/1000000, speedStr)
			}
			frameCount = 0
			lastFPSTime = now
		}
	}

	fmt.Println("\nShutting down...")
}
