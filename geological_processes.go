package main

import (
	"math"
	"math/rand"
)

// applyAdvancedHotspotVolcanism creates volcanic island chains with more variety
func applyAdvancedHotspotVolcanism(planet Planet, deltaYears float64) Planet {
	// Initialize hotspots if not exists
	if planet.Hotspots == nil {
		planet.Hotspots = []Hotspot{}
		// Create 5-10 random hotspots for more volcanic activity
		numHotspots := 5 + rand.Intn(6)
		for i := 0; i < numHotspots; i++ {
			// Random position on sphere
			theta := rand.Float64() * 2 * math.Pi
			phi := math.Acos(2*rand.Float64() - 1)
			planet.Hotspots = append(planet.Hotspots, Hotspot{
				Position: Vector3{
					X: math.Sin(phi) * math.Cos(theta),
					Y: math.Sin(phi) * math.Sin(theta),
					Z: math.Cos(phi),
				},
				Intensity: 0.5 + rand.Float64()*0.5, // 0.5 to 1.0
				Age:       0,
			})
		}
	}
	
	yearScale := deltaYears / 1000000.0
	
	// Track height changes to prevent multiple updates
	heightDeltas := make([]float64, len(planet.Vertices))
	
	// Apply hotspot effects
	for _, hotspot := range planet.Hotspots {
		// Find vertices near hotspot
		for i := range planet.Vertices {
			v := &planet.Vertices[i]
			dist := distance(v.Position, hotspot.Position)
			
			if dist < 0.15 { // Larger hotspot influence radius
				// Create volcanic peak with height based on distance
				uplift := hotspot.Intensity * 0.0002 * yearScale * (1.0 - dist/0.15)
				
				// Hotspots can build tall islands even in deep ocean
				if v.Height < 0 {
					uplift *= 3.0 // Extra building in ocean to create islands
				}
				
				heightDeltas[i] += uplift
				
				// Add some randomness for varied peaks
				if rand.Float64() < 0.2 {
					heightDeltas[i] += uplift * rand.Float64() * 0.5
				}
			}
		}
		
		// Age the hotspot
		hotspot.Age += deltaYears
	}
	
	// Apply accumulated height changes with reasonable limits
	for i := range planet.Vertices {
		// Limit volcanic growth rate
		maxVolcanicGrowth := 0.01 * yearScale
		if heightDeltas[i] > maxVolcanicGrowth {
			heightDeltas[i] = maxVolcanicGrowth
		}
		planet.Vertices[i].Height += heightDeltas[i]
	}
	
	return planet
}

// applyCratonStability makes ancient continental cores more resistant to change
func applyCratonStability(planet Planet) Planet {
	// Mark old, stable continental areas
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Continental areas that have been above sea level for a long time
		if v.PlateID >= 0 && v.PlateID < len(planet.Plates) {
			if planet.Plates[v.PlateID].Type == Continental && v.Height > 0.005 {
				// Mark as craton if it's been stable
				// (In a full implementation, we'd track age)
				v.IsCraton = true
			}
		}
	}
	
	return planet
}

// applyDeepOceanTrenches creates deeper trenches at subduction zones
func applyDeepOceanTrenches(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	// Check plate boundaries for subduction zones
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Skip if not near a boundary
		if planet.NeighborCache == nil {
			continue
		}
		
		neighbors, ok := planet.NeighborCache[i]
		if !ok {
			continue
		}
		
		for _, nIdx := range neighbors {
			if nIdx >= len(planet.Vertices) {
				continue
			}
			
			n := &planet.Vertices[nIdx]
			if n.PlateID != v.PlateID && v.PlateID < len(planet.Plates) && n.PlateID < len(planet.Plates) {
				plate1 := planet.Plates[v.PlateID]
				plate2 := planet.Plates[n.PlateID]
				
				// Oceanic-continental subduction creates deep trenches
				if plate1.Type == Oceanic && plate2.Type == Continental {
					// Deepen the trench significantly
					if v.Height < 0.01 && v.Height > -0.08 {
						v.Height -= 0.00008 * yearScale
					}
				} else if plate1.Type == Oceanic && plate2.Type == Oceanic {
					// Oceanic-oceanic boundaries also create trenches
					if v.Height < 0 && v.Height > -0.06 {
						v.Height -= 0.00005 * yearScale
					}
				}
			}
		}
	}
	
	return planet
}

// applyHeightDependentErosion erodes based on elevation and slope
func applyHeightDependentErosion(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Skip cratons - they erode very slowly
		if v.IsCraton {
			continue
		}
		
		// Higher elevations erode faster (thinner air, more freeze-thaw)
		if v.Height > 0.05 { // Very high mountains
			erosionRate := 0.000008 * yearScale * (v.Height / 0.1)
			v.Height -= erosionRate
		} else if v.Height > 0.02 { // Mountains
			erosionRate := 0.000004 * yearScale
			v.Height -= erosionRate
		} else if v.Height > 0.005 { // Hills
			erosionRate := 0.000002 * yearScale
			v.Height -= erosionRate
		} else if v.Height > 0 { // Lowlands
			erosionRate := 0.000001 * yearScale
			v.Height -= erosionRate
		}
		
		// Calculate slope if we have neighbors
		if planet.NeighborCache != nil {
			if neighbors, ok := planet.NeighborCache[i]; ok {
				maxSlope := 0.0
				for _, nIdx := range neighbors {
					if nIdx < len(planet.Vertices) {
						slope := math.Abs(v.Height - planet.Vertices[nIdx].Height)
						if slope > maxSlope {
							maxSlope = slope
						}
					}
				}
				
				// Steep slopes erode faster
				if maxSlope > 0.01 {
					v.Height -= maxSlope * 0.00001 * yearScale
				}
			}
		}
	}
	
	return planet
}

// applySedimentDeposition fills in low areas with eroded material
func applySedimentDeposition(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Deposition in low areas and ocean basins
		if v.Height < -0.002 { // Ocean basins
			// Slow accumulation of sediment
			v.Height += 0.0000005 * yearScale
		} else if v.Height < 0.001 && v.Height > -0.002 { // Coastal areas
			// Faster deposition in shallow water
			v.Height += 0.000001 * yearScale
		}
		
		// River deltas and alluvial plains
		if v.Height > 0 && v.Height < 0.003 {
			// Check if in a low area surrounded by higher terrain
			if planet.NeighborCache != nil {
				if neighbors, ok := planet.NeighborCache[i]; ok {
					higherNeighbors := 0
					for _, nIdx := range neighbors {
						if nIdx < len(planet.Vertices) && planet.Vertices[nIdx].Height > v.Height + 0.002 {
							higherNeighbors++
						}
					}
					
					// If surrounded by higher terrain, accumulate sediment
					if higherNeighbors >= 2 {
						v.Height += 0.000002 * yearScale
					}
				}
			}
		}
	}
	
	return planet
}

// applyIsostasticRebound causes crust to rise when weight is removed
func applyIsostasticRebound(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Areas that have lost height (erosion/melting) rebound slowly
		// This is simplified - real isostasy is more complex
		if v.Height < 0.01 && v.Height > -0.01 {
			// Near sea level areas rebound to maintain equilibrium
			if v.PlateID >= 0 && v.PlateID < len(planet.Plates) {
				if planet.Plates[v.PlateID].Type == Continental {
					// Continental crust rebounds
					v.Height += 0.0000003 * yearScale
				}
			}
		}
		
		// Deep ocean basins also rebound very slowly
		if v.Height < -0.02 {
			v.Height += 0.0000001 * yearScale
		}
	}
	
	return planet
}