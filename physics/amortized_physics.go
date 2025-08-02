package physics

import (
	"worldgenerator/core"
	"worldgenerator/gpu"
)

// AmortizedPhysicsState manages spreading physics calculations over multiple frames
type AmortizedPhysicsState struct {
	// Current processing state
	currentShell    int
	currentLatBand  int
	currentPhase    PhysicsPhase
	
	// Phases of physics calculation
	totalPhases     int
	
	// Time accumulation for physics
	accumulatedTime float64
	targetDeltaTime float64
	
	// Progress tracking
	shellsPerFrame  int
	phasesPerFrame  int
}

// PhysicsPhase represents different stages of physics calculation
type PhysicsPhase int

const (
	PhaseTemperatureDiffusion PhysicsPhase = iota
	PhasePressureCalculation
	PhasePhaseTransitions
	PhaseMechanics
	PhaseConvection
	PhasePlateMotion
	PhaseBoundaryProcesses
	PhaseAdvection
	PhaseSurfaceProcesses
	PhaseAgeUpdate
	PhaseComplete
)

// NewAmortizedPhysicsState creates a new amortized physics state
func NewAmortizedPhysicsState() *AmortizedPhysicsState {
	return &AmortizedPhysicsState{
		currentShell:    0,
		currentLatBand:  0,
		currentPhase:    PhaseTemperatureDiffusion,
		totalPhases:     int(PhaseComplete),
		shellsPerFrame:  2,  // Process 2 shells per frame
		phasesPerFrame:  1,  // Process 1 phase per frame
		targetDeltaTime: 0.0, // Will be set when physics runs
	}
}

// UpdateAmortized performs a partial physics update, spreading work across frames
func UpdateAmortizedPhysics(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState, gpuCompute gpu.GPUCompute) bool {
	// Accumulate time
	state.accumulatedTime += dt
	
	// Only start new physics cycle if we've accumulated enough time
	if state.currentPhase == PhaseComplete {
		if state.accumulatedTime < state.targetDeltaTime {
			return false // Not time for physics yet
		}
		// Reset for new cycle
		state.currentPhase = PhaseTemperatureDiffusion
		state.currentShell = 0
		state.targetDeltaTime = state.accumulatedTime
		state.accumulatedTime = 0
	}
	
	// Perform work for this frame
	switch state.currentPhase {
	case PhaseTemperatureDiffusion:
		updateTemperatureAmortized(planet, state.targetDeltaTime, state)
		
	case PhasePressureCalculation:
		updatePressureAmortized(planet, state.targetDeltaTime, state)
		
	case PhasePhaseTransitions:
		updatePhaseTransitionsAmortized(planet, state.targetDeltaTime, state)
		
	case PhaseMechanics:
		if planet.Physics != nil {
			if vp, ok := planet.Physics.(*VoxelPhysics); ok && vp.mechanics != nil {
				vp.mechanics.UpdateMechanics(state.targetDeltaTime)
			}
		}
		state.currentPhase++
		
	case PhaseConvection:
		if planet.Physics != nil {
			if vp, ok := planet.Physics.(*VoxelPhysics); ok && vp.advection != nil {
				vp.advection.UpdateConvection(state.targetDeltaTime)
			}
		}
		state.currentPhase++
		
	case PhasePlateMotion:
		if planet.Physics != nil {
			if vp, ok := planet.Physics.(*VoxelPhysics); ok && vp.plates != nil {
				// Only re-identify plates occasionally
				if int(planet.Time)%10000000 == 0 {
					vp.plates.IdentifyPlates()
				}
				vp.plates.UpdatePlateMotion(state.targetDeltaTime)
			}
		}
		state.currentPhase++
		
	case PhaseBoundaryProcesses:
		if planet.Physics != nil {
			if vp, ok := planet.Physics.(*VoxelPhysics); ok && vp.mechanics != nil {
				vp.mechanics.ApplyRidgePush(state.targetDeltaTime)
				vp.mechanics.UpdateTransformFaults(state.targetDeltaTime)
				vp.mechanics.UpdateCollisions(state.targetDeltaTime)
				vp.mechanics.UpdateContinentalBreakup(state.targetDeltaTime)
			}
		}
		state.currentPhase++
		
	case PhaseAdvection:
		if planet.Physics != nil {
			if vp, ok := planet.Physics.(*VoxelPhysics); ok && vp.advection != nil {
				vp.advection.AdvectMaterial(state.targetDeltaTime)
			}
		}
		state.currentPhase++
		
	case PhaseSurfaceProcesses:
		updateSurfaceProcessesAmortized(planet, state.targetDeltaTime, state)
		
	case PhaseAgeUpdate:
		updateAgeAmortized(planet, state.targetDeltaTime, state)
	}
	
	// Return true if physics cycle is complete
	return state.currentPhase == PhaseComplete
}

