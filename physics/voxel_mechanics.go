package physics

import (
	"math"
	"worldgenerator/core"
)

// VoxelMechanics handles stress, strain, and fracturing
type VoxelMechanics struct {
	planet  *core.VoxelPlanet
	physics *VoxelPhysics
}

// NewVoxelMechanics creates a mechanics simulator
func NewVoxelMechanics(planet *core.VoxelPlanet, physics *VoxelPhysics) *VoxelMechanics {
	return &VoxelMechanics{
		planet:  planet,
		physics: physics,
	}
}

// UpdateMechanics calculates stress and material strength
func (vm *VoxelMechanics) UpdateMechanics(dt float64) {
	// Update yield strength based on temperature
	vm.updateYieldStrength()

	// Calculate stress from velocity gradients
	vm.updateStress(dt)

	// Check for fracturing
	vm.checkFracturing()
}

// updateYieldStrength calculates temperature-dependent material strength
func (vm *VoxelMechanics) updateYieldStrength() {
	for shellIdx := range vm.planet.Shells {
		shell := &vm.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip air and water
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				// Calculate strength based on temperature
				// Rock strength decreases exponentially with temperature
				T := float64(voxel.Temperature)
				Tm := float64(core.MaterialProperties[voxel.Type].MeltingPoint)

				// Normalized temperature (0 = cold, 1 = melting)
				Tnorm := (T - 273.15) / (Tm - 273.15)
				if Tnorm < 0 {
					Tnorm = 0
				}
				if Tnorm > 1 {
					Tnorm = 1
				}

				// Brittle-ductile transition
				// Cold rock is brittle (high strength but fractures)
				// Hot rock is ductile (low strength but flows)
				if Tnorm < 0.4 {
					// Brittle regime
					voxel.YieldStrength = float32(1e9 * (1.0 - Tnorm)) // Up to 1 GPa
					voxel.IsBrittle = true
				} else {
					// Ductile regime
					voxel.YieldStrength = float32(1e9 * math.Exp(-5.0*Tnorm)) // Exponential decrease
					voxel.IsBrittle = false
				}

				// Pressure strengthening
				// Higher pressure increases strength
				pressureFactor := 1.0 + float64(voxel.Pressure)/1e9
				voxel.YieldStrength *= float32(pressureFactor)
			}
		}
	}
}

// updateStress calculates stress from velocity gradients
func (vm *VoxelMechanics) updateStress(dt float64) {
	for shellIdx := range vm.planet.Shells {
		shell := &vm.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip fluids
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater || voxel.Type == core.MatMagma {
					voxel.Stress = 0
					continue
				}

				// Calculate velocity gradients (strain rate)
				strainRate := vm.calculateStrainRate(shellIdx, latIdx, lonIdx)

				// Stress = viscosity * strain rate
				viscosity := vm.getEffectiveViscosity(voxel)
				newStress := float32(viscosity * strainRate * dt)

				// Accumulate stress
				voxel.Stress += newStress

				// Maxwell relaxation: stress decays over time
				relaxTime := viscosity / float64(voxel.YieldStrength)
				voxel.Stress *= float32(math.Exp(-dt / relaxTime))
			}
		}
	}
}

// calculateStrainRate computes velocity gradients
func (vm *VoxelMechanics) calculateStrainRate(shellIdx, latIdx, lonIdx int) float64 {
	shell := &vm.planet.Shells[shellIdx]
	voxel := &shell.Voxels[latIdx][lonIdx]

	// Get neighboring velocities for gradient calculation
	strainRate := 0.0

	// Radial strain
	if shellIdx < len(vm.planet.Shells)-1 {
		outerVoxel := vm.physics.getRadialNeighbor(shellIdx, shellIdx+1, latIdx, lonIdx)
		if outerVoxel != nil {
			dr := shell.OuterRadius - shell.InnerRadius
			dVr := float64(outerVoxel.VelR - voxel.VelR)
			strainRate += math.Abs(dVr / dr)
		}
	}

	// Lateral strain (simplified)
	// Check east-west gradient
	if lonIdx < len(shell.Voxels[latIdx])-1 {
		eastVoxel := &shell.Voxels[latIdx][lonIdx+1]
		dVphi := float64(eastVoxel.VelEast - voxel.VelEast)
		// Approximate distance
		radius := (shell.OuterRadius + shell.InnerRadius) / 2
		lat := core.GetLatitudeForBand(latIdx, shell.LatBands) * math.Pi / 180
		dx := radius * math.Cos(lat) * 2 * math.Pi / float64(len(shell.Voxels[latIdx]))
		if dx > 0 {
			strainRate += math.Abs(dVphi / dx)
		}
	}

	return strainRate
}

