package physics

import (
	"math"
	"worldgenerator/core"
)

// VoxelAdvection handles material movement and convection
type VoxelAdvection struct {
	planet  *core.VoxelPlanet
	physics *VoxelPhysics

	// Temporary buffers for advection
	tempMaterials [][]core.VoxelMaterial
}

// NewVoxelAdvection creates an advection simulator
func NewVoxelAdvection(planet *core.VoxelPlanet, physics *VoxelPhysics) *VoxelAdvection {
	return &VoxelAdvection{
		planet:  planet,
		physics: physics,
	}
}

// UpdateConvection calculates convection velocities based on temperature gradients
func (va *VoxelAdvection) UpdateConvection(dt float64) {
	g := 9.81 // gravity

	// Process each shell from bottom to top
	for shellIdx := 0; shellIdx < len(va.planet.Shells)-1; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		// Determine convection scale based on depth
		depthFraction := float64(shellIdx) / float64(len(va.planet.Shells))

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip air and water for now
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				// Get temperature gradient in radial direction
				outerVoxel := va.physics.getRadialNeighbor(shellIdx, shellIdx+1, latIdx, lonIdx)
				if outerVoxel == nil {
					continue
				}

				// Calculate buoyancy force
				// Thermal expansion coefficient for rock ~ 3e-5 /K
				alpha := 3e-5
				deltaT := float64(voxel.Temperature - outerVoxel.Temperature)

				// Density difference due to temperature
				deltaDensity := float64(voxel.Density) * alpha * deltaT

				// Buoyancy force: F = -Δρ * g
				buoyancyForce := -deltaDensity * g

				// Continental crust buoyancy
				// Granite is fundamentally less dense than basalt/peridotite
				// This prevents continental crust from subducting
				if voxel.Type == core.MatGranite {
					// Continental crust is ~2700 kg/m³ vs oceanic ~2900 kg/m³
					// This creates persistent upward force
					avgDensity := float64(outerVoxel.Density)
					if outerVoxel.Type == core.MatBasalt || outerVoxel.Type == core.MatPeridotite {
						avgDensity = 2900.0 // Reference oceanic density
					}
					compositionalBuoyancy := (avgDensity - float64(voxel.Density)) * g / 100.0
					buoyancyForce += compositionalBuoyancy
				}

				// Get material viscosity
				viscosity := va.getViscosity(voxel)

				// Stokes velocity: v = F * r² / (6πμ)
				// Using characteristic length scale
				lengthScale := (shell.OuterRadius - shell.InnerRadius) / 10.0
				velocity := buoyancyForce * lengthScale * lengthScale / (6.0 * math.Pi * viscosity)

				// Apply Rayleigh number criterion for convection
				// Ra = (ρgαΔT L³) / (κμ)
				thermalDiff := 1e-6 // thermal diffusivity
				rayleigh := math.Abs(deltaDensity * g * lengthScale * lengthScale * lengthScale /
					(thermalDiff * viscosity))

				// Critical Rayleigh number for convection onset
				if rayleigh > 1000 {
					// Set radial velocity
					voxel.VelR = float32(velocity * dt)

					// Multi-scale lateral circulation
					// Large scale cells (whole mantle convection)
					largeScale := math.Sin(float64(latIdx)*0.1) * math.Cos(float64(lonIdx)*0.15)

					// Medium scale cells (upper/lower mantle)
					mediumScale := 0.0
					if depthFraction > 0.3 && depthFraction < 0.7 {
						mediumScale = math.Sin(float64(latIdx)*0.3) * math.Cos(float64(lonIdx)*0.4)
					}

					// Small scale cells (near surface)
					smallScale := 0.0
					if depthFraction > 0.7 {
						smallScale = math.Sin(float64(latIdx)*0.6) * math.Cos(float64(lonIdx)*0.8)
					}

					// Combine scales with depth-dependent weighting
					cellPattern := largeScale*(1.0-depthFraction) +
						mediumScale*0.5 +
						smallScale*depthFraction*0.3

					voxel.VelTheta = float32(velocity * 0.1 * cellPattern * dt)
					voxel.VelPhi = float32(velocity * 0.1 * math.Cos(cellPattern) * dt)
				} else {
					// No convection - decay velocities
					voxel.VelR *= 0.95
					voxel.VelTheta *= 0.95
					voxel.VelPhi *= 0.95
				}
			}
		}
	}
}

