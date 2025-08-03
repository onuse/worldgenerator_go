package physics

import (
	"math"
	"worldgenerator/core"
)

// ContinentalDriftState tracks continent movement for visualization
type ContinentalDriftState struct {
	// Accumulated displacement for each surface point
	LonDisplacement [][]float64 // Degrees of longitude displacement per voxel
	LatDisplacement [][]float64 // Degrees of latitude displacement per voxel

	// For interpolating movement
	LastUpdateTime float64
	UpdateInterval float64 // How often to actually move voxels
}

// NewContinentalDriftState creates a drift tracker
func NewContinentalDriftState(planet *core.VoxelPlanet) *ContinentalDriftState {
	surfaceShell := len(planet.Shells) - 2
	if surfaceShell < 0 {
		return nil
	}

	shell := &planet.Shells[surfaceShell]
	state := &ContinentalDriftState{
		LonDisplacement: make([][]float64, shell.LatBands),
		LatDisplacement: make([][]float64, shell.LatBands),
		UpdateInterval:  100000.0, // Update every 100k years of sim time
	}

	for latIdx := range shell.Voxels {
		state.LonDisplacement[latIdx] = make([]float64, len(shell.Voxels[latIdx]))
		state.LatDisplacement[latIdx] = make([]float64, len(shell.Voxels[latIdx]))
	}

	return state
}

// UpdateContinentalDrift accumulates movement and periodically shifts continents
func UpdateContinentalDrift(planet *core.VoxelPlanet, state *ContinentalDriftState, dt float64, speedMultiplier float64) {
	if state == nil {
		return
	}

	surfaceShell := len(planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &planet.Shells[surfaceShell]
	radius := (shell.InnerRadius + shell.OuterRadius) / 2

	// Accumulate displacement
	for latIdx := range shell.Voxels {
		lat := core.GetLatitudeForBand(latIdx, shell.LatBands)
		cosLat := math.Cos(lat * math.Pi / 180.0)
		if math.Abs(cosLat) < 0.01 {
			cosLat = 0.01 // Avoid division by zero near poles
		}

		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip water and air
			if voxel.Type == core.MatWater || voxel.Type == core.MatAir {
				continue
			}

			// Convert velocity (m/s) to degrees per time
			// Circumference at this latitude = 2*pi*radius*cos(lat)
			circumference := 2.0 * math.Pi * radius * cosLat
			degreesPerMeter := 360.0 / circumference

			// Accumulate displacement
			lonDisp := float64(voxel.VelEast) * dt * speedMultiplier * degreesPerMeter
			latDisp := float64(voxel.VelNorth) * dt * speedMultiplier * 360.0 / (2.0 * math.Pi * radius)

			state.LonDisplacement[latIdx][lonIdx] += lonDisp
			state.LatDisplacement[latIdx][lonIdx] += latDisp
		}
	}

	// Check if it's time to actually move the continents
	planet.Time += dt * speedMultiplier
	if planet.Time-state.LastUpdateTime < state.UpdateInterval {
		return
	}

	state.LastUpdateTime = planet.Time

	// Create a copy of the surface to move materials
	newSurface := make([][]core.VoxelMaterial, shell.LatBands)
	for latIdx := range shell.Voxels {
		newSurface[latIdx] = make([]core.VoxelMaterial, len(shell.Voxels[latIdx]))
		// Initialize with water
		for lonIdx := range newSurface[latIdx] {
			newSurface[latIdx][lonIdx] = core.VoxelMaterial{
				Type:        core.MatWater,
				Density:     core.MaterialProperties[core.MatWater].DefaultDensity,
				Temperature: shell.Voxels[latIdx][lonIdx].Temperature,
			}
		}
	}

	// Move continental material
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only move solid crustal material
			if voxel.Type != core.MatGranite && voxel.Type != core.MatBasalt {
				continue
			}

			// Calculate new position
			lonDisp := state.LonDisplacement[latIdx][lonIdx]
			latDisp := state.LatDisplacement[latIdx][lonIdx]

			// Convert displacement to grid indices
			lonOffset := int(math.Round(lonDisp * float64(len(shell.Voxels[latIdx])) / 360.0))
			latOffset := int(math.Round(latDisp * float64(shell.LatBands) / 180.0))

			// Calculate new indices with wrapping
			newLon := (lonIdx + lonOffset) % len(shell.Voxels[latIdx])
			if newLon < 0 {
				newLon += len(shell.Voxels[latIdx])
			}

			newLat := latIdx + latOffset
			if newLat >= 0 && newLat < shell.LatBands {
				// Place the material at its new location
				// Handle collisions - if there's already land there, create mountains
				if newSurface[newLat][newLon].Type == core.MatGranite ||
					newSurface[newLat][newLon].Type == core.MatBasalt {
					// Collision! Increase stress and push material up
					voxel.Stress += 1e7
					voxel.VelR = 1e-4 // Uplift
				} else {
					// Move the material
					newSurface[newLat][newLon] = *voxel
				}
			}

			// Reset displacement after moving
			state.LonDisplacement[latIdx][lonIdx] = 0
			state.LatDisplacement[latIdx][lonIdx] = 0
		}
	}

	// Copy new surface back
	shell.Voxels = newSurface

	// Update the rendering textures
	planet.MeshDirty = true
}