// updateTemperatureAmortized handles heat diffusion for a subset of shells
func updateTemperatureAmortized(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState) {
	dtFloat := float32(dt)
	
	// Process only a few shells per frame
	endShell := state.currentShell + state.shellsPerFrame
	if endShell > len(planet.Shells) {
		endShell = len(planet.Shells)
	}
	
	// Create temporary buffer for shells we're processing
	for shellIdx := state.currentShell; shellIdx < endShell; shellIdx++ {
		shell := &planet.Shells[shellIdx]
		
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx, voxel := range latVoxels {
				// Skip air
				if voxel.Type == core.MatAir {
					continue
				}
				
				// Get material properties
				props := core.MaterialProperties[voxel.Type]
				alpha := props.ThermalConductivity / (props.DefaultDensity * props.SpecificHeat)
				
				// Calculate heat flow from neighbors
				heatFlow := float32(0.0)
				neighborCount := 0
				
				// Simplified heat diffusion (radial only for performance)
				if shellIdx > 0 && shellIdx < len(planet.Shells)-1 {
					innerShell := &planet.Shells[shellIdx-1]
					outerShell := &planet.Shells[shellIdx+1]
					
					if latIdx < len(innerShell.Voxels) && lonIdx < len(innerShell.Voxels[latIdx]) {
						innerVoxel := &innerShell.Voxels[latIdx][lonIdx]
						dr := shell.InnerRadius - innerShell.OuterRadius
						if dr > 0 {
							dT := innerVoxel.Temperature - voxel.Temperature
							heatFlow += dT * float32(alpha) / float32(dr*dr)
							neighborCount++
						}
					}
					
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
				
				// Apply heat flow
				if neighborCount > 0 {
					planet.Shells[shellIdx].Voxels[latIdx][lonIdx].Temperature += heatFlow * dtFloat
				}
				
				// Add radioactive heating in deep shells
				if shellIdx < 5 {
					radioHeat := float32(1e-12) * dtFloat * 1e6
					planet.Shells[shellIdx].Voxels[latIdx][lonIdx].Temperature += radioHeat
				}
			}
		}
	}
	
	// Update state
	state.currentShell = endShell
	if state.currentShell >= len(planet.Shells) {
		state.currentShell = 0
		state.currentPhase++
		
		// Apply surface boundary conditions when done
		if len(planet.Shells) > 0 {
			surfaceShell := len(planet.Shells) - 1
			for latIdx, latVoxels := range planet.Shells[surfaceShell].Voxels {
				lat := core.GetLatitudeForBand(latIdx, planet.Shells[surfaceShell].LatBands)
				for lonIdx := range latVoxels {
					surfaceTemp := float32(288 - 50*absFloat32(float32(lat))/90.0)
					planet.Shells[surfaceShell].Voxels[latIdx][lonIdx].Temperature = surfaceTemp
				}
			}
		}
	}
}

// updatePressureAmortized calculates pressure for a subset of shells
func updatePressureAmortized(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState) {
	// Process from surface down, a few shells at a time
	startShell := len(planet.Shells) - 1 - state.currentShell
	endShell := startShell - state.shellsPerFrame
	if endShell < 0 {
		endShell = 0
	}
	
	for shellIdx := startShell; shellIdx >= endShell; shellIdx-- {
		shell := &planet.Shells[shellIdx]
		
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx := range latVoxels {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				if shellIdx == len(planet.Shells)-1 {
					voxel.Pressure = 101325 // Surface pressure
				} else {
					outerShell := &planet.Shells[shellIdx+1]
					if latIdx < len(outerShell.Voxels) && lonIdx < len(outerShell.Voxels[latIdx]) {
						outerVoxel := &outerShell.Voxels[latIdx][lonIdx]
						dr := outerShell.InnerRadius - shell.OuterRadius
						g := 9.8
						dP := outerVoxel.Density * float32(g*dr)
						voxel.Pressure = outerVoxel.Pressure + dP
					}
				}
			}
		}
	}
	
	// Update state
	state.currentShell += state.shellsPerFrame
	if startShell-state.currentShell < 0 {
		state.currentShell = 0
		state.currentPhase++
	}
}