// getViscosity returns material viscosity based on temperature and pressure
func (va *VoxelAdvection) getViscosity(voxel *core.VoxelMaterial) float64 {
	// Simplified Arrhenius law for rock viscosity
	// μ = μ₀ * exp(E/(RT))

	baseViscosity := 1e21        // Pa·s at reference conditions
	activationEnergy := 300000.0 // J/mol
	gasConstant := 8.314         // J/(mol·K)

	// Temperature dependence
	viscosity := baseViscosity * math.Exp(activationEnergy/
		(gasConstant*float64(voxel.Temperature)))

	// Pressure dependence (simplified)
	// Higher pressure increases viscosity
	pressureFactor := 1.0 + float64(voxel.Pressure-101325)/1e9
	viscosity *= pressureFactor

	// Material-specific adjustments
	switch voxel.Type {
	case core.MatMagma:
		viscosity *= 1e-15 // Magma is much less viscous
	case core.MatWater:
		viscosity = 1e-3 // Water viscosity
	case core.MatAir:
		viscosity = 1.8e-5 // Air viscosity
	}

	// Clamp to reasonable range
	if viscosity > 1e25 {
		viscosity = 1e25
	}
	if viscosity < 1e-5 {
		viscosity = 1e-5
	}

	return viscosity
}

// AdvectMaterial moves material based on velocity field
func (va *VoxelAdvection) AdvectMaterial(dt float64) {
	// Simple advection for demo purposes
	// Move surface materials based on their velocities
	va.advectSurfacePlates(dt)

	// Original upwelling/downwelling code follows
	// Full advection would require solving transport equations

	// Process shells from bottom to top for upwelling
	for shellIdx := 0; shellIdx < len(va.planet.Shells)-1; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Check for significant upward velocity
				if voxel.VelR > 0.1 {
					// Find corresponding upper voxel
					upperVoxel := va.physics.getRadialNeighbor(shellIdx, shellIdx+1, latIdx, lonIdx)
					if upperVoxel == nil {
						continue
					}

					// Transfer some properties upward (simplified)
					// In reality, this would be mass-conserving flux
					mixFactor := float32(0.001) // Small mixing per timestep

					// Mix temperatures
					tempDiff := voxel.Temperature - upperVoxel.Temperature
					upperVoxel.Temperature += tempDiff * mixFactor
					voxel.Temperature -= tempDiff * mixFactor * 0.1

					// Transfer composition for magma
					if voxel.Type == core.MatMagma && upperVoxel.Type != core.MatAir {
						upperVoxel.Composition = (upperVoxel.Composition + voxel.Composition*mixFactor) /
							(1.0 + mixFactor)
					}
				}
			}
		}
	}

	// Mark planet mesh as dirty
	va.planet.MeshDirty = true
}

// InitializeConvectionCells sets up initial convection patterns
func (va *VoxelAdvection) InitializeConvectionCells() {
	// Create some initial temperature perturbations to kick-start convection
	for shellIdx := 1; shellIdx < len(va.planet.Shells)/2; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Add small temperature perturbations
				// This creates initial instabilities that grow into convection cells
				perturbation := math.Sin(float64(latIdx)*0.1) *
					math.Cos(float64(lonIdx)*0.15) * 50.0
				voxel.Temperature += float32(perturbation)
			}
		}
	}

	// Initialize mantle plumes at core-mantle boundary
	va.initializeMantlePlumes()
}

// initializeMantlePlumes creates hot spots at the core-mantle boundary
func (va *VoxelAdvection) initializeMantlePlumes() {
	// Create 3-5 major plume locations
	plumeLocs := []struct{ lat, lon float64 }{
		{30, 45},   // Pacific hotspot
		{-20, 160}, // Another Pacific
		{45, -30},  // Atlantic
		{-40, 80},  // Indian Ocean
		{0, -120},  // East Pacific
	}

	// Core-mantle boundary is around shell 1-2
	for i := 0; i < 3 && i < len(va.planet.Shells); i++ {
		shell := &va.planet.Shells[i]

		for _, plume := range plumeLocs {
			// Find nearest voxel to plume location
			targetLat := int((plume.lat + 90.0) * float64(shell.LatBands) / 180.0)
			targetLon := int((plume.lon + 180.0) * float64(shell.LonCounts[targetLat]) / 360.0)

			if targetLat >= 0 && targetLat < shell.LatBands {
				// Heat up a region around the plume center
				for dLat := -2; dLat <= 2; dLat++ {
					latIdx := targetLat + dLat
					if latIdx < 0 || latIdx >= shell.LatBands {
						continue
					}

					for dLon := -3; dLon <= 3; dLon++ {
						lonIdx := (targetLon + dLon + shell.LonCounts[latIdx]) % shell.LonCounts[latIdx]

						// Distance from plume center
						dist := math.Sqrt(float64(dLat*dLat + dLon*dLon))
						heatBoost := 500.0 * math.Exp(-dist*dist/4.0) // Gaussian profile

						shell.Voxels[latIdx][lonIdx].Temperature += float32(heatBoost)
					}
				}
			}
		}
	}
}

