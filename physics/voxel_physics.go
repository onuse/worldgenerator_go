package physics

import (
	"fmt"
	"math"
	"worldgenerator/core"
	"worldgenerator/gpu"
	"worldgenerator/simulation"
)

// VoxelPhysics handles all physical simulations for the voxel planet
type VoxelPhysics struct {
	planet *core.VoxelPlanet

	// Simulation parameters
	thermalDiffusivity float64 // m²/s
	solarConstant      float64 // W/m²
	stefanBoltzmann    float64 // W/(m²·K⁴)

	// Subsystems
	advection *VoxelAdvection
	mechanics *VoxelMechanics
	plates    *simulation.PlateManager

	// GPU acceleration
	gpuCompute *gpu.MetalCompute
	useGPU     bool

	// Debug tracking
	lastPrintTime float64
}

// NewVoxelPhysics creates a physics simulator for the planet
func NewVoxelPhysics(planet *core.VoxelPlanet) *VoxelPhysics {
	vp := &VoxelPhysics{
		planet:             planet,
		thermalDiffusivity: 1e-6,    // Rock thermal diffusivity
		solarConstant:      1361.0,  // Solar radiation at Earth
		stefanBoltzmann:    5.67e-8, // Stefan-Boltzmann constant
	}

	// Create subsystems
	vp.advection = NewVoxelAdvection(planet, vp)
	vp.mechanics = NewVoxelMechanics(planet, vp)
	vp.plates = simulation.NewPlateManager(planet)

	// Initialize convection patterns
	vp.advection.InitializeConvectionCells()

	// Identify initial plates
	vp.plates.IdentifyPlates()
	fmt.Printf("✅ Identified %d tectonic plates\n", len(vp.plates.Plates))

	// Try to initialize GPU acceleration
	if gpuCompute, err := gpu.NewMetalCompute(planet); err == nil {
		if err := gpuCompute.InitializeForPlanet(planet); err == nil {
			vp.gpuCompute = gpuCompute
			vp.useGPU = true
			fmt.Println("✅ GPU acceleration enabled via Metal")
		} else {
			gpuCompute.Release()
			fmt.Printf("⚠️  GPU initialization failed: %v\n", err)
		}
	} else {
		fmt.Printf("ℹ️  GPU not available: %v\n", err)
	}

	return vp
}

// UpdatePhysics performs one physics timestep
func (vp *VoxelPhysics) UpdatePhysics(deltaTime float64) {
	if vp.useGPU {
		// GPU-accelerated physics
		// Upload current state
		// TODO: Need to export uploadPlanetData method or use a different approach
		// if err := vp.gpuCompute.uploadPlanetData(vp.planet); err != nil {
		//     fmt.Printf("GPU upload error: %v\n", err)
		//     vp.useGPU = false
		//     return
		// }

		// Run physics kernels
		if err := vp.gpuCompute.UpdateTemperature(deltaTime); err != nil {
			fmt.Printf("GPU temperature error: %v\n", err)
		}

		if err := vp.gpuCompute.UpdateConvection(deltaTime); err != nil {
			fmt.Printf("GPU convection error: %v\n", err)
		}

		if err := vp.gpuCompute.UpdateAdvection(deltaTime); err != nil {
			fmt.Printf("GPU advection error: %v\n", err)
		}

		// Download results
		// TODO: Need to export downloadPlanetData method or use a different approach
		// if err := vp.gpuCompute.downloadPlanetData(vp.planet); err != nil {
		//     fmt.Printf("GPU download error: %v\n", err)
		//     vp.useGPU = false
		//     return
		// }
	} else {
		// CPU fallback - simplified for demo
		vp.advection.AdvectMaterial(deltaTime)
	}

	// Mark surface as needing remesh
	vp.planet.MeshDirty = true

	// Debug: Print average temperatures periodically
	if vp.planet.Time-vp.lastPrintTime > 1000.0 {
		vp.lastPrintTime = vp.planet.Time

		// Check for surface volcanism
		volcanicCount := vp.countSurfaceVolcanism()

		// Detect plate boundaries
		boundaries := vp.mechanics.DetectPlateBoundaries()
		divergentCount := 0
		convergentCount := 0
		transformCount := 0
		for _, b := range boundaries {
			switch b.Type {
			case "divergent":
				divergentCount++
			case "convergent":
				convergentCount++
			case "transform":
				transformCount++
			}
		}

		// Sample lithosphere thickness
		avgLitho := 0.0
		samples := 0
		for i := 0; i < 10; i++ {
			lat := i * 18
			lon := i * 36
			thickness := vp.mechanics.GetLithosphereThickness(lat, lon)
			if thickness < vp.planet.Radius {
				avgLitho += thickness / 1000 // Convert to km
				samples++
			}
		}
		if samples > 0 {
			avgLitho /= float64(samples)
		}

		fmt.Printf("Year %.0f: T(S/M/C)=%.0f/%.0f/%.0f°C, Volc=%d, Bound(D/C/T)=%d/%d/%d, Litho=%.0fkm\n",
			vp.planet.Time,
			vp.GetAverageTemperature(len(vp.planet.Shells)-1)-273.15,
			vp.GetAverageTemperature(len(vp.planet.Shells)/2)-273.15,
			vp.GetAverageTemperature(0)-273.15,
			volcanicCount,
			divergentCount,
			convergentCount,
			transformCount,
			avgLitho)
	}
}