// getEffectiveViscosity returns temperature and stress-dependent viscosity
func (vm *VoxelMechanics) getEffectiveViscosity(voxel *core.VoxelMaterial) float64 {
	// Base viscosity from temperature
	baseVisc := 1e21 // Pa·s
	T := float64(voxel.Temperature)

	// Arrhenius temperature dependence
	viscosity := baseVisc * math.Exp(30000.0/8.314/T)

	// Non-Newtonian behavior: viscosity decreases with stress
	if voxel.Stress > 0 {
		// Power-law creep
		n := 3.0 // Stress exponent
		stressRatio := float64(voxel.Stress) / float64(voxel.YieldStrength)
		if stressRatio > 0.1 {
			viscosity *= math.Pow(stressRatio, -1.0/n)
		}
	}

	// Clamp to reasonable range
	if viscosity > 1e25 {
		viscosity = 1e25
	}
	if viscosity < 1e19 {
		viscosity = 1e19
	}

	return viscosity
}

// checkFracturing creates faults when stress exceeds strength
func (vm *VoxelMechanics) checkFracturing() {
	for shellIdx := range vm.planet.Shells {
		shell := &vm.planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Only brittle materials can fracture
				if !voxel.IsBrittle {
					continue
				}

				// Check if stress exceeds yield strength
				if voxel.Stress > voxel.YieldStrength {
					// Fracture! Release stress
					voxel.Stress = 0

					// Mark as fractured (could trigger earthquakes, etc.)
					voxel.IsFractured = true

					// Reduce cohesion temporarily
					voxel.YieldStrength *= 0.5
				} else {
					// Healing over time
					voxel.IsFractured = false
					if voxel.YieldStrength < float32(1e9) {
						voxel.YieldStrength *= 1.01 // Slow healing
					}
				}
			}
		}
	}
}

// GetLithosphereThickness returns the depth of the rigid lithosphere
func (vm *VoxelMechanics) GetLithosphereThickness(latIdx, lonIdx int) float64 {
	// Find the depth where material transitions from brittle to ductile
	for shellIdx := len(vm.planet.Shells) - 1; shellIdx >= 0; shellIdx-- {
		shell := &vm.planet.Shells[shellIdx]

		if latIdx >= len(shell.Voxels) {
			continue
		}

		lonCount := len(shell.Voxels[latIdx])
		if lonIdx >= lonCount {
			continue
		}

		voxel := &shell.Voxels[latIdx][lonIdx%lonCount]

		// Found the brittle-ductile transition
		if !voxel.IsBrittle {
			depth := vm.planet.Radius - shell.OuterRadius
			return depth
		}
	}

	// All brittle (shouldn't happen)
	return vm.planet.Radius
}

// DetectPlateBoundaries identifies boundaries based on velocity differences
func (vm *VoxelMechanics) DetectPlateBoundaries() []PlateBoundary {
	boundaries := []PlateBoundary{}

	// Check surface shell
	surfaceShell := len(vm.planet.Shells) - 2
	if surfaceShell < 0 {
		return boundaries
	}

	shell := &vm.planet.Shells[surfaceShell]

	// Scan for velocity discontinuities
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip non-lithosphere
			if !voxel.IsBrittle || voxel.Type == core.MatAir || voxel.Type == core.MatWater {
				continue
			}

			// Check velocity difference with neighbors
			maxVelDiff := float32(0.0)
			var boundaryType string

			// East neighbor
			eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
			eastVoxel := &shell.Voxels[latIdx][eastLon]
			if eastVoxel.IsBrittle {
				velDiff := eastVoxel.VelEast - voxel.VelEast
				if abs(velDiff) > maxVelDiff {
					maxVelDiff = abs(velDiff)
					if velDiff > 0 {
						boundaryType = "divergent"
					} else if velDiff < 0 {
						boundaryType = "convergent"
					}
				}

				// Check for shear (transform)
				thetaDiff := abs(eastVoxel.VelNorth - voxel.VelNorth)
				if thetaDiff > maxVelDiff {
					maxVelDiff = thetaDiff
					boundaryType = "transform"
				}
			}

			// Significant velocity difference indicates boundary
			if maxVelDiff > 1e-6 {
				boundaries = append(boundaries, PlateBoundary{
					Type:      boundaryType,
					LatIdx:    latIdx,
					LonIdx:    lonIdx,
					Intensity: maxVelDiff,
				})
			}
		}
	}

	return boundaries
}

