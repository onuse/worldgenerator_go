package physics

import (
	"fmt"
	"math"
	"time"
	"worldgenerator/core"
)

// VoxelAdvection handles material movement and convection
type VoxelAdvection struct {
	planet    *core.VoxelPlanet
	physics   *VoxelPhysics
	waterFlow *WaterFlow

	// Temporary buffers for advection
	tempMaterials [][]core.VoxelMaterial

	// Tracking for sea level changes
	lastReportedSeaLevel float64

	// Timing for debug output
	lastAdvectionReport time.Time

	// Water flow timing
	lastWaterFlowUpdate float64
}

// NewVoxelAdvection creates an advection simulator
func NewVoxelAdvection(planet *core.VoxelPlanet, physics *VoxelPhysics) *VoxelAdvection {
	return &VoxelAdvection{
		planet:    planet,
		physics:   physics,
		waterFlow: NewWaterFlow(planet),
	}
}

// UpdateConvection calculates convection velocities based on temperature gradients
func (va *VoxelAdvection) UpdateConvection(dt float64) {
	g := 9.81 // gravity

	// Process each shell from bottom to top
	for shellIdx := 0; shellIdx < len(va.planet.Shells)-1; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]

		// Determine convection scale based on depth
		// depthFraction := float64(shellIdx) / float64(len(va.planet.Shells))

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
					// largeScale := math.Sin(float64(latIdx)*0.1) * math.Cos(float64(lonIdx)*0.15)

					// Medium scale cells (upper/lower mantle)
					// mediumScale := 0.0
					// if depthFraction > 0.3 && depthFraction < 0.7 {
					// 	mediumScale = math.Sin(float64(latIdx)*0.3) * math.Cos(float64(lonIdx)*0.4)
					// }

					// Small scale cells (near surface)
					// smallScale := 0.0
					// if depthFraction > 0.7 {
					// 	smallScale = math.Sin(float64(latIdx)*0.6) * math.Cos(float64(lonIdx)*0.8)
					// }

					// Combine scales with depth-dependent weighting
					// cellPattern := largeScale*(1.0-depthFraction) +
					// 	mediumScale*0.5 +
					// 	smallScale*depthFraction*0.3

					// IMPORTANT: Only affect radial velocity for convection
					// Horizontal velocities are controlled by plate tectonics
					// voxel.VelNorth = float32(velocity * 0.1 * cellPattern * dt)
					// voxel.VelEast = float32(velocity * 0.1 * math.Cos(cellPattern) * dt)
				} else {
					// No convection - only decay radial velocity
					// Horizontal velocities are preserved for plate motion
					voxel.VelR *= 0.95
					// voxel.VelNorth *= 0.95
					// voxel.VelEast *= 0.95
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
						voxel.VelNorth *= 0.5
						voxel.VelEast *= 0.5
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
						voxel.VelNorth *= 1.2
						voxel.VelEast *= 1.2

						// Mark as subducting by cooling it further
						voxel.Temperature *= 0.99
					}
				} else if voxel.Type == core.MatGranite && voxel.Temperature < 800 {
					// Continental crust resists subduction
					// Instead, it crumples and thickens (mountain building)

					// Check for convergent motion
					eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
					eastVoxel := &shell.Voxels[latIdx][eastLon]

					if eastVoxel.VelEast < voxel.VelEast-1e-6 {
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

// interpolateBoundaryProperties smooths material properties at plate boundaries
func (va *VoxelAdvection) interpolateBoundaryProperties(shell *core.SphericalShell) {
	// For each voxel, check if it's at a plate boundary
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip non-crustal material
			if voxel.Type != core.MatGranite && voxel.Type != core.MatBasalt {
				continue
			}

			// Check if this is a boundary voxel
			isBoundary := false
			neighborPlates := make(map[int32]int)
			totalTemp := float32(0)
			totalStress := float32(0)
			neighborCount := 0

			// Check all 8 neighbors
			for dlat := -1; dlat <= 1; dlat++ {
				for dlon := -1; dlon <= 1; dlon++ {
					if dlat == 0 && dlon == 0 {
						continue
					}

					nlat := latIdx + dlat
					nlon := (lonIdx + dlon + len(shell.Voxels[latIdx])) % len(shell.Voxels[latIdx])

					if nlat >= 0 && nlat < len(shell.Voxels) && nlon < len(shell.Voxels[nlat]) {
						neighbor := &shell.Voxels[nlat][nlon]
						if neighbor.Type == core.MatGranite || neighbor.Type == core.MatBasalt {
							neighborPlates[neighbor.PlateID]++
							totalTemp += neighbor.Temperature
							totalStress += neighbor.Stress
							neighborCount++

							if neighbor.PlateID != voxel.PlateID {
								isBoundary = true
							}
						}
					}
				}
			}

			// Smooth properties at boundaries
			if isBoundary && neighborCount > 0 {
				// Blend temperature slightly
				avgTemp := totalTemp / float32(neighborCount)
				voxel.Temperature = voxel.Temperature*0.7 + avgTemp*0.3

				// Average stress at boundaries
				avgStress := totalStress / float32(neighborCount)
				voxel.Stress = (voxel.Stress + avgStress) / 2

				// Mark as boundary for visualization
				voxel.IsFractured = true
			}
		}
	}
}