// updateTemperature implements heat diffusion and surface heating/cooling
func (vp *VoxelPhysics) updateTemperature(dt float64) {
	// Process each shell
	for shellIdx := range vp.planet.Shells {
		shell := &vp.planet.Shells[shellIdx]

		// Create temporary array for new temperatures
		newTemps := make([][]float32, len(shell.Voxels))
		for i := range newTemps {
			newTemps[i] = make([]float32, len(shell.Voxels[i]))
		}

		// Heat diffusion within the shell
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip air - it has its own dynamics
				if voxel.Type == core.MatAir {
					newTemps[latIdx][lonIdx] = voxel.Temperature
					continue
				}

				// Get material properties
				props := core.MaterialProperties[voxel.Type]

				// Calculate thermal diffusivity: α = k/(ρ*c)
				alpha := float64(props.ThermalConductivity) /
					(float64(voxel.Density) * float64(props.SpecificHeat))

				// Get neighbor temperatures
				tempSum := float64(voxel.Temperature)
				neighborCount := 1.0

				// Radial neighbors (up/down between shells)
				if shellIdx > 0 {
					// Inner neighbor
					innerVoxel := vp.getRadialNeighbor(shellIdx, shellIdx-1, latIdx, lonIdx)
					if innerVoxel != nil && innerVoxel.Type != core.MatAir {
						tempSum += float64(innerVoxel.Temperature)
						neighborCount++
					}
				}
				if shellIdx < len(vp.planet.Shells)-1 {
					// Outer neighbor
					outerVoxel := vp.getRadialNeighbor(shellIdx, shellIdx+1, latIdx, lonIdx)
					if outerVoxel != nil && outerVoxel.Type != core.MatAir {
						tempSum += float64(outerVoxel.Temperature)
						neighborCount++
					}
				}

				// Lateral neighbors (within shell)
				// North
				if latIdx > 0 && lonIdx < len(shell.Voxels[latIdx-1]) {
					tempSum += float64(shell.Voxels[latIdx-1][lonIdx].Temperature)
					neighborCount++
				}
				// South
				if latIdx < len(shell.Voxels)-1 && lonIdx < len(shell.Voxels[latIdx+1]) {
					tempSum += float64(shell.Voxels[latIdx+1][lonIdx].Temperature)
					neighborCount++
				}
				// East (with wrapping)
				eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
				tempSum += float64(shell.Voxels[latIdx][eastLon].Temperature)
				neighborCount++
				// West (with wrapping)
				westLon := (lonIdx - 1 + len(shell.Voxels[latIdx])) % len(shell.Voxels[latIdx])
				tempSum += float64(shell.Voxels[latIdx][westLon].Temperature)
				neighborCount++

				// Average neighbor temperature
				avgNeighborTemp := tempSum / neighborCount

				// Heat diffusion equation: dT/dt = α∇²T
				// Using finite difference approximation
				dr := (shell.OuterRadius - shell.InnerRadius) / 1000.0 // Convert to km for stability
				dTemp := alpha * (avgNeighborTemp - float64(voxel.Temperature)) * dt / (dr * dr)

				// Internal heat generation (radioactive decay)
				if shellIdx < len(vp.planet.Shells)/2 {
					// More heating in deeper layers
					internalHeat := 1e-9 * dt // Simplified radioactive heating
					dTemp += internalHeat
				}

				newTemps[latIdx][lonIdx] = voxel.Temperature + float32(dTemp)
			}
		}

		// Apply surface heating/cooling for outermost shell
		if shellIdx == len(vp.planet.Shells)-1 {
			vp.applySurfaceHeatExchange(shell, newTemps, dt)
		}

		// Update temperatures
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				shell.Voxels[latIdx][lonIdx].Temperature = newTemps[latIdx][lonIdx]
			}
		}
	}
}

