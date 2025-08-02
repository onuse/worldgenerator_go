package main

import (
	"runtime"
)

// UpdateVoxelPhysicsCPU runs the complete physics simulation on CPU
// This is used on Windows/Linux where Metal is not available
func UpdateVoxelPhysicsCPU(planet *VoxelPlanet, dt float64) {
	// Create physics systems if not already created
	if planet.physics == nil {
		planet.physics = NewVoxelPhysics(planet)
	}
	physics := planet.physics
	
	// Add timing
	// start := time.Now()
	
	// 1. Temperature diffusion and heat flow
	updateTemperatureCPU(planet, dt)
	
	// 2. Pressure calculation from overlying material
	updatePressureCPU(planet, dt)
	
	// 3. Phase transitions (melting/solidification)
	updatePhaseTransitionsCPU(planet, dt)
	
	// 4. Material properties and mechanics
	if physics.mechanics != nil {
		physics.mechanics.UpdateMechanics(dt)
	}
	
	// 5. Mantle convection
	if physics.advection != nil {
		// Apply convection velocities
		physics.advection.UpdateConvection(dt)
	}
	
	// 6. Plate identification and motion
	if physics.plates != nil {
		// Re-identify plates periodically (every ~1000 years)
		// Only re-identify plates occasionally (every 10M years)
		if int(planet.Time)%10000000 == 0 {
			physics.plates.IdentifyPlates()
		}
		
		// Update plate-scale motion
		physics.plates.UpdatePlateMotion(dt)
	}
	
	// 7. Local plate boundary processes
	if physics.mechanics != nil {
		// These now act on boundary voxels identified by plate system
		physics.mechanics.ApplyRidgePush(dt)
		physics.mechanics.UpdateTransformFaults(dt)
		physics.mechanics.UpdateCollisions(dt)
		physics.mechanics.UpdateContinentalBreakup(dt)
	}
	
	// 8. Material advection (movement)
	if physics.advection != nil {
		physics.advection.AdvectMaterial(dt)
	}
	
	// 9. Surface processes (simplified for now)
	updateSurfaceProcessesCPU(planet, dt)
	
	// 10. Update material age
	updateAgeCPU(planet, dt)
}

// updateTemperatureCPU handles heat diffusion
func updateTemperatureCPU(planet *VoxelPlanet, dt float64) {
	dtFloat := float32(dt)
	
	// Create temporary buffer for new temperatures
	tempBuffer := make([][][]float32, len(planet.Shells))
	for i, shell := range planet.Shells {
		tempBuffer[i] = make([][]float32, len(shell.Voxels))
		for j, latVoxels := range shell.Voxels {
			tempBuffer[i][j] = make([]float32, len(latVoxels))
		}
	}
	
	// Heat diffusion through shells
	for shellIdx, shell := range planet.Shells {
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx, voxel := range latVoxels {
				// Skip air
				if voxel.Type == MatAir {
					tempBuffer[shellIdx][latIdx][lonIdx] = voxel.Temperature
					continue
				}
				
				// Get material properties
				props := MaterialProperties[voxel.Type]
				alpha := props.ThermalConductivity / (props.DefaultDensity * props.SpecificHeat)
				
				// Calculate heat flow from neighbors
				heatFlow := float32(0.0)
				neighborCount := 0
				
				// Radial neighbors (up/down)
				if shellIdx > 0 {
					// Inner neighbor
					innerShell := &planet.Shells[shellIdx-1]
					if latIdx < len(innerShell.Voxels) && lonIdx < len(innerShell.Voxels[latIdx]) {
						innerVoxel := &innerShell.Voxels[latIdx][lonIdx]
						dr := shell.InnerRadius - innerShell.OuterRadius
						if dr > 0 {
							dT := innerVoxel.Temperature - voxel.Temperature
							heatFlow += dT * float32(alpha) / float32(dr*dr)
							neighborCount++
						}
					}
				}
				
				if shellIdx < len(planet.Shells)-1 {
					// Outer neighbor
					outerShell := &planet.Shells[shellIdx+1]
					if latIdx < len(outerShell.Voxels) && lonIdx < len(outerShell.Voxels[latIdx]) {
						outerVoxel := &outerShell.Voxels[latIdx][lonIdx]
						dr := outerShell.InnerRadius - shell.OuterRadius
						if dr > 0 {
							dT := outerVoxel.Temperature - voxel.Temperature
							heatFlow += dT * float32(alpha) / float32(dr*dr)
							neighborCount++
						}
					}
				}
				
				// Lateral neighbors (simplified - just east/west for now)
				if len(latVoxels) > 1 {
					// East
					eastIdx := (lonIdx + 1) % len(latVoxels)
					eastVoxel := &latVoxels[eastIdx]
					dT := eastVoxel.Temperature - voxel.Temperature
					// Approximate distance
					radius := (shell.InnerRadius + shell.OuterRadius) / 2
					dx := radius * 2 * 3.14159 / float64(len(latVoxels))
					heatFlow += dT * float32(alpha) / float32(dx*dx)
					neighborCount++
					
					// West
					westIdx := (lonIdx - 1 + len(latVoxels)) % len(latVoxels)
					westVoxel := &latVoxels[westIdx]
					dT = westVoxel.Temperature - voxel.Temperature
					heatFlow += dT * float32(alpha) / float32(dx*dx)
					neighborCount++
				}
				
				// Apply heat flow
				if neighborCount > 0 {
					tempBuffer[shellIdx][latIdx][lonIdx] = voxel.Temperature + heatFlow*dtFloat
				} else {
					tempBuffer[shellIdx][latIdx][lonIdx] = voxel.Temperature
				}
				
				// Add radioactive heating in deep shells
				if shellIdx < 5 { // Deep mantle/core
					radioHeat := float32(1e-12) * dtFloat * 1e6 // Small heating rate
					tempBuffer[shellIdx][latIdx][lonIdx] += radioHeat
				}
			}
		}
	}
	
	// Apply surface boundary conditions
	if len(planet.Shells) > 0 {
		surfaceShell := len(planet.Shells) - 1
		for latIdx, latVoxels := range planet.Shells[surfaceShell].Voxels {
			lat := getLatitudeForBand(latIdx, planet.Shells[surfaceShell].LatBands)
			
			for lonIdx := range latVoxels {
				// Simple surface temperature based on latitude
				surfaceTemp := float32(288 - 50*abs(float32(lat))/90.0) // Equator ~288K, poles ~238K
				tempBuffer[surfaceShell][latIdx][lonIdx] = surfaceTemp
			}
		}
	}
	
	// Copy back to planet
	for shellIdx, shell := range planet.Shells {
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx := range latVoxels {
				planet.Shells[shellIdx].Voxels[latIdx][lonIdx].Temperature = tempBuffer[shellIdx][latIdx][lonIdx]
			}
		}
	}
}