// advectSurfacePlates moves surface materials based on plate velocities
func (va *VoxelAdvection) advectSurfacePlates(dt float64) {
	// Check if virtual voxel system is enabled
	if va.planet.VirtualVoxelSystem != nil && va.planet.UseVirtualVoxels {
		// Use virtual voxel physics instead of grid-based movement
		// Skip if GPU is handling physics (UseGPU flag is set by renderer)
		if !va.planet.VirtualVoxelSystem.UseGPU {
			va.planet.VirtualVoxelSystem.UpdatePhysics(float32(dt))
			va.planet.VirtualVoxelSystem.MapToGrid()
		}
		return
	}

	// Original grid-based implementation with fractional tracking
	// Work on surface shells (can handle multiple for vertical movement)
	surfaceShell := len(va.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &va.planet.Shells[surfaceShell]
	radius := (shell.InnerRadius + shell.OuterRadius) / 2
	currentTime := float32(va.planet.Time)

	// Phase 1: Update sub-cell positions for smooth movement
	for latIdx := range shell.Voxels {
		lat := core.GetLatitudeForBand(latIdx, shell.LatBands)
		cosLat := math.Cos(lat * math.Pi / 180.0)
		if math.Abs(cosLat) < 0.01 {
			cosLat = 0.01 // Avoid division by zero near poles
		}

		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip air but allow water to move
			if voxel.Type == core.MatAir {
				continue
			}

			// Water moves differently - it flows to fill gaps
			if voxel.Type == core.MatWater {
				// Water doesn't have plate velocities, skip velocity update
				continue
			}

			// Convert velocity (m/s) to grid cells per timestep
			circumference := 2.0 * math.Pi * radius * cosLat
			cellsPerMeter := float64(len(shell.Voxels[latIdx])) / circumference
			latCircumference := 2.0 * math.Pi * radius

			// Update sub-cell position with smooth movement
			// SubPosLon/Lat are in [0,1) within the current cell
			lonMovement := float32(float64(voxel.VelEast) * dt * cellsPerMeter)
			latMovement := float32(float64(voxel.VelNorth) * dt * float64(shell.LatBands) / latCircumference)

			// Add movement to sub-position
			voxel.SubPosLon += lonMovement
			voxel.SubPosLat += latMovement

			// Also accumulate in fractional fields for compatibility
			voxel.FracLon += lonMovement
			voxel.FracLat += latMovement

			// Update vertical position and elevation
			if voxel.VelR != 0 {
				// Convert velocity (m/s) to elevation change
				elevationChange := voxel.VelR * float32(dt)
				voxel.Elevation += elevationChange

				// Also update sub-position within shell
				shellThickness := float32(shell.OuterRadius - shell.InnerRadius)
				if shellThickness > 0 {
					voxel.SubPosR += elevationChange / shellThickness
				}
			}
		}
	}

	// Phase 2: Handle cell boundary transitions based on sub-position
	type voxelMove struct {
		voxel      *core.VoxelMaterial
		sourceLat  int
		sourceLon  int
		targetLat  int
		targetLon  int
		intLonMove int // Integer cells to move
		intLatMove int
	}

	var movements []voxelMove

	// Collect voxels that need to move to a new cell
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip non-crustal material
			if voxel.Type != core.MatGranite && voxel.Type != core.MatBasalt {
				continue
			}

			// Check if sub-position has crossed cell boundaries
			intLonMove := 0
			intLatMove := 0

			// Handle longitude boundary crossing
			if voxel.SubPosLon >= 1.0 {
				intLonMove = int(voxel.SubPosLon)
				voxel.SubPosLon -= float32(intLonMove) // Keep fractional part
			} else if voxel.SubPosLon < 0.0 {
				intLonMove = int(voxel.SubPosLon) - 1
				voxel.SubPosLon -= float32(intLonMove) // Keep fractional part
			}

			// Handle latitude boundary crossing
			if voxel.SubPosLat >= 1.0 {
				intLatMove = int(voxel.SubPosLat)
				voxel.SubPosLat -= float32(intLatMove) // Keep fractional part
			} else if voxel.SubPosLat < 0.0 {
				intLatMove = int(voxel.SubPosLat) - 1
				voxel.SubPosLat -= float32(intLatMove) // Keep fractional part
			}

			// Also update fractional fields for compatibility
			if voxel.FracLon >= 1.0 {
				voxel.FracLon -= float32(int(voxel.FracLon))
			} else if voxel.FracLon <= -1.0 {
				voxel.FracLon -= float32(int(voxel.FracLon))
			}

			if voxel.FracLat >= 1.0 {
				voxel.FracLat -= float32(int(voxel.FracLat))
			} else if voxel.FracLat <= -1.0 {
				voxel.FracLat -= float32(int(voxel.FracLat))
			}

			if intLonMove != 0 || intLatMove != 0 {
				// This voxel needs to move to a new cell
				targetLon := lonIdx + intLonMove
				targetLat := latIdx + intLatMove

				// Handle longitude wrapping
				numLons := len(shell.Voxels[latIdx])
				targetLon = ((targetLon % numLons) + numLons) % numLons

				movements = append(movements, voxelMove{
					voxel:      voxel,
					sourceLat:  latIdx,
					sourceLon:  lonIdx,
					targetLat:  targetLat,
					targetLon:  targetLon,
					intLonMove: intLonMove,
					intLatMove: intLatMove,
				})
			}
		}
	}

	// Phase 3: Create new voxel array starting with current state
	newVoxels := make([][]core.VoxelMaterial, shell.LatBands)
	for latIdx := range newVoxels {
		newVoxels[latIdx] = make([]core.VoxelMaterial, len(shell.Voxels[latIdx]))
		// Copy current state
		copy(newVoxels[latIdx], shell.Voxels[latIdx])
	}

	// Phase 4: Apply movements
	// Early exit if no movements
	if len(movements) == 0 {
		// No movement needed - keep current state
		return
	}

	// Debug output - only show when significant movement occurs and every 5 seconds
	if len(movements) > 100 && time.Since(va.lastAdvectionReport).Seconds() > 5.0 {
		yearsElapsed := dt / (365.25 * 24 * 3600)

		// Count voxels with velocity
		velCount := 0
		maxVel := float32(0.0)
		for _, move := range movements {
			vel := float32(math.Sqrt(float64(move.voxel.VelNorth*move.voxel.VelNorth + move.voxel.VelEast*move.voxel.VelEast)))
			if vel > 0 {
				velCount++
				if vel > maxVel {
					maxVel = vel
				}
			}
		}

		fmt.Printf("ADVECTION: %d voxels moving (%.1f years, %.1f My total). %d have velocity, max=%.2e m/s\n",
			len(movements), yearsElapsed, va.planet.Time/1e6, velCount, maxVel)
		va.lastAdvectionReport = time.Now()
	}

	for _, move := range movements {
		// Check latitude bounds
		if move.targetLat < 0 || move.targetLat >= shell.LatBands {
			// Voxel moves off grid - skip
			continue
		}

		// Clear source location (will be filled later if needed)
		newVoxels[move.sourceLat][move.sourceLon] = core.VoxelMaterial{
			Type:          core.MatWater,
			Density:       core.MaterialProperties[core.MatWater].DefaultDensity,
			Temperature:   shell.Voxels[move.sourceLat][move.sourceLon].Temperature,
			PlateID:       0,
			StretchFactor: 1.0,
			Age:           shell.Voxels[move.sourceLat][move.sourceLon].Age,
			// All other fields are zero-initialized
		}

		// Place at target
		if move.targetLon < len(newVoxels[move.targetLat]) {
			target := &newVoxels[move.targetLat][move.targetLon]

			// Update timestamp
			move.voxel.LastMoveTime = currentTime

			if target.Type == core.MatWater || target.Type == core.MatAir {
				// Simple case - move to empty space
				// The sub-positions have already been adjusted in Phase 2
				*target = *move.voxel
			} else if target.Type == core.MatGranite || target.Type == core.MatBasalt {
				// Collision! Handle based on material types and velocities
				collisionStress := float32(1e7)

				// Different collision behaviors based on materials
				if move.voxel.Type == core.MatBasalt && target.Type == core.MatGranite {
					// Oceanic crust hitting continental - prepare for subduction
					// Mark both with high stress but different behaviors
					target.Stress += collisionStress * 0.5     // Continental crust resists more
					move.voxel.Stress += collisionStress * 1.5 // Oceanic crust deforms more

					// Mark oceanic crust for potential downward movement
					move.voxel.VelR = -0.001 // Small downward velocity

					// Track compression
					move.voxel.StretchFactor = 0.8 // Compressed
					target.StretchFactor = 0.9     // Slightly compressed
				} else if move.voxel.Type == core.MatGranite && target.Type == core.MatGranite {
					// Continental collision - both resist equally
					target.Stress += collisionStress
					move.voxel.Stress += collisionStress

					// Both compress and may buckle upward
					move.voxel.StretchFactor = 0.85
					target.StretchFactor = 0.85

					// Small upward velocity for mountain building
					if move.voxel.Stress > 5e7 {
						move.voxel.VelR = 0.0005
					}
				} else {
					// Default collision
					target.Stress += collisionStress
					move.voxel.Stress += collisionStress
				}

				// The moving voxel stays at source with high stress
				// Reset sub-positions since it couldn't move
				move.voxel.SubPosLon += float32(move.intLonMove)
				move.voxel.SubPosLat += float32(move.intLatMove)
				move.voxel.FracLon += float32(move.intLonMove)
				move.voxel.FracLat += float32(move.intLatMove)
				newVoxels[move.sourceLat][move.sourceLon] = *move.voxel
			}
		}
	}

	// === CONTINENT/PLATE MOVEMENT PASS ===
	// Phase 5: Fill gaps in plates with transient voxels (maintains plate continuity)
	va.fillPlateGaps(&newVoxels, shell)

	// Replace old voxels
	shell.Voxels = newVoxels

	// Phase 6: Smooth properties at boundaries for better continuity
	va.interpolateBoundaryProperties(shell)

	// Phase 7: Handle shell-to-shell movement for subduction and rising
	va.handleShellToShellMovement(dt, surfaceShell)

	// === WATER PROCESSES PASS ===
	// Now that all plate movements are complete, handle water

	// Phase 8: Fill ocean gaps where continents have moved away
	va.fillOceanGaps(&shell.Voxels, shell)

	// Phase 9: Realistic water flow physics (disabled material type changes)
	// Only update water flow every 100 years to prevent oscillation
	if va.planet.Time-va.lastWaterFlowUpdate > 100.0 {
		va.waterFlow.UpdateFlow(float32(dt))
		va.applyCoastalErosion(shell)
		va.lastWaterFlowUpdate = va.planet.Time
	}

	// Phase 10: Update sea level to maintain water conservation
	va.planet.UpdateSeaLevel()
	va.applySeaLevelChange(shell)
}