// applySurfaceHeatExchange handles solar heating and radiative cooling
func (vp *VoxelPhysics) applySurfaceHeatExchange(shell *core.SphericalShell, temps [][]float32, dt float64) {
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only apply to surface materials
			if voxel.Type == core.MatAir {
				continue
			}

			// Calculate latitude for solar angle
			lat := core.GetLatitudeForBand(latIdx, shell.LatBands) * math.Pi / 180.0

			// Simple day/night cycle based on longitude
			lon := float64(lonIdx) / float64(len(shell.Voxels[latIdx])) * 2 * math.Pi
			dayFactor := math.Max(0, math.Cos(lon-vp.planet.Time*7.27e-5)) // Earth rotation rate

			// Solar heating (simplified - no atmosphere)
			solarHeating := vp.solarConstant * math.Cos(lat) * dayFactor

			// Stefan-Boltzmann cooling
			temp := float64(voxel.Temperature)
			radiativeCooling := vp.stefanBoltzmann * temp * temp * temp * temp

			// Net heat flux
			netFlux := solarHeating - radiativeCooling

			// Convert to temperature change
			props := core.MaterialProperties[voxel.Type]
			dTemp := netFlux * dt / (float64(voxel.Density) * float64(props.SpecificHeat) * 1000.0) // 1000m depth assumption

			temps[latIdx][lonIdx] += float32(dTemp)

			// Clamp to reasonable values
			if temps[latIdx][lonIdx] < 0 {
				temps[latIdx][lonIdx] = 0
			}
			if temps[latIdx][lonIdx] > 6000 {
				temps[latIdx][lonIdx] = 6000 // Max temp (hotter than Earth's core)
			}
		}
	}
}

// updatePressure calculates pressure from overlying material
func (vp *VoxelPhysics) updatePressure() {
	g := 9.81 // Gravity (m/s²)

	// Start from the top and work down
	for shellIdx := len(vp.planet.Shells) - 1; shellIdx >= 0; shellIdx-- {
		shell := &vp.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				if shellIdx == len(vp.planet.Shells)-1 {
					// Surface pressure
					if voxel.Type == core.MatAir {
						// Atmospheric pressure decreases with altitude
						voxel.Pressure = 101325 // 1 atm at sea level
					} else {
						voxel.Pressure = 101325
					}
				} else {
					// Pressure from overlying shell
					outerVoxel := vp.getRadialNeighbor(shellIdx, shellIdx+1, latIdx, lonIdx)
					if outerVoxel != nil {
						// Add weight of overlying material
						dr := shell.OuterRadius - shell.InnerRadius
						additionalPressure := float32(float64(outerVoxel.Density) * g * dr)
						voxel.Pressure = outerVoxel.Pressure + additionalPressure
					}
				}
			}
		}
	}
}