// UpdateMantlePlumes maintains and evolves mantle plumes
func (va *VoxelAdvection) UpdateMantlePlumes(dt float64) {
	// Find hot regions at core-mantle boundary
	if len(va.planet.Shells) < 2 {
		return
	}

	// Check bottom shells for hot spots
	for shellIdx := 0; shellIdx < 3 && shellIdx < len(va.planet.Shells); shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Check if this is a hot spot (significantly hotter than average)
				if voxel.Temperature > 5500 { // Very hot for deep mantle
					// Enhance upward velocity for plume
					voxel.VelR = float32(math.Max(float64(voxel.VelR), 0.001*dt))

					// Create plume head expansion
					if shellIdx > 0 {
						// Reduce lateral velocities to focus upward flow
						voxel.VelTheta *= 0.5
						voxel.VelPhi *= 0.5
					}
				}
			}
		}
	}
}

// UpdateSlabPull simulates dense oceanic crust sinking into the mantle
func (va *VoxelAdvection) UpdateSlabPull(dt float64) {
	// Check upper shells for dense, cold material
	startShell := len(va.planet.Shells) * 3 / 4 // Upper quarter of planet

	for shellIdx := startShell; shellIdx < len(va.planet.Shells)-1; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip air and water
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				// Check for cold, dense oceanic crust (basalt)
				// Continental crust (granite) resists subduction due to buoyancy
				if voxel.Type == core.MatBasalt && voxel.Temperature < 800 {
					// Get material below
					innerVoxel := va.physics.getRadialNeighbor(shellIdx, shellIdx-1, latIdx, lonIdx)
					if innerVoxel == nil {
						continue
					}

					// Check if denser than material below (negative buoyancy)
					if voxel.Density > innerVoxel.Density*1.05 { // 5% denser
						// Apply slab pull - enhance downward velocity
						pullVelocity := 0.0001 * dt * float64(voxel.Density-innerVoxel.Density) / float64(voxel.Density)
						voxel.VelR = float32(math.Min(float64(voxel.VelR)-pullVelocity, -0.0001*dt))

						// Create lateral flow away from subduction
						// This simulates trench rollback
						voxel.VelTheta *= 1.2
						voxel.VelPhi *= 1.2

						// Mark as subducting by cooling it further
						voxel.Temperature *= 0.99
					}
				} else if voxel.Type == core.MatGranite && voxel.Temperature < 800 {
					// Continental crust resists subduction
					// Instead, it crumples and thickens (mountain building)

					// Check for convergent motion
					eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
					eastVoxel := &shell.Voxels[latIdx][eastLon]

					if eastVoxel.VelPhi < voxel.VelPhi-1e-6 {
						// Convergence detected - resist downward motion
						voxel.VelR = float32(math.Max(float64(voxel.VelR), 0))

						// Thicken crust (simplified - increase local elevation)
						// In reality this would deform multiple voxels
						voxel.VelR += float32(0.00001 * dt) // Slight uplift
					}
				}
			}
		}
	}
}

// DetectSubductionZones identifies areas where slabs are descending
func (va *VoxelAdvection) DetectSubductionZones() []SubductionZone {
	zones := []SubductionZone{}

	// Look for regions with significant downward velocity
	surfaceShell := len(va.planet.Shells) - 2
	if surfaceShell < 0 {
		return zones
	}

	shell := &va.planet.Shells[surfaceShell]

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Strong downward velocity indicates subduction
			if voxel.VelR < -0.0001 && voxel.Type == core.MatBasalt {
				zone := SubductionZone{
					LatIdx: latIdx,
					LonIdx: lonIdx,
					Depth:  float64(voxel.VelR) * -1000, // Rough depth estimate
				}
				zones = append(zones, zone)
			}
		}
	}

	return zones
}

// SubductionZone represents an active subduction region
type SubductionZone struct {
	LatIdx int
	LonIdx int
	Depth  float64
}

// advectSurfacePlates moves surface materials based on plate velocities
func (va *VoxelAdvection) advectSurfacePlates(dt float64) {
	// Work on surface shell
	surfaceShell := len(va.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &va.planet.Shells[surfaceShell]

	// SIMPLE DEMO: Shift continents eastward over time
	// This is not physically accurate but will show visible motion

	// Calculate how many grid cells to shift based on time
	// At 100 Myr/s, shift 1 cell every 10 million years
	yearsElapsed := dt / (365.25 * 24 * 3600)
	cellsToShift := int(va.planet.Time / 10000000) // 1 cell per 10 My

	// Create new voxel array
	newVoxels := make([][]core.VoxelMaterial, len(shell.Voxels))
	for latIdx := range shell.Voxels {
		newVoxels[latIdx] = make([]core.VoxelMaterial, len(shell.Voxels[latIdx]))

		for lonIdx := range shell.Voxels[latIdx] {
			// Calculate source position (shift westward to move content eastward)
			sourceLon := (lonIdx - cellsToShift + 1000*len(shell.Voxels[latIdx])) % len(shell.Voxels[latIdx])

			// Copy from source
			newVoxels[latIdx][lonIdx] = shell.Voxels[latIdx][sourceLon]

			// Update age
			newVoxels[latIdx][lonIdx].Age += float32(yearsElapsed)
		}
	}

	// Replace old voxels
	shell.Voxels = newVoxels
}