// fillOceanGaps fills air gaps with ocean water where continents have moved away
func (va *VoxelAdvection) fillOceanGaps(newVoxels *[][]core.VoxelMaterial, shell *core.SphericalShell) {
	// Find air voxels that should be ocean (below sea level and surrounded by water)
	for latIdx := range *newVoxels {
		for lonIdx := range (*newVoxels)[latIdx] {
			voxel := &(*newVoxels)[latIdx][lonIdx]

			// Only process air voxels below sea level
			if voxel.Type != core.MatAir || voxel.Elevation > float32(va.planet.SeaLevel) {
				continue
			}

			// Count water neighbors
			waterNeighbors := 0
			totalNeighbors := 0

			// Check all 8 neighbors
			for dlat := -1; dlat <= 1; dlat++ {
				for dlon := -1; dlon <= 1; dlon++ {
					if dlat == 0 && dlon == 0 {
						continue
					}

					nlat := latIdx + dlat
					nlon := (lonIdx + dlon + len((*newVoxels)[latIdx])) % len((*newVoxels)[latIdx])

					if nlat >= 0 && nlat < len(*newVoxels) && nlon < len((*newVoxels)[nlat]) {
						neighbor := &(*newVoxels)[nlat][nlon]
						totalNeighbors++
						if neighbor.Type == core.MatWater {
							waterNeighbors++
						}
					}
				}
			}

			// If mostly surrounded by water and below sea level, fill with ocean
			if waterNeighbors > 0 && float64(waterNeighbors) >= float64(totalNeighbors)*0.5 {
				voxel.Type = core.MatWater
				voxel.Density = core.MaterialProperties[core.MatWater].DefaultDensity
				voxel.Temperature = 288.15 // Ocean temperature
				voxel.PlateID = 0          // Water has no plate
				voxel.VelR = 0
				voxel.VelNorth = 0
				voxel.VelEast = 0
				voxel.WaterVolume = 1.0 // Full water
				voxel.Age = 0
				voxel.IsTransient = true
			}
		}
	}
}