// updatePhaseTransitionsAmortized handles melting/solidification for a subset of shells
func updatePhaseTransitionsAmortized(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState) {
	endShell := state.currentShell + state.shellsPerFrame
	if endShell > len(planet.Shells) {
		endShell = len(planet.Shells)
	}
	
	for shellIdx := state.currentShell; shellIdx < endShell; shellIdx++ {
		shell := &planet.Shells[shellIdx]
		
		for latIdx, latVoxels := range shell.Voxels {
			for lonIdx := range latVoxels {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				// Skip air and water
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}
				
				props := core.MaterialProperties[voxel.Type]
				
				// Check for melting
				if voxel.Type != core.MatMagma && voxel.Temperature > props.MeltingPoint {
					meltFraction := (voxel.Temperature - props.MeltingPoint) / 200.0
					if meltFraction > 0.5 {
						voxel.Type = core.MatMagma
						voxel.Density = core.MaterialProperties[core.MatMagma].DefaultDensity
					}
				}
				
				// Check for solidification
				if voxel.Type == core.MatMagma {
					solidusTemp := float32(1200)
					if voxel.Temperature < solidusTemp {
						voxel.Type = core.MatBasalt
						voxel.Density = core.MaterialProperties[core.MatBasalt].DefaultDensity
						voxel.Age = 0
					}
				}
			}
		}
	}
	
	state.currentShell = endShell
	if state.currentShell >= len(planet.Shells) {
		state.currentShell = 0
		state.currentPhase++
	}
}

// updateSurfaceProcessesAmortized handles surface processes
func updateSurfaceProcessesAmortized(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState) {
	if len(planet.Shells) < 2 {
		state.currentPhase++
		return
	}
	
	surfaceShell := len(planet.Shells) - 2
	shell := &planet.Shells[surfaceShell]
	
	// Process only part of the surface each frame
	latBands := len(shell.Voxels)
	bandsPerFrame := latBands / 4 // Process 1/4 of surface per frame
	if bandsPerFrame < 1 {
		bandsPerFrame = 1
	}
	
	endLat := state.currentLatBand + bandsPerFrame
	if endLat > latBands {
		endLat = latBands
	}
	
	for latIdx := state.currentLatBand; latIdx < endLat; latIdx++ {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Simple erosion
			if voxel.Type == core.MatGranite || voxel.Type == core.MatBasalt {
				if voxel.VelR > 0 {
					voxel.VelR -= float32(1e-10 * dt)
				}
			}
		}
	}
	
	state.currentLatBand = endLat
	if state.currentLatBand >= latBands {
		state.currentLatBand = 0
		state.currentPhase++
	}
}

// updateAgeAmortized increments age for a subset of shells
func updateAgeAmortized(planet *core.VoxelPlanet, dt float64, state *AmortizedPhysicsState) {
	endShell := state.currentShell + state.shellsPerFrame * 2 // Age update is fast, do more shells
	if endShell > len(planet.Shells) {
		endShell = len(planet.Shells)
	}
	
	for shellIdx := state.currentShell; shellIdx < endShell; shellIdx++ {
		shell := &planet.Shells[shellIdx]
		
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				// Only age solid materials
				if voxel.Type != core.MatAir && voxel.Type != core.MatWater && voxel.Type != core.MatMagma {
					voxel.Age += float32(dt)
				}
			}
		}
	}
	
	state.currentShell = endShell
	if state.currentShell >= len(planet.Shells) {
		state.currentShell = 0
		state.currentPhase = PhaseComplete
	}
}

// absFloat32 is a helper function for absolute value
func absFloat32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}