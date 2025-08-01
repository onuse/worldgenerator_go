package main

import (
	"fmt"
	"math"
	"math/rand"
)

func updateTectonics(planet Planet, deltaYears float64) Planet {
	// Store old heights for spike prevention
	oldPlanet := Planet{
		Vertices: make([]Vertex, len(planet.Vertices)),
	}
	for i, v := range planet.Vertices {
		oldPlanet.Vertices[i].Height = v.Height
	}
	
	planet.GeologicalTime += deltaYears
	
	// Use realistic plate movement for high fidelity
	planet = updateRealisticPlatesSimple(planet, deltaYears)
	
	// Update boundaries based on actual vertex assignments
	if len(planet.Boundaries) == 0 || deltaYears > 10000 || int(planet.GeologicalTime) % 100000 == 0 {
		planet.Boundaries = findPlateBoundaries(planet)
		
		// Debug info
		if deltaYears > 1000000 {
			convergent, divergent, transform := 0, 0, 0
			for _, b := range planet.Boundaries {
				switch b.Type {
				case Convergent: convergent++
				case Divergent: divergent++
				case Transform: transform++
				}
			}
			fmt.Printf("DEBUG: %.0f My - Plates:%d Conv:%d Div:%d Trans:%d\n", 
				planet.GeologicalTime/1000000.0, len(planet.Plates), convergent, divergent, transform)
		}
	}
	
	// Apply volcanic activity (less frequent for large time steps)
	if deltaYears < 1000000 {
		planet = applyVolcanism(planet, deltaYears)
	} else {
		// For very large time steps, apply volcanism less frequently
		if int(planet.GeologicalTime) % 5000000 == 0 {
			planet = applyVolcanism(planet, 5000000)
		}
	}
	
	// Apply erosion (only for time steps > 1000 years)
	if deltaYears > 1000 {
		planet = applyErosion(planet, deltaYears)
		if deltaYears < 1000000 { // Skip sedimentation for very large time steps
			planet = applySedimentation(planet, deltaYears)
			planet = applyIsostasticAdjustment(planet, deltaYears)
		}
	}
	
	// Prevent spikes - clamp height changes based on time step
	maxChange := 0.001 * (deltaYears / 1000.0) // Scale with time
	if maxChange > 0.01 {
		maxChange = 0.01 // Cap maximum change per frame
	}
	planet = clampHeightChanges(planet, oldPlanet, maxChange)
	
	// Apply smoothing for realistic terrain at all speeds
	iterations := 2 // Base smoothing for realistic features
	if deltaYears >= 1000000 {
		iterations = 3 // Extra smoothing at very high speeds
	}
	planet = smoothHeights(planet, iterations)
	
	// Preserve minimum landmass (30% of Earth's surface is land)
	planet = preserveLandmass(planet, 0.3)
	
	// Positions should never change - only heights are modified
	
	return planet
}

func movePlates(planet Planet, deltaYears float64) Planet {
	// Scale movement for visible changes
	movementScale := deltaYears / 1000000.0 // Years to millions of years
	
	for i := range planet.Plates {
		// Move plate center
		movement := planet.Plates[i].Velocity.Scale(movementScale)
		planet.Plates[i].Center = planet.Plates[i].Center.Add(movement)
		planet.Plates[i].Center = planet.Plates[i].Center.Normalize()
		
		// Gradually adjust velocities for more realistic movement
		// Add small random perturbations
		if rand.Float64() < 0.01 { // 1% chance per update
			perturbation := Vector3{
				X: (rand.Float64() - 0.5) * 0.0002,
				Y: (rand.Float64() - 0.5) * 0.0002,
				Z: (rand.Float64() - 0.5) * 0.0002,
			}
			planet.Plates[i].Velocity = planet.Plates[i].Velocity.Add(perturbation)
			
			// Keep velocity tangent to sphere
			center := planet.Plates[i].Center
			vel := planet.Plates[i].Velocity
			// Remove radial component
			radialComponent := center.Scale(vel.Dot(center))
			planet.Plates[i].Velocity = vel.Add(radialComponent.Scale(-1))
		}
	}
	
	// Reassign vertices to plates after movement
	// This ensures boundaries visually update as plates move
	planet = updateVoronoiAssignment(planet)
	
	return planet
}

func updateVoronoiAssignment(planet Planet) Planet {
	// Clear existing assignments
	for i := range planet.Plates {
		planet.Plates[i].Vertices = []int{}
	}
	
	// Reassign vertices to nearest plate
	for i := range planet.Vertices {
		minDist := math.MaxFloat64
		closestPlate := 0
		
		for j, plate := range planet.Plates {
			dist := distance(planet.Vertices[i].Position.Normalize(), plate.Center)
			if dist < minDist {
				minDist = dist
				closestPlate = j
			}
		}
		
		planet.Vertices[i].PlateID = closestPlate
		planet.Plates[closestPlate].Vertices = append(planet.Plates[closestPlate].Vertices, i)
	}
	
	return planet
}