// flowWaterIntoGaps allows water to flow into adjacent empty spaces
func (va *VoxelAdvection) flowWaterIntoGaps(shell *core.SphericalShell) {
	// DISABLED: This old water flow system conflicts with the new realistic water flow physics
	// and causes continent blinking. Water flow is now handled by WaterFlow struct.
	return
	// Create a copy to avoid modifying while iterating
	waterFlows := make([]struct {
		fromLat, fromLon int
		toLat, toLon     int
	}, 0)

	// Find water voxels adjacent to gaps
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only process water voxels
			if voxel.Type != core.MatWater {
				continue
			}

			// Check all 4 direct neighbors
			neighbors := []struct{ dlat, dlon int }{
				{-1, 0}, {1, 0}, {0, -1}, {0, 1},
			}

			for _, n := range neighbors {
				nlat := latIdx + n.dlat
				nlon := (lonIdx + n.dlon + len(shell.Voxels[latIdx])) % len(shell.Voxels[latIdx])

				// Check bounds
				if nlat < 0 || nlat >= len(shell.Voxels) {
					continue
				}
				if nlon >= len(shell.Voxels[nlat]) {
					continue
				}

				neighbor := &shell.Voxels[nlat][nlon]

				// Check if neighbor is a gap that should be water
				// This happens when continents move and leave behind empty space
				if neighbor.Type == core.MatAir && neighbor.Elevation < 0 {
					// This should be ocean
					waterFlows = append(waterFlows, struct {
						fromLat, fromLon int
						toLat, toLon     int
					}{
						fromLat: latIdx,
						fromLon: lonIdx,
						toLat:   nlat,
						toLon:   nlon,
					})
				}
			}
		}
	}

	// Apply water flows
	for _, flow := range waterFlows {
		source := &shell.Voxels[flow.fromLat][flow.fromLon]
		target := &shell.Voxels[flow.toLat][flow.toLon]

		// Convert air to water
		if target.Type == core.MatAir {
			target.Type = core.MatWater
			target.Density = core.MaterialProperties[core.MatWater].DefaultDensity
			target.Temperature = source.Temperature // Inherit temperature
			target.PlateID = 0                      // Water has no plate
			target.Elevation = -1000                // Default ocean depth
			// Clear velocities
			target.VelR = 0
			target.VelNorth = 0
			target.VelEast = 0
		}
	}
}