// ApplyRidgePush adds force at spreading centers
func (vm *VoxelMechanics) ApplyRidgePush(dt float64) {
	// Find divergent boundaries
	boundaries := vm.DetectPlateBoundaries()

	surfaceShell := len(vm.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vm.planet.Shells[surfaceShell]

	for _, boundary := range boundaries {
		if boundary.Type != "divergent" {
			continue
		}

		// Apply outward push from ridge
		if boundary.LatIdx < len(shell.Voxels) && boundary.LonIdx < len(shell.Voxels[boundary.LatIdx]) {
			// Ridge push magnitude based on elevation
			// Higher ridges = stronger push
			ridgeHeight := shell.OuterRadius - vm.planet.Radius
			pushForce := float32(ridgeHeight * 1e-10 * dt)

			// Apply force in direction away from ridge
			// This is simplified - in reality depends on ridge orientation
			for dLon := -5; dLon <= 5; dLon++ {
				targetLon := (boundary.LonIdx + dLon + len(shell.Voxels[boundary.LatIdx])) % len(shell.Voxels[boundary.LatIdx])
				targetVoxel := &shell.Voxels[boundary.LatIdx][targetLon]

				if targetVoxel.IsBrittle {
					// Push away from ridge
					if dLon > 0 {
						targetVoxel.VelEast += pushForce
					} else if dLon < 0 {
						targetVoxel.VelEast -= pushForce
					}
				}
			}
		}
	}
}

// UpdateTransformFaults handles strike-slip motion at transform boundaries
func (vm *VoxelMechanics) UpdateTransformFaults(dt float64) {
	boundaries := vm.DetectPlateBoundaries()

	surfaceShell := len(vm.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vm.planet.Shells[surfaceShell]

	for _, boundary := range boundaries {
		if boundary.Type != "transform" {
			continue
		}

		if boundary.LatIdx < len(shell.Voxels) && boundary.LonIdx < len(shell.Voxels[boundary.LatIdx]) {
			voxel := &shell.Voxels[boundary.LatIdx][boundary.LonIdx]

			// Accumulate shear stress at transform boundaries
			shearStress := boundary.Intensity * float32(dt) * 1e9
			voxel.Stress += shearStress

			// Check for strike-slip events (earthquakes)
			if voxel.Stress > voxel.YieldStrength {
				// Release stress in sudden slip
				voxel.Stress = 0
				voxel.IsFractured = true

				// Could trigger earthquake event here
			}
		}
	}
}

// abs returns absolute value of float32
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// PlateBoundary represents a detected plate boundary
type PlateBoundary struct {
	Type      string // "divergent", "convergent", "transform"
	LatIdx    int
	LonIdx    int
	Intensity float32 // Velocity difference magnitude
}

// UpdateCollisions handles continental collision and mountain building
func (vm *VoxelMechanics) UpdateCollisions(dt float64) {
	// Check surface shell for colliding continents
	surfaceShell := len(vm.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vm.planet.Shells[surfaceShell]

	// Find convergent boundaries involving continental crust
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip non-continental crust
			if voxel.Type != core.MatGranite {
				continue
			}

			// Check neighbors for collision
			// East neighbor
			eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
			eastVoxel := &shell.Voxels[latIdx][eastLon]

			// Continental-continental collision
			if eastVoxel.Type == core.MatGranite {
				velDiff := voxel.VelEast - eastVoxel.VelEast

				// Convergent motion between continents
				if velDiff > 1e-6 {
					// Apply collision effects
					vm.applyCollisionEffects(shell, latIdx, lonIdx, eastLon, dt)
				}
			}

			// Continental-oceanic collision (different dynamics)
			if eastVoxel.Type == core.MatBasalt {
				velDiff := voxel.VelEast - eastVoxel.VelEast

				if velDiff > 1e-6 {
					// Oceanic plate subducts, continental margin uplifts
					// This creates volcanic arcs
					vm.applySubductionEffects(shell, latIdx, lonIdx, eastLon, dt)
				}
			}
		}
	}
}

// applyCollisionEffects simulates continental collision
func (vm *VoxelMechanics) applyCollisionEffects(shell *core.SphericalShell, lat1, lon1, lon2 int, dt float64) {
	voxel1 := &shell.Voxels[lat1][lon1]
	voxel2 := &shell.Voxels[lat1][lon2]

	// Calculate collision intensity
	velDiff := abs(voxel1.VelEast - voxel2.VelEast)
	collisionForce := velDiff * float32(dt) * 1e6

	// 1. Crustal thickening (uplift)
	// Mountains form by vertical displacement
	upliftRate := collisionForce * 0.01
	voxel1.VelR += upliftRate
	voxel2.VelR += upliftRate

	// 2. Lateral deformation
	// Crust spreads perpendicular to collision
	voxel1.VelNorth += upliftRate * 0.5
	voxel2.VelNorth -= upliftRate * 0.5

	// 3. Stress accumulation
	voxel1.Stress += collisionForce
	voxel2.Stress += collisionForce

	// 4. Crustal thickening changes properties
	// Thicker crust = lower density at depth
	if voxel1.Pressure > 1e8 { // Deep crust
		voxel1.Density *= 0.999 // Slight density reduction
		voxel2.Density *= 0.999
	}

	// 5. Metamorphism - temperature increases with depth
	voxel1.Temperature += collisionForce * 0.001
	voxel2.Temperature += collisionForce * 0.001

	// 6. Slow down convergence (resistance)
	voxel1.VelEast *= 0.95
	voxel2.VelEast *= 0.95
}

// applySubductionEffects handles oceanic-continental collision
func (vm *VoxelMechanics) applySubductionEffects(shell *core.SphericalShell, lat1, lon1, lon2 int, dt float64) {
	continentalVoxel := &shell.Voxels[lat1][lon1]
	oceanicVoxel := &shell.Voxels[lat1][lon2]

	// Oceanic crust subducts
	oceanicVoxel.VelR = float32(math.Min(float64(oceanicVoxel.VelR)-0.00001*dt, -0.00001*dt))

	// Continental margin uplifts (volcanic arc)
	continentalVoxel.VelR += float32(0.000005 * dt)

	// Heat from subducting slab causes melting
	continentalVoxel.Temperature += 10 * float32(dt)

	// Create back-arc extension
	continentalVoxel.VelEast += float32(0.000001 * dt)
}

// UpdateContinentalBreakup handles rifting and continental separation
func (vm *VoxelMechanics) UpdateContinentalBreakup(dt float64) {
	// Check for areas under extension (rifting)
	surfaceShell := len(vm.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vm.planet.Shells[surfaceShell]

	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only continental crust can rift
			if voxel.Type != core.MatGranite {
				continue
			}

			// Check for extensional stress
			// This could be from mantle plumes or divergent boundaries

			// 1. Check if over a mantle plume (hot spot)
			if shellIdx := surfaceShell - 3; shellIdx >= 0 {
				deeperVoxel := vm.physics.getRadialNeighbor(surfaceShell, shellIdx, latIdx, lonIdx)
				if deeperVoxel != nil && deeperVoxel.Temperature > 3000 {
					// Hot mantle below - potential rifting
					vm.applyRiftingEffects(shell, latIdx, lonIdx, dt, "plume")
				}
			}

			// 2. Check for divergent motion with neighbors
			eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
			eastVoxel := &shell.Voxels[latIdx][eastLon]

			if eastVoxel.Type == core.MatGranite {
				velDiff := eastVoxel.VelEast - voxel.VelEast

				// Divergent motion between continental blocks
				if velDiff > 1e-6 {
					vm.applyRiftingEffects(shell, latIdx, lonIdx, dt, "extension")
				}
			}
		}
	}
}