// updatePhases handles melting and solidification
func (vp *VoxelPhysics) updatePhases() {
	for shellIdx := range vp.planet.Shells {
		shell := &vp.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip air and water (handled separately)
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				props := core.MaterialProperties[voxel.Type]

				// Adjust melting point for pressure (Clausius-Clapeyron)
				// Simplified: 1°C per 30 MPa
				meltingPoint := props.MeltingPoint + (voxel.Pressure-101325)/30e6

				// Check for melting
				if voxel.Type != core.MatMagma && voxel.Temperature > float32(meltingPoint) {
					// Melt to magma
					if voxel.Type == core.MatBasalt || voxel.Type == core.MatGranite {
						voxel.Type = core.MatMagma
						voxel.Density = core.MaterialProperties[core.MatMagma].DefaultDensity
					}
				}

				// Check for solidification
				if voxel.Type == core.MatMagma && voxel.Temperature < float32(meltingPoint-100) {
					// Solidify based on composition
					if voxel.Composition < 0.5 {
						voxel.Type = core.MatGranite // Felsic magma -> granite
					} else {
						voxel.Type = core.MatBasalt // Mafic magma -> basalt
					}
					voxel.Density = core.MaterialProperties[voxel.Type].DefaultDensity
				}

				// Water phase changes
				if voxel.Type == core.MatWater && voxel.Temperature < 273.15 {
					voxel.Type = core.MatIce
					voxel.Density = core.MaterialProperties[core.MatIce].DefaultDensity
				}
				if voxel.Type == core.MatIce && voxel.Temperature > 273.15 {
					voxel.Type = core.MatWater
					voxel.Density = core.MaterialProperties[core.MatWater].DefaultDensity
				}
			}
		}
	}
}

// getRadialNeighbor finds the corresponding voxel in an adjacent shell
func (vp *VoxelPhysics) getRadialNeighbor(sourceShellIdx, targetShellIdx, latIdx, lonIdx int) *core.VoxelMaterial {
	if targetShellIdx < 0 || targetShellIdx >= len(vp.planet.Shells) {
		return nil
	}
	if sourceShellIdx < 0 || sourceShellIdx >= len(vp.planet.Shells) {
		return nil
	}

	sourceShell := &vp.planet.Shells[sourceShellIdx]
	targetShell := &vp.planet.Shells[targetShellIdx]

	// Map latitude index based on shell resolutions
	targetLat := latIdx * targetShell.LatBands / sourceShell.LatBands
	if targetLat >= targetShell.LatBands {
		targetLat = targetShell.LatBands - 1
	}

	// Map longitude accounting for different counts per latitude
	if latIdx >= len(sourceShell.Voxels) {
		return nil
	}
	sourceLonCount := len(sourceShell.Voxels[latIdx])
	targetLonCount := len(targetShell.Voxels[targetLat])
	targetLon := lonIdx * targetLonCount / sourceLonCount
	if targetLon >= targetLonCount {
		targetLon = targetLonCount - 1
	}

	return &targetShell.Voxels[targetLat][targetLon]
}

// GetAverageTemperature returns the average temperature at a given depth
func (vp *VoxelPhysics) GetAverageTemperature(shellIdx int) float32 {
	if shellIdx < 0 || shellIdx >= len(vp.planet.Shells) {
		return 0
	}

	shell := &vp.planet.Shells[shellIdx]
	sum := float64(0)
	count := 0

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			sum += float64(shell.Voxels[latIdx][lonIdx].Temperature)
			count++
		}
	}

	if count > 0 {
		return float32(sum / float64(count))
	}
	return 0
}

// countSurfaceVolcanism counts volcanic regions at the surface
func (vp *VoxelPhysics) countSurfaceVolcanism() int {
	count := 0

	// Check the outermost non-atmosphere shell
	surfaceShell := len(vp.planet.Shells) - 2
	if surfaceShell < 0 {
		return 0
	}

	shell := &vp.planet.Shells[surfaceShell]

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Check for magma or very hot rock with upward velocity
			if voxel.Type == core.MatMagma ||
				(voxel.Temperature > 1500 && voxel.VelR > 0) {
				count++
			}
		}
	}

	return count
}

// Release cleans up GPU resources
func (vp *VoxelPhysics) Release() {
	if vp.gpuCompute != nil {
		vp.gpuCompute.Release()
		vp.gpuCompute = nil
		vp.useGPU = false
	}
}