// applyCoastalErosion handles realistic water-land interactions
func (va *VoxelAdvection) applyCoastalErosion(shell *core.SphericalShell) {
	// Track changes to apply after iteration
	erosionChanges := make([]struct {
		lat, lon     int
		newType      core.MaterialType
		newElevation float32
	}, 0)

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only process land near sea level
			if voxel.Type != core.MatGranite && voxel.Type != core.MatBasalt {
				continue
			}

			// Low-lying land is vulnerable to flooding
			if voxel.Elevation < 10 && voxel.Elevation > -50 {
				// Count adjacent water cells
				waterNeighbors := 0
				totalNeighbors := 0

				// Check all 8 neighbors
				for dlat := -1; dlat <= 1; dlat++ {
					for dlon := -1; dlon <= 1; dlon++ {
						if dlat == 0 && dlon == 0 {
							continue
						}

						nlat := latIdx + dlat
						nlon := (lonIdx + dlon + len(shell.Voxels[latIdx])) % len(shell.Voxels[latIdx])

						if nlat >= 0 && nlat < len(shell.Voxels) && nlon < len(shell.Voxels[nlat]) {
							neighbor := &shell.Voxels[nlat][nlon]
							totalNeighbors++
							if neighbor.Type == core.MatWater {
								waterNeighbors++
							}
						}
					}
				}

				// If mostly surrounded by water and low elevation, convert to water
				if waterNeighbors >= 5 && totalNeighbors >= 6 {
					// Coastal erosion or flooding
					erosionChanges = append(erosionChanges, struct {
						lat, lon     int
						newType      core.MaterialType
						newElevation float32
					}{
						lat:          latIdx,
						lon:          lonIdx,
						newType:      core.MatWater,
						newElevation: -100, // Shallow water
					})
				}
			}

			// Also handle submerged land (negative elevation land should become ocean)
			if voxel.Elevation < -200 {
				erosionChanges = append(erosionChanges, struct {
					lat, lon     int
					newType      core.MaterialType
					newElevation float32
				}{
					lat:          latIdx,
					lon:          lonIdx,
					newType:      core.MatWater,
					newElevation: voxel.Elevation, // Keep depth
				})
			}
		}
	}

	// Apply erosion changes
	for _, change := range erosionChanges {
		voxel := &shell.Voxels[change.lat][change.lon]
		if voxel.Type != core.MatWater { // Don't re-process if already changed
			voxel.Type = change.newType
			voxel.Density = core.MaterialProperties[change.newType].DefaultDensity
			voxel.Elevation = change.newElevation
			voxel.PlateID = 0 // Water has no plate
			// Clear land-specific properties
			voxel.Stress = 0
			voxel.IsBrittle = false
			voxel.StretchFactor = 1.0
		}
	}
}

// applySeaLevelChange floods or exposes land based on the new sea level
func (va *VoxelAdvection) applySeaLevelChange(shell *core.SphericalShell) {
	// DISABLED: Changing material types based on sea level causes continent blinking
	// Sea level should affect rendering but not change land to water instantly
	return

	seaLevel := float32(va.planet.SeaLevel)

	// Track changes
	changes := make([]struct {
		lat, lon int
		newType  core.MaterialType
		reason   string
	}, 0)

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Land below sea level becomes ocean
			if (voxel.Type == core.MatGranite || voxel.Type == core.MatBasalt) &&
				voxel.Elevation < seaLevel {
				// This land is now underwater
				changes = append(changes, struct {
					lat, lon int
					newType  core.MaterialType
					reason   string
				}{
					lat:     latIdx,
					lon:     lonIdx,
					newType: core.MatWater,
					reason:  "flooded",
				})
			}

			// Ocean floor above sea level becomes exposed land (rare but possible)
			if voxel.Type == core.MatWater && voxel.Elevation > seaLevel {
				// This ocean floor is now exposed - becomes sediment/sand
				changes = append(changes, struct {
					lat, lon int
					newType  core.MaterialType
					reason   string
				}{
					lat:     latIdx,
					lon:     lonIdx,
					newType: core.MatSediment,
					reason:  "exposed",
				})
			}
		}
	}

	// Apply changes
	for _, change := range changes {
		voxel := &shell.Voxels[change.lat][change.lon]

		voxel.Type = change.newType
		voxel.Density = core.MaterialProperties[change.newType].DefaultDensity

		if change.newType == core.MatWater {
			// Land flooded by rising sea level
			voxel.PlateID = 0
			voxel.Stress = 0
			voxel.IsBrittle = false
			voxel.StretchFactor = 1.0
			// Keep elevation (now ocean floor depth)
		} else if change.newType == core.MatSediment {
			// Exposed ocean floor
			voxel.PlateID = 0 // No plate initially
			voxel.IsBrittle = true
			voxel.Age = 0 // Fresh sediment
			// Keep elevation (now land elevation)
		}
	}

	// Report major sea level changes
	if math.Abs(va.planet.SeaLevel-va.lastReportedSeaLevel) > 10 { // Report 10m+ changes
		fmt.Printf("SEA LEVEL CHANGE: %.1fm, %d cells affected\n",
			va.planet.SeaLevel, len(changes))
		va.lastReportedSeaLevel = va.planet.SeaLevel
	}
}

