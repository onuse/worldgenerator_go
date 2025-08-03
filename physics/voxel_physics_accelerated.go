package physics

import (
	"worldgenerator/core"
)

// AcceleratedPhysicsParams allows speeding up geological processes for visualization
type AcceleratedPhysicsParams struct {
	// Time acceleration factors
	ConvectionSpeedMultiplier float64 // Speed up mantle convection
	PlateVelocityMultiplier   float64 // Speed up plate motion
	ErosionRateMultiplier     float64 // Speed up erosion

	// Visual feedback
	ExaggerateTopography   bool    // Make mountains taller
	TopographyExaggeration float64 // How much to exaggerate

	// Debugging
	ShowPlateVelocityArrows bool
	ShowConvectionCells     bool
	ShowStressField         bool
}

// DefaultAcceleratedParams returns params for visible plate tectonics
func DefaultAcceleratedParams() *AcceleratedPhysicsParams {
	return &AcceleratedPhysicsParams{
		ConvectionSpeedMultiplier: 1000000.0, // Million times faster
		PlateVelocityMultiplier:   1000000.0, // 3 cm/year -> 30 km/year
		ErosionRateMultiplier:     10000.0,
		ExaggerateTopography:      true,
		TopographyExaggeration:    10.0,
	}
}

// ApplyAcceleratedPlateMotion moves plates at visible speeds
func ApplyAcceleratedPlateMotion(planet *core.VoxelPlanet, dt float64, params *AcceleratedPhysicsParams) {
	if params == nil {
		return
	}

	// Work only on crustal shells
	for shellIdx := len(planet.Shells) - 3; shellIdx < len(planet.Shells)-1; shellIdx++ {
		if shellIdx < 0 {
			continue
		}

		shell := &planet.Shells[shellIdx]

		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Skip non-solid materials
				if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
					continue
				}

				// Set accelerated velocities based on material type
				// Base velocity: 3 cm/year = 1e-9 m/s
				// With 1M multiplier: 30 km/year = 1e-3 m/s
				baseVel := float32(3e-9) // 3 cm/year
				if voxel.Type == core.MatGranite {
					// Continental plates move a bit slower
					voxel.VelEast = baseVel * float32(params.PlateVelocityMultiplier) * (0.5 + 0.5*float32(lonIdx%3))
					voxel.VelNorth = baseVel * float32(params.PlateVelocityMultiplier) * 0.2 * (1.0 - 2.0*float32(latIdx%2))
				} else if voxel.Type == core.MatBasalt {
					// Oceanic plates can move faster
					voxel.VelEast = baseVel * float32(params.PlateVelocityMultiplier) * (0.8 + 0.4*float32(lonIdx%5)/5.0)
					voxel.VelNorth = baseVel * float32(params.PlateVelocityMultiplier) * 0.3 * (1.0 - 2.0*float32(latIdx%3)/3.0)
				}

				// For future use: calculate actual displacement
				// lat := core.GetLatitudeForBand(latIdx, shell.LatBands)
				// radius := (shell.InnerRadius + shell.OuterRadius) / 2
				// degPerMeter := 360.0 / (2.0 * math.Pi * radius)
				// lonDisplacement := float64(voxel.VelEast) * dt * degPerMeter / math.Cos(lat*math.Pi/180.0)

				// Add visible deformation at plate boundaries
				if voxel.Stress > 1e6 { // High stress = plate boundary
					// Push material up (mountain building) or down (subduction)
					if voxel.Type == core.MatGranite {
						voxel.VelR = float32(1e-3 * params.ConvectionSpeedMultiplier) // Uplift
					} else {
						voxel.VelR = float32(-1e-3 * params.ConvectionSpeedMultiplier) // Subduction
					}
				}
			}
		}
	}
}

// CreateVisibleHotspot adds a hotspot with exaggerated effects
func CreateVisibleHotspot(planet *core.VoxelPlanet, lat, lon float64, params *AcceleratedPhysicsParams) {
	// Find the mantle shell at the hotspot location
	for shellIdx := 5; shellIdx < len(planet.Shells)-3; shellIdx++ {
		shell := &planet.Shells[shellIdx]

		// Find the voxel closest to the hotspot
		latBand := int((lat + 90.0) / 180.0 * float64(shell.LatBands))
		if latBand >= shell.LatBands {
			latBand = shell.LatBands - 1
		}

		lonCount := len(shell.Voxels[latBand])
		lonIdx := int((lon + 180.0) / 360.0 * float64(lonCount))
		lonIdx = lonIdx % lonCount

		if latBand >= 0 && latBand < len(shell.Voxels) && lonIdx >= 0 && lonIdx < len(shell.Voxels[latBand]) {
			voxel := &shell.Voxels[latBand][lonIdx]

			// Make it hot and rising
			voxel.Temperature = 3000 // Very hot
			voxel.Type = core.MatMagma
			voxel.VelR = float32(0.1 * params.ConvectionSpeedMultiplier) // Strong upwelling

			// Heat neighbors too
			for dLat := -1; dLat <= 1; dLat++ {
				for dLon := -1; dLon <= 1; dLon++ {
					nLat := latBand + dLat
					nLon := (lonIdx + dLon + lonCount) % lonCount

					if nLat >= 0 && nLat < len(shell.Voxels) {
						neighbor := &shell.Voxels[nLat][nLon]
						neighbor.Temperature = 2500
						neighbor.VelR = float32(0.05 * params.ConvectionSpeedMultiplier)
					}
				}
			}
		}
	}
}

// VisualizeConvectionCells adds tracer particles or colors to show convection
func VisualizeConvectionCells(planet *core.VoxelPlanet) {
	// Color code by vertical velocity for visualization
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Use composition field to store visualization data
				// Red = upwelling, Blue = downwelling
				if voxel.VelR > 0 {
					voxel.Composition = float32(voxel.VelR * 1e6) // Scale for visibility
				} else {
					voxel.Composition = float32(voxel.VelR * 1e6)
				}
			}
		}
	}
}
