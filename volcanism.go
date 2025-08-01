package main

import (
	"math"
	"math/rand"
)

// Hotspot represents a mantle plume that creates volcanic islands
type Hotspot struct {
	Position  Vector3
	Intensity float64
	Age       float64
}

// applyVolcanism simulates volcanic activity
func applyVolcanism(planet Planet, deltaYears float64) Planet {
	scale := deltaYears / 1000000.0 // Convert to millions of years
	
	// Apply volcanism at convergent boundaries (subduction zones)
	planet = applySubductionVolcanism(planet, scale)
	
	// Apply volcanism at divergent boundaries (mid-ocean ridges)
	planet = applyDivergentVolcanism(planet, scale)
	
	// Apply hotspot volcanism (Hawaii-style)
	planet = applyHotspotVolcanism(planet, scale)
	
	return planet
}

// applySubductionVolcanism creates volcanic arcs at subduction zones
func applySubductionVolcanism(planet Planet, scale float64) Planet {
	for _, boundary := range planet.Boundaries {
		if boundary.Type != Convergent {
			continue
		}
		
		plate1 := planet.Plates[boundary.Plate1]
		plate2 := planet.Plates[boundary.Plate2]
		
		// Check for subduction (oceanic under continental)
		var volcanicPlate int
		if plate1.Type == Oceanic && plate2.Type == Continental {
			volcanicPlate = boundary.Plate2 // Volcanism on continental plate
		} else if plate1.Type == Continental && plate2.Type == Oceanic {
			volcanicPlate = boundary.Plate1 // Volcanism on continental plate
		} else {
			continue // No subduction volcanism
		}
		
		// Create volcanic arc parallel to boundary, offset inland
		for _, vertexIdx := range boundary.EdgeVertices {
			if vertexIdx >= len(planet.Vertices) {
				continue
			}
			
			// Find vertices on volcanic plate near boundary
			v := &planet.Vertices[vertexIdx]
			if v.PlateID != volcanicPlate {
				continue
			}
			
			// Volcanic activity probability
			// Use smoothed probability to avoid resonance at 1Myr/s
			prob := 0.01 * scale
			if prob > 0.005 {
				prob = 0.005 + (prob-0.005)*0.5 // Soften the probability curve
			}
			if rand.Float64() < prob {
				// Build volcanic cone with time-scaled uplift
				uplift := (0.001 + rand.Float64()*0.002) * math.Min(scale, 1.0)
				v.Height += uplift
				
				// Cap volcano height
				if v.Height > 0.05 {
					v.Height = 0.05
				}
				
				// Affect nearby vertices for cone shape
				neighbors := findVertexNeighbors(planet, vertexIdx)
				for _, n := range neighbors {
					if n < len(planet.Vertices) && planet.Vertices[n].PlateID == volcanicPlate {
						planet.Vertices[n].Height += uplift * 0.5
						if planet.Vertices[n].Height > 0.04 {
							planet.Vertices[n].Height = 0.04
						}
						
						// Position stays on unit sphere - height is separate
					}
				}
				
				// Position stays on unit sphere - height is separate
			}
		}
	}
	
	return planet
}

// applyDivergentVolcanism creates new crust at spreading centers
func applyDivergentVolcanism(planet Planet, scale float64) Planet {
	for _, boundary := range planet.Boundaries {
		if boundary.Type != Divergent {
			continue
		}
		
		// Add basaltic volcanism along ridge
		for _, vertexIdx := range boundary.EdgeVertices {
			if vertexIdx >= len(planet.Vertices) {
				continue
			}
			
			v := &planet.Vertices[vertexIdx]
			
			// Mid-ocean ridges are elevated but underwater
			if v.Height < -0.005 {
				// Small chance of pillow basalt formation
				// Smooth probability to avoid resonance
				prob := 0.05 * scale
				if prob > 0.02 {
					prob = 0.02 + (prob-0.02)*0.5
				}
				if rand.Float64() < prob {
					v.Height += 0.0002 * math.Min(scale, 1.0)
					
					// Keep underwater
					if v.Height > -0.003 {
						v.Height = -0.003
					}
					// Position stays on unit sphere - height is separate
				}
			}
		}
	}
	
	return planet
}

// applyHotspotVolcanism creates volcanic islands from mantle plumes
func applyHotspotVolcanism(planet Planet, scale float64) Planet {
	// scale is already in millions of years
	deltaYears := scale * 1000000.0 // Convert back to years for age tracking
	
	// Create random hotspots if we don't have any
	if len(planet.Hotspots) == 0 {
		numHotspots := 3 + rand.Intn(5) // 3-7 hotspots
		planet.Hotspots = make([]Hotspot, numHotspots)
		
		for i := range planet.Hotspots {
			// Random position on sphere
			theta := rand.Float64() * 2 * math.Pi
			phi := math.Acos(1 - 2*rand.Float64())
			
			planet.Hotspots[i] = Hotspot{
				Position: Vector3{
					X: math.Sin(phi) * math.Cos(theta),
					Y: math.Sin(phi) * math.Sin(theta),
					Z: math.Cos(phi),
				},
				Intensity: 0.5 + rand.Float64()*0.5,
				Age:       0,
			}
		}
	}
	
	// Apply hotspot volcanism
	for i := range planet.Hotspots {
		hotspot := &planet.Hotspots[i]
		hotspot.Age += deltaYears
		
		// Find nearest vertex
		minDist := math.MaxFloat64
		nearestVertex := -1
		
		for j := range planet.Vertices {
			dist := distance(planet.Vertices[j].Position.Normalize(), hotspot.Position)
			if dist < minDist {
				minDist = dist
				nearestVertex = j
			}
		}
		
		if nearestVertex >= 0 && minDist < 0.1 { // Within influence radius
			v := &planet.Vertices[nearestVertex]
			
			// Build volcanic island
			// Smooth probability to avoid resonance
			prob := hotspot.Intensity * scale * 0.1
			if prob > 0.03 {
				prob = 0.03 + (prob-0.03)*0.5
			}
			if rand.Float64() < prob {
				uplift := 0.0005 * hotspot.Intensity * math.Min(scale, 1.0)
				v.Height += uplift
				
				// Create shield volcano shape
				neighbors := findVertexNeighbors(planet, nearestVertex)
				for _, n := range neighbors {
					if n < len(planet.Vertices) {
						neighborDist := distance(planet.Vertices[n].Position.Normalize(), hotspot.Position)
						if neighborDist < 0.08 {
							falloff := 1.0 - (neighborDist / 0.08)
							planet.Vertices[n].Height += uplift * falloff * 0.7
							// Position stays on unit sphere - height is separate
						}
					}
				}
				
				// Cap height
				if v.Height > 0.04 {
					v.Height = 0.04
				}
				
				// Position stays on unit sphere - height is separate
			}
		}
		
		// Hotspots can fade over very long timescales
		if hotspot.Age > 100000000 { // 100 My
			hotspot.Intensity *= 0.99
		}
	}
	
	return planet
}

// Add hotspots to Planet struct - you'll need to add this to types.go:
// Hotspots []Hotspot