// verticalMove tracks voxel movement between shells
type verticalMove struct {
	voxel                *core.VoxelMaterial
	sourceLat, sourceLon int
	targetShell          int
}

// handleShellToShellMovement moves voxels between shells based on vertical velocity
func (va *VoxelAdvection) handleShellToShellMovement(dt float64, surfaceShell int) {
	// Work from bottom to top for rising material, top to bottom for sinking

	// First pass: Handle sinking material (subduction)
	for shellIdx := surfaceShell; shellIdx > 0; shellIdx-- {
		shell := &va.planet.Shells[shellIdx]
		shellBelow := &va.planet.Shells[shellIdx-1]

		var movements []verticalMove

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip non-rock materials
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				// Check if SubPosR has moved below this shell
				if voxel.SubPosR < -1.0 && voxel.VelR < 0 {
					// This voxel should move to the shell below
					movements = append(movements, verticalMove{
						voxel:       voxel,
						sourceLat:   latIdx,
						sourceLon:   lonIdx,
						targetShell: shellIdx - 1,
					})
				}
			}
		}

		// Apply movements
		for _, move := range movements {
			// Find corresponding position in lower shell
			targetLat, targetLon := va.findCorrespondingVoxel(
				shellIdx, move.sourceLat, move.sourceLon,
				move.targetShell)

			if targetLat >= 0 && targetLat < len(shellBelow.Voxels) &&
				targetLon >= 0 && targetLon < len(shellBelow.Voxels[targetLat]) {

				targetVoxel := &shellBelow.Voxels[targetLat][targetLon]
				sourceVoxel := &shell.Voxels[move.sourceLat][move.sourceLon]

				// Handle material transformation during subduction
				if move.voxel.Type == core.MatBasalt && targetVoxel.Type == core.MatPeridotite {
					// Oceanic crust subducting into mantle
					// Mix properties based on depth
					mixRatio := float32(0.3) // 30% of subducting material mixes

					// Transfer heat and composition
					targetVoxel.Temperature = targetVoxel.Temperature*(1-mixRatio) +
						move.voxel.Temperature*mixRatio
					targetVoxel.Composition = targetVoxel.Composition*(1-mixRatio) +
						move.voxel.Composition*mixRatio

					// Increase density as basalt transforms under pressure
					move.voxel.Density *= 1.1

					// Potential for magma generation if hot enough
					if targetVoxel.Temperature > 1400 {
						// Partial melting - some material becomes magma
						if targetVoxel.Type != core.MatMagma {
							targetVoxel.Type = core.MatPeridotite // Keep as peridotite but mark as melting
							targetVoxel.MeltFraction = 0.1        // 10% melt
						}
					}
				}

				// Replace source with ocean water or mantle material
				if shellIdx == surfaceShell {
					*sourceVoxel = core.VoxelMaterial{
						Type:        core.MatWater,
						Density:     core.MaterialProperties[core.MatWater].DefaultDensity,
						Temperature: sourceVoxel.Temperature * 0.3, // Cool ocean water
						Elevation:   -2000,                         // Ocean depth
					}
				} else {
					// Fill with mantle material from below
					*sourceVoxel = *targetVoxel
					sourceVoxel.VelR = 0.0001 // Slight upward velocity to fill gap
				}

				// Move material properties to target
				if targetVoxel.Type == core.MatPeridotite || targetVoxel.Type == core.MatMagma {
					// Mix into mantle
					*targetVoxel = *move.voxel
					targetVoxel.SubPosR += 1.0 // Adjust position within new shell
				}
			}
		}
	}

	// Second pass: Handle rising material (mantle plumes, mountain building)
	for shellIdx := 1; shellIdx < surfaceShell+1; shellIdx++ {
		shell := &va.planet.Shells[shellIdx]
		if shellIdx >= len(va.planet.Shells)-1 {
			continue
		}
		shellAbove := &va.planet.Shells[shellIdx+1]

		var movements []verticalMove

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Check if SubPosR has moved above this shell
				if voxel.SubPosR > 1.0 && voxel.VelR > 0 {
					movements = append(movements, verticalMove{
						voxel:       voxel,
						sourceLat:   latIdx,
						sourceLon:   lonIdx,
						targetShell: shellIdx + 1,
					})
				}
			}
		}

		// Apply upward movements
		for _, move := range movements {
			targetLat, targetLon := va.findCorrespondingVoxel(
				shellIdx, move.sourceLat, move.sourceLon,
				move.targetShell)

			if targetLat >= 0 && targetLat < len(shellAbove.Voxels) &&
				targetLon >= 0 && targetLon < len(shellAbove.Voxels[targetLat]) {

				targetVoxel := &shellAbove.Voxels[targetLat][targetLon]
				sourceVoxel := &shell.Voxels[move.sourceLat][move.sourceLon]

				// Handle material transformation during ascent
				if move.voxel.Type == core.MatMagma {
					// Magma rising - may cool and solidify
					coolingRate := float32(50.0) // K per shell
					move.voxel.Temperature -= coolingRate

					if move.voxel.Temperature < 1200 && shellIdx >= surfaceShell-2 {
						// Solidify near surface
						move.voxel.Type = core.MatBasalt
						move.voxel.Density = core.MaterialProperties[core.MatBasalt].DefaultDensity
					}
				} else if move.voxel.Type == core.MatGranite && shellIdx == surfaceShell-1 {
					// Continental crust rising - mountain building
					move.voxel.Elevation += 100 // Extra elevation boost
				}

				// Move to upper shell
				if targetVoxel.Type == core.MatAir || targetVoxel.Type == core.MatWater {
					// Simple replacement
					*targetVoxel = *move.voxel
					targetVoxel.SubPosR -= 1.0 // Adjust position within new shell
				} else {
					// Collision - push existing material up
					targetVoxel.VelR = float32(math.Max(float64(targetVoxel.VelR), 0.0001))
					targetVoxel.Elevation += 50
				}

				// Replace source with material from below
				*sourceVoxel = core.VoxelMaterial{
					Type:        core.MatPeridotite,
					Density:     core.MaterialProperties[core.MatPeridotite].DefaultDensity,
					Temperature: sourceVoxel.Temperature + 100, // Hotter material from below
				}
			}
		}
	}
}