func applyTectonicProcesses(planet Planet, deltaYears float64) Planet {
	for _, boundary := range planet.Boundaries {
		switch boundary.Type {
		case Convergent:
			planet = applyConvergentBoundary(planet, boundary, deltaYears)
		case Divergent:
			planet = applyDivergentBoundary(planet, boundary, deltaYears)
		case Transform:
			planet = applyTransformBoundary(planet, boundary, deltaYears)
		}
	}
	
	return planet
}

func applyConvergentBoundary(planet Planet, boundary PlateBoundary, deltaYears float64) Planet {
	plate1 := planet.Plates[boundary.Plate1]
	plate2 := planet.Plates[boundary.Plate2]
	
	// Calculate convergence rate
	relVel := plate1.Velocity.Add(plate2.Velocity.Scale(-1))
	direction := plate2.Center.Add(plate1.Center.Scale(-1)).Normalize()
	convergenceRate := -relVel.Dot(direction)
	
	if convergenceRate <= 0 {
		return planet
	}
	
	// Scale rates based on time step
	// Real plate velocities: 2-10 cm/year
	// convergenceRate is already in simulation units
	
	// Base rates in meters per year
	var baseUpliftRate, baseSubductRate float64
	
	if plate1.Type == Continental && plate2.Type == Continental {
		// Continental-continental: Himalayas grow ~1cm/year
		baseUpliftRate = 0.01 * convergenceRate / 0.01  // Normalized to typical convergence
		baseSubductRate = 0.002 * convergenceRate / 0.01
	} else if (plate1.Type == Oceanic && plate2.Type == Continental) || 
		      (plate1.Type == Continental && plate2.Type == Oceanic) {
		// Subduction zones: Andes grow ~0.5cm/year
		baseUpliftRate = 0.005 * convergenceRate / 0.01
		baseSubductRate = 0.02 * convergenceRate / 0.01  // Oceanic plate sinks faster
	} else {
		// Oceanic-oceanic: Island arcs
		baseUpliftRate = 0.003 * convergenceRate / 0.01
		baseSubductRate = 0.015 * convergenceRate / 0.01
	}
	
	// Convert to simulation units and apply time step
	// 1 unit height = ~8000m (roughly Earth radius / 800)
	upliftRate := (baseUpliftRate / 8000.0) * deltaYears
	subductRate := (baseSubductRate / 8000.0) * deltaYears
	
	// Determine subduction direction
	var upperPlate, lowerPlate int
	
	if plate1.Type == Oceanic && plate2.Type == Continental {
		upperPlate, lowerPlate = boundary.Plate2, boundary.Plate1
	} else if plate1.Type == Continental && plate2.Type == Oceanic {
		upperPlate, lowerPlate = boundary.Plate1, boundary.Plate2
	} else if plate1.Type == Continental && plate2.Type == Continental {
		// Both uplift in continental collision
		upperPlate, lowerPlate = boundary.Plate1, boundary.Plate2
	} else {
		// Oceanic-oceanic: older/denser subducts (use ID as proxy)
		if boundary.Plate1 < boundary.Plate2 {
			upperPlate, lowerPlate = boundary.Plate2, boundary.Plate1
		} else {
			upperPlate, lowerPlate = boundary.Plate1, boundary.Plate2
		}
	}
	
	// Apply deformation near boundaries with broader influence
	for _, vertexIdx := range boundary.EdgeVertices {
		if vertexIdx < len(planet.Vertices) {
			// Direct boundary vertices get maximum effect
			v := &planet.Vertices[vertexIdx]
			
			if v.PlateID == upperPlate {
				v.Height += upliftRate * 0.0001 // Reduced rate to prevent spikes
				// Cap maximum height
				if v.Height > 0.08 {
					v.Height = 0.08
				}
			} else if v.PlateID == lowerPlate {
				v.Height -= subductRate * 0.00008
				// Cap minimum depth
				if v.Height < -0.04 {
					v.Height = -0.04
				}
			}
			
			// Position stays on unit sphere - height is separate
		}
	}
	
	// Broader influence zone around boundaries
	influenceDistance := 0.15 // Sphere units
	
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Skip if not on either plate
		if v.PlateID != upperPlate && v.PlateID != lowerPlate {
			continue
		}
		
		// Find distance to nearest boundary vertex
		minDist := math.MaxFloat64
		for _, bIdx := range boundary.EdgeVertices {
			if bIdx < len(planet.Vertices) {
				dist := distance(v.Position, planet.Vertices[bIdx].Position)
				if dist < minDist {
					minDist = dist
				}
			}
		}
		
		if minDist < influenceDistance {
			// Smooth falloff from boundary
			falloff := 1.0 - (minDist / influenceDistance)
			falloff *= falloff // Quadratic falloff
			
			if v.PlateID == upperPlate {
				v.Height += upliftRate * 0.00005 * falloff
				if v.Height > 0.08 {
					v.Height = 0.08
				}
			} else {
				v.Height -= subductRate * 0.00004 * falloff
				if v.Height < -0.04 {
					v.Height = -0.04
				}
			}
			// Position stays on unit sphere - height is separate
		}
	}
	
	return planet
}