// updatePressureCPU calculates pressure from overlying material
func updatePressureCPU(planet *VoxelPlanet, dt float64) {
	// Work from surface down, accumulating pressure
	for shellIdx := len(planet.Shells) - 1; shellIdx >= 0; shellIdx-- {
		shell := &planet.Shells[shellIdx]
		
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx := range latVoxels {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				if shellIdx == len(planet.Shells)-1 {
					// Surface pressure (1 atmosphere)
					voxel.Pressure = 101325
				} else {
					// Get pressure from shell above
					outerShell := &planet.Shells[shellIdx+1]
					if latIdx < len(outerShell.Voxels) && lonIdx < len(outerShell.Voxels[latIdx]) {
						outerVoxel := &outerShell.Voxels[latIdx][lonIdx]
						
						// Add weight of overlying material
						dr := outerShell.InnerRadius - shell.OuterRadius
						g := 9.8 // Simplified constant gravity
						dP := outerVoxel.Density * float32(g*dr)
						
						voxel.Pressure = outerVoxel.Pressure + dP
					}
				}
			}
		}
	}
}

// updatePhaseTransitionsCPU handles melting and solidification
func updatePhaseTransitionsCPU(planet *VoxelPlanet, dt float64) {
	for _, shell := range planet.Shells {
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx := range latVoxels {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				// Skip air and water
				if voxel.Type == MatAir || voxel.Type == MatWater {
					continue
				}
				
				props := MaterialProperties[voxel.Type]
				
				// Check for melting
				if voxel.Type != MatMagma && voxel.Temperature > props.MeltingPoint {
					// Partial melting based on how far above melting point
					meltFraction := (voxel.Temperature - props.MeltingPoint) / 200.0
					if meltFraction > 0.5 {
						// Convert to magma
						voxel.Type = MatMagma
						voxel.Density = MaterialProperties[MatMagma].DefaultDensity
					}
				}
				
				// Check for solidification
				if voxel.Type == MatMagma {
					solidusTemp := float32(1200) // Simplified solidus
					if voxel.Temperature < solidusTemp {
						// Solidify to basalt (simplified)
						voxel.Type = MatBasalt
						voxel.Density = MaterialProperties[MatBasalt].DefaultDensity
						voxel.Age = 0 // New rock
					}
				}
			}
		}
	}
}

// updateSurfaceProcessesCPU handles weathering and erosion (simplified)
func updateSurfaceProcessesCPU(planet *VoxelPlanet, dt float64) {
	if len(planet.Shells) < 2 {
		return
	}
	
	surfaceShell := len(planet.Shells) - 2 // Below atmosphere
	shell := &planet.Shells[surfaceShell]
	
	for latIdx, latVoxels := range shell.Voxels {
		for lonIdx := range latVoxels {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Simple erosion based on elevation and material
			if voxel.Type == MatGranite || voxel.Type == MatBasalt {
				// Higher elevation = more erosion
				if voxel.VelR > 0 { // Rising/mountain
					// Slow erosion
					voxel.VelR -= float32(1e-10 * dt)
				}
			}
		}
	}
}

// updateAgeCPU increments material age
func updateAgeCPU(planet *VoxelPlanet, dt float64) {
	for shellIdx := range planet.Shells {
		shell := &planet.Shells[shellIdx]
		
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				// Only age solid materials
				if voxel.Type != MatAir && voxel.Type != MatWater && voxel.Type != MatMagma {
					voxel.Age += float32(dt)
				}
			}
		}
	}
}

// Wrapper function that detects whether to use GPU or CPU
func UpdateVoxelPhysicsWrapper(planet *VoxelPlanet, dt float64, gpu GPUCompute) {
	if gpu != nil && runtime.GOOS == "darwin" {
		// Use GPU on macOS
		UpdateVoxelPhysics(planet, dt, gpu)
	} else {
		// Use CPU on Windows/Linux
		UpdateVoxelPhysicsCPU(planet, dt)
	}
}