// findCorrespondingVoxel maps a voxel position from one shell to another
func (va *VoxelAdvection) findCorrespondingVoxel(fromShell, fromLat, fromLon, toShell int) (int, int) {
	if toShell < 0 || toShell >= len(va.planet.Shells) {
		return -1, -1
	}

	fromShellData := &va.planet.Shells[fromShell]
	toShellData := &va.planet.Shells[toShell]

	// Map latitude
	latRatio := float64(fromLat) / float64(fromShellData.LatBands)
	targetLat := int(latRatio * float64(toShellData.LatBands))
	if targetLat >= toShellData.LatBands {
		targetLat = toShellData.LatBands - 1
	}

	// Map longitude
	lonRatio := float64(fromLon) / float64(fromShellData.LonCounts[fromLat])
	targetLon := int(lonRatio * float64(toShellData.LonCounts[targetLat]))
	if targetLon >= toShellData.LonCounts[targetLat] {
		targetLon = toShellData.LonCounts[targetLat] - 1
	}

	return targetLat, targetLon
}

// fillPlateGaps detects and fills gaps within tectonic plates using improved algorithms
func (va *VoxelAdvection) fillPlateGaps(newVoxels *[][]core.VoxelMaterial, shell *core.SphericalShell) {
	// Define helper types
	type voxelPos struct {
		lat, lon int
	}

	type subPos struct {
		lat, lon float32
	}

	type gapInfo struct {
		lat, lon      int
		neighborCount int
		avgSubPos     subPos
	}

	// Group voxels by plate ID with enhanced tracking
	type plateInfo struct {
		voxels     []voxelPos
		material   core.MaterialType
		avgTemp    float32
		avgAge     float32
		avgDensity float32
		avgStress  float32
		// Track plate deformation
		totalArea        int // Number of voxels
		stretchedVoxels  int
		compressedVoxels int
	}
	plates := make(map[int32]*plateInfo)

	// Collect plate voxels with enhanced tracking
	for latIdx := range *newVoxels {
		for lonIdx := range (*newVoxels)[latIdx] {
			voxel := &(*newVoxels)[latIdx][lonIdx]
			if voxel.PlateID > 0 && (voxel.Type == core.MatGranite || voxel.Type == core.MatBasalt) {
				if plates[voxel.PlateID] == nil {
					plates[voxel.PlateID] = &plateInfo{
						material: voxel.Type,
					}
				}
				info := plates[voxel.PlateID]
				info.voxels = append(info.voxels, voxelPos{latIdx, lonIdx})
				info.avgTemp += voxel.Temperature
				info.avgAge += voxel.Age
				info.avgDensity += voxel.Density
				info.avgStress += voxel.Stress
				info.totalArea++

				// Track deformation
				if voxel.StretchFactor > 1.1 {
					info.stretchedVoxels++
				} else if voxel.StretchFactor < 0.9 {
					info.compressedVoxels++
				}
			}
		}
	}

	// Calculate averages
	for _, info := range plates {
		if len(info.voxels) > 0 {
			count := float32(len(info.voxels))
			info.avgTemp /= count
			info.avgAge /= count
			info.avgDensity /= count
			info.avgStress /= count
		}
	}

	// For each plate, detect and fill internal gaps using flood-fill
	for plateID, info := range plates {
		if len(info.voxels) < 10 {
			continue // Skip very small plates
		}

		// Calculate plate deformation rate
		deformationRate := float32(info.stretchedVoxels+info.compressedVoxels) / float32(info.totalArea)

		// Create a visited map for flood-fill
		visited := make(map[voxelPos]bool)

		// Mark all existing plate voxels as visited
		for _, v := range info.voxels {
			visited[v] = true
		}

		// For each plate voxel, check for adjacent gaps
		gapsToFill := make([]gapInfo, 0)

		for _, v := range info.voxels {
			// Check all 4 direct neighbors (not diagonals for initial detection)
			neighbors := []struct{ dlat, dlon int }{
				{-1, 0}, {1, 0}, {0, -1}, {0, 1},
			}

			for _, n := range neighbors {
				nlat := v.lat + n.dlat
				nlon := (v.lon + n.dlon + len((*newVoxels)[v.lat])) % len((*newVoxels)[v.lat])

				// Skip if out of bounds
				if nlat < 0 || nlat >= len(*newVoxels) {
					continue
				}

				// Check if longitude index is valid for this latitude
				if nlon >= len((*newVoxels)[nlat]) {
					continue
				}

				// Skip if already visited or not a gap
				if visited[voxelPos{nlat, nlon}] {
					continue
				}

				neighbor := &(*newVoxels)[nlat][nlon]
				if neighbor.Type != core.MatWater && neighbor.Type != core.MatAir {
					continue
				}

				// Found a potential gap - check if it's internal to the plate
				plateNeighbors := 0
				totalSubPosLat := float32(0)
				totalSubPosLon := float32(0)

				// Check all 8 neighbors of the gap
				for dlat := -1; dlat <= 1; dlat++ {
					for dlon := -1; dlon <= 1; dlon++ {
						if dlat == 0 && dlon == 0 {
							continue
						}

						nnlat := nlat + dlat
						nnlon := (nlon + dlon + len((*newVoxels)[nlat])) % len((*newVoxels)[nlat])

						if nnlat >= 0 && nnlat < len(*newVoxels) && nnlon < len((*newVoxels)[nnlat]) {
							if (*newVoxels)[nnlat][nnlon].PlateID == plateID {
								plateNeighbors++
								// Accumulate sub-positions for interpolation
								totalSubPosLat += (*newVoxels)[nnlat][nnlon].SubPosLat
								totalSubPosLon += (*newVoxels)[nnlat][nnlon].SubPosLon
							}
						}
					}
				}

				// If mostly surrounded by plate material, mark for filling
				if plateNeighbors >= 4 {
					avgSubPos := subPos{
						lat: totalSubPosLat / float32(plateNeighbors),
						lon: totalSubPosLon / float32(plateNeighbors),
					}
					gapsToFill = append(gapsToFill, gapInfo{
						lat:           nlat,
						lon:           nlon,
						neighborCount: plateNeighbors,
						avgSubPos:     avgSubPos,
					})
					visited[voxelPos{nlat, nlon}] = true
				}
			}
		}

		// Fill the detected gaps with appropriate material properties
		for _, gap := range gapsToFill {
			voxel := &(*newVoxels)[gap.lat][gap.lon]

			// Determine stretch factor based on neighbors and plate deformation
			stretchFactor := float32(1.0)
			if deformationRate > 0.1 {
				// High deformation - gaps are likely from stretching
				stretchFactor = 1.0 + deformationRate
			}

			// Fill with interpolated properties
			voxel.Type = info.material
			voxel.Density = info.avgDensity
			voxel.Temperature = info.avgTemp
			voxel.Age = info.avgAge
			voxel.Stress = info.avgStress * 0.8 // Slightly less stress in filled areas
			voxel.PlateID = plateID
			voxel.IsTransient = true
			voxel.SourcePlateID = plateID
			voxel.StretchFactor = stretchFactor

			// Interpolate sub-positions from neighbors
			voxel.SubPosLat = gap.avgSubPos.lat
			voxel.SubPosLon = gap.avgSubPos.lon

			// Mark as having moved recently to prevent immediate re-movement
			voxel.LastMoveTime = float32(va.planet.Time)
		}

		// Removed debug logging for cleaner output
	}
}
