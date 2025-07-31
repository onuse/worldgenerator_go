package main

import (
	"math"
	"math/rand"
)

// applyErosion simulates erosion processes over geological time
func applyErosion(planet Planet, deltaYears float64) Planet {
	// Scale erosion based on time step
	erosionScale := math.Min(deltaYears / 1000000.0, 1.0) // Cap at 1 My worth of erosion per step
	
	// Apply different erosion processes
	planet = applyFluvialErosion(planet, erosionScale)
	planet = applyCoastalErosion(planet, erosionScale)
	planet = applyGlacialErosion(planet, erosionScale)
	
	return planet
}

// applyFluvialErosion simulates river and rainfall erosion
func applyFluvialErosion(planet Planet, scale float64) Planet {
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Only erode above sea level
		if v.Height > 0 {
			// Higher elevations erode faster (more rainfall, steeper slopes)
			erosionRate := 0.00001 * scale
			if v.Height > 0.02 { // Mountains
				erosionRate *= 3.0
			} else if v.Height > 0.01 { // Hills
				erosionRate *= 2.0
			}
			
			// Random variation
			erosionRate *= (0.5 + rand.Float64())
			
			// Apply erosion
			v.Height -= erosionRate
			if v.Height < -0.001 { // Don't erode below sea level quickly
				v.Height = -0.001
			}
			
			// Position stays on unit sphere - height is separate
		}
	}
	
	return planet
}

// applyCoastalErosion simulates wave action on coastlines
func applyCoastalErosion(planet Planet, scale float64) Planet {
	// Find coastal vertices (near sea level)
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Coastal zone: -0.002 to 0.002
		if v.Height > -0.002 && v.Height < 0.002 {
			// Check if near water by looking at neighbors
			hasWaterNeighbor := false
			hasLandNeighbor := false
			
			neighbors := findVertexNeighbors(planet, i)
			for _, n := range neighbors {
				if n < len(planet.Vertices) {
					if planet.Vertices[n].Height < -0.001 {
						hasWaterNeighbor = true
					}
					if planet.Vertices[n].Height > 0.001 {
						hasLandNeighbor = true
					}
				}
			}
			
			// Erode coastlines
			if hasWaterNeighbor && hasLandNeighbor {
				erosionRate := 0.00002 * scale * (0.5 + rand.Float64())
				v.Height -= erosionRate
				
				// Smooth coastal areas
				if v.Height < -0.0005 && v.Height > -0.002 {
					v.Height *= 0.98 // Gentle smoothing
				}
				
				// Position stays on unit sphere - height is separate
			}
		}
	}
	
	return planet
}

// applyGlacialErosion simulates ice sheet erosion at high elevations
func applyGlacialErosion(planet Planet, scale float64) Planet {
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Glacial erosion at high elevations and polar regions
		polarFactor := math.Abs(v.Position.Y) // Y is up axis
		
		if v.Height > 0.03 || (v.Height > 0.01 && polarFactor > 0.7) {
			// Glaciers carve deep valleys
			erosionRate := 0.00003 * scale * (0.5 + rand.Float64())
			
			// Polar regions have more glaciation
			if polarFactor > 0.7 {
				erosionRate *= 1.5
			}
			
			v.Height -= erosionRate
			
			// Glacial valleys are U-shaped, so we smooth nearby areas
			radius := 1.0 + v.Height
			v.Position = v.Position.Normalize().Scale(radius)
		}
	}
	
	return planet
}

// applySedimentation fills in low areas with eroded material
func applySedimentation(planet Planet, scale float64) Planet {
	// Simple sedimentation in ocean basins
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Deep ocean basins slowly fill with sediment
		if v.Height < -0.02 {
			sedimentRate := 0.000002 * scale * rand.Float64()
			v.Height += sedimentRate
			
			radius := 1.0 + v.Height
			v.Position = v.Position.Normalize().Scale(radius)
		}
	}
	
	return planet
}

// applyIsostasticAdjustment adjusts crust based on weight changes
func applyIsostasticAdjustment(planet Planet, scale float64) Planet {
	// Simple isostatic adjustment
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Mountains sink slightly, ocean basins rise slightly
		if v.Height > 0.02 {
			// Heavy mountains depress the crust
			v.Height -= 0.000001 * scale
		} else if v.Height < -0.01 {
			// Light water allows crust to rise
			v.Height += 0.0000005 * scale
		}
		
		// Position stays on unit sphere - height is separate
	}
	
	return planet
}