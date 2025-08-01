package main

import (
	"math/rand"
)

// improvedTectonicUpdate uses Voronoi for basic structure but improved interactions
func improvedTectonicUpdate(planet Planet, deltaYears float64) Planet {
	scale := deltaYears / 1000000.0
	
	// Move plate centers
	for i := range planet.Plates {
		plate := &planet.Plates[i]
		
		// Move plate center
		movement := plate.Velocity.Scale(scale)
		plate.Center = plate.Center.Add(movement).Normalize()
		
		// Occasional velocity adjustments
		if rand.Float64() < 0.01 {
			plate.Velocity = plate.Velocity.Add(Vector3{
				X: (rand.Float64() - 0.5) * 0.0001,
				Y: (rand.Float64() - 0.5) * 0.0001,
				Z: (rand.Float64() - 0.5) * 0.0001,
			})
			
			// Keep velocity tangent to sphere
			radial := plate.Center.Scale(plate.Velocity.Dot(plate.Center))
			plate.Velocity = plate.Velocity.Add(radial.Scale(-1))
		}
	}
	
	// Simple Voronoi assignment
	planet = updateVoronoiAssignment(planet)
	
	// Apply enhanced boundary interactions
	planet = applyEnhancedBoundaries(planet, deltaYears)
	
	return planet
}

// applyEnhancedBoundaries handles realistic subduction and collision
func applyEnhancedBoundaries(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	// Process existing boundaries
	for _, boundary := range planet.Boundaries {
		plate1 := &planet.Plates[boundary.Plate1]
		plate2 := &planet.Plates[boundary.Plate2]
		
		// Skip transform boundaries for performance
		if boundary.Type == Transform {
			continue
		}
		
		// Process boundary vertices
		for _, vIdx := range boundary.EdgeVertices {
			if vIdx >= len(planet.Vertices) {
				continue
			}
			
			v := &planet.Vertices[vIdx]
			
			if boundary.Type == Convergent {
				// Subduction logic
				if plate1.Type == Oceanic && plate2.Type == Continental {
					if v.PlateID == boundary.Plate1 {
						// Oceanic plate subducts
						v.Height -= 0.00003 * yearScale
					} else {
						// Volcanic arc on continental side
						if rand.Float64() < 0.05 {
							v.Height += 0.00005 * yearScale
						}
					}
				} else if plate1.Type == Continental && plate2.Type == Oceanic {
					if v.PlateID == boundary.Plate2 {
						// Oceanic plate subducts
						v.Height -= 0.00003 * yearScale
					} else {
						// Volcanic arc on continental side
						if rand.Float64() < 0.05 {
							v.Height += 0.00005 * yearScale
						}
					}
				} else if plate1.Type == Continental && plate2.Type == Continental {
					// Continental collision - both rise
					v.Height += 0.00008 * yearScale
				}
			} else if boundary.Type == Divergent {
				// Rifting
				v.Height -= 0.00002 * yearScale
				
				// New oceanic crust at mid-ocean ridges
				if v.Height < -0.005 && rand.Float64() < 0.1 {
					v.Height = -0.003
				}
			}
			
			// Clamp heights
			if v.Height > 0.08 {
				v.Height = 0.08
			} else if v.Height < -0.04 {
				v.Height = -0.04
			}
		}
	}
	
	return planet
}