func applyDivergentBoundary(planet Planet, boundary PlateBoundary, deltaYears float64) Planet {
	plate1 := planet.Plates[boundary.Plate1]
	plate2 := planet.Plates[boundary.Plate2]
	
	// Calculate divergence rate
	relVel := plate1.Velocity.Add(plate2.Velocity.Scale(-1))
	direction := plate2.Center.Add(plate1.Center.Scale(-1)).Normalize()
	divergenceRate := relVel.Dot(direction)
	
	if divergenceRate <= 0 {
		return planet
	}
	
	yearlyRate := deltaYears / 1000000.0
	
	// Divergent boundaries create rifts and new crust
	riftRate := divergenceRate * 0.4 * yearlyRate
	upliftRate := divergenceRate * 0.2 * yearlyRate // New crust is elevated
	
	// Apply rifting along boundary
	for _, vertexIdx := range boundary.EdgeVertices {
		if vertexIdx < len(planet.Vertices) {
			v := &planet.Vertices[vertexIdx]
			
			// Create rift valley in continental crust
			if v.Height > -0.005 { // Continental or shallow
				v.Height -= riftRate * 0.00008
			} else { // Oceanic - create mid-ocean ridge
				v.Height += upliftRate * 0.00004
			}
			
			// Cap heights
			if v.Height < -0.04 {
				v.Height = -0.04
			} else if v.Height > 0.08 {
				v.Height = 0.08
			}
			// Position stays on unit sphere - height is separate
		}
	}
	
	// Create broader rift zone
	influenceDistance := 0.1
	
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Only affect vertices on diverging plates
		if v.PlateID != boundary.Plate1 && v.PlateID != boundary.Plate2 {
			continue
		}
		
		// Find distance to boundary
		minDist := math.MaxFloat64
		for _, bIdx := range boundary.EdgeVertices {
			if bIdx < len(planet.Vertices) {
				dist := distance(v.Position, planet.Vertices[bIdx].Position)
				if dist < minDist {
					minDist = dist
				}
			}
		}
		
		if minDist < influenceDistance {
			falloff := 1.0 - (minDist / influenceDistance)
			falloff *= falloff
			
			if v.Height > -0.005 { // Continental rifting
				v.Height -= riftRate * 0.0004 * falloff
			} else { // Oceanic spreading
				v.Height += upliftRate * 0.0002 * falloff
			}
			
			if v.Height < -0.04 {
				v.Height = -0.04
			} else if v.Height > 0.08 {
				v.Height = 0.08
			}
			// Position stays on unit sphere - height is separate
		}
	}
	
	return planet
}

func applyTransformBoundary(planet Planet, boundary PlateBoundary, deltaYears float64) Planet {
	// Transform boundaries create minor vertical changes due to friction
	yearlyRate := deltaYears / 1000000.0
	
	// Small random vertical displacement along fault
	for _, vertexIdx := range boundary.EdgeVertices {
		if vertexIdx < len(planet.Vertices) {
			v := &planet.Vertices[vertexIdx]
			
			// Minor vertical displacement from friction and grinding
			noise := (rand.Float64() - 0.5) * 0.0001 * yearlyRate
			v.Height += noise
			
			// Keep within reasonable bounds
			if v.Height < -0.04 {
				v.Height = -0.04
			} else if v.Height > 0.08 {
				v.Height = 0.08
			}
			// Position stays on unit sphere - height is separate
		}
	}
	
	return planet
}

func minDistanceToBoundary(pos Vector3, boundary PlateBoundary, planet Planet) float64 {
	minDist := math.MaxFloat64
	for _, vertexIdx := range boundary.EdgeVertices {
		if vertexIdx < len(planet.Vertices) {
			dist := distance(pos, planet.Vertices[vertexIdx].Position)
			if dist < minDist {
				minDist = dist
			}
		}
	}
	return minDist
}