// applyRiftingEffects simulates continental rifting
func (vm *VoxelMechanics) applyRiftingEffects(shell *core.SphericalShell, latIdx, lonIdx int, dt float64, riftType string) {
	voxel := &shell.Voxels[latIdx][lonIdx]

	// 1. Crustal thinning
	// Continental crust stretches and thins
	if voxel.YieldStrength < 1e8 { // Weakened crust
		// Subsidence as crust thins
		voxel.VelR -= float32(0.000002 * dt)

		// Increase temperature from mantle upwelling
		voxel.Temperature += float32(5 * dt)

		// Reduce yield strength further (positive feedback)
		voxel.YieldStrength *= 0.99
	}

	// 2. Check for complete breakup
	if voxel.YieldStrength < 1e7 && voxel.Temperature > 1200 {
		// Continental crust has failed - new ocean basin forms
		// Convert to basaltic crust (new oceanic crust)
		voxel.Type = core.MatBasalt
		voxel.Density = core.MaterialProperties[core.MatBasalt].DefaultDensity
		voxel.Age = 0 // New crust

		// Reset strength for new oceanic lithosphere
		voxel.YieldStrength = float32(2e8)
	}

	// 3. Create divergent velocities
	if riftType == "plume" {
		// Radial pattern away from plume
		voxel.VelNorth += float32(0.00001 * dt * math.Sin(float64(lonIdx)))
		voxel.VelEast += float32(0.00001 * dt * math.Cos(float64(lonIdx)))
	} else {
		// Linear rifting
		eastLon := (lonIdx + 1) % len(shell.Voxels[latIdx])
		eastVoxel := &shell.Voxels[latIdx][eastLon]

		// Push blocks apart
		voxel.VelEast -= float32(0.000005 * dt)
		eastVoxel.VelEast += float32(0.000005 * dt)
	}
}
