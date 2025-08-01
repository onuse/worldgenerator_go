package main

import (
	"math"
	"math/rand"
)

// updateRealisticPlatesSimple allows plates to move more freely
func updateRealisticPlatesSimple(planet Planet, deltaYears float64) Planet {
	scale := deltaYears / 1000000.0
	
	// Move each plate's center
	for i := range planet.Plates {
		plate := &planet.Plates[i]
		
		// Move plate center
		movement := plate.Velocity.Scale(scale)
		plate.Center = plate.Center.Add(movement).Normalize()
		
		// Add random perturbations for more dynamic behavior
		// Scale probability with time step to avoid resonance
		perturbProb := 0.05 * math.Min(deltaYears/100000.0, 1.0)
		if rand.Float64() < perturbProb {
			plate.Velocity = plate.Velocity.Add(Vector3{
				X: (rand.Float64() - 0.5) * 0.0005, // Larger perturbations
				Y: (rand.Float64() - 0.5) * 0.0005,
				Z: (rand.Float64() - 0.5) * 0.0005,
			})
			
			// Keep velocity tangent to sphere
			radial := plate.Center.Scale(plate.Velocity.Dot(plate.Center))
			plate.Velocity = plate.Velocity.Add(radial.Scale(-1))
		}
	}
	
	// Update vertex ownership based on multiple factors
	planet = updateVertexOwnership(planet, deltaYears)
	
	// Apply boundary interactions
	planet = applyBoundaryInteractions(planet, deltaYears)
	
	return planet
}

// updateVertexOwnership assigns vertices based on proximity and plate strength
func updateVertexOwnership(planet Planet, deltaYears float64) Planet {
	// Update vertex ownership based on time elapsed, not modulo
	// This avoids resonance at specific speeds
	if !planet.NeedsOwnershipUpdate && deltaYears < 100000 {
		return planet
	}
	
	// Reset the flag
	planet.NeedsOwnershipUpdate = false
	
	// Build a set of vertices near boundaries that need updating
	boundaryVertices := make(map[int]bool)
	
	// First pass: find vertices at boundaries
	for i := 0; i < len(planet.Indices); i += 3 {
		v0, v1, v2 := int(planet.Indices[i]), int(planet.Indices[i+1]), int(planet.Indices[i+2])
		
		p0 := planet.Vertices[v0].PlateID
		p1 := planet.Vertices[v1].PlateID
		p2 := planet.Vertices[v2].PlateID
		
		// If triangle spans plates, all vertices are near boundary
		if p0 != p1 || p1 != p2 || p0 != p2 {
			boundaryVertices[v0] = true
			boundaryVertices[v1] = true
			boundaryVertices[v2] = true
		}
	}
	
	// Expand boundary region by one neighbor ring
	expanded := make(map[int]bool)
	if planet.NeighborCache == nil {
		planet = buildNeighborCache(planet)
	}
	
	for vIdx := range boundaryVertices {
		expanded[vIdx] = true
		if neighbors, ok := planet.NeighborCache[vIdx]; ok {
			for _, n := range neighbors {
				expanded[n] = true
			}
		}
	}
	
	// Convert to slice for GPU
	boundaryList := make([]int, 0, len(expanded))
	for vIdx := range expanded {
		boundaryList = append(boundaryList, vIdx)
	}
	
	// Try GPU acceleration if available
	if simpleMetalGPU != nil && simpleMetalGPU.enabled {
		simpleMetalGPU.accelerateVertexOwnership(&planet, boundaryList)
	} else {
		// CPU fallback
		for i := range expanded {
			v := &planet.Vertices[i]
			
			maxInfluence := 0.0
			bestPlate := v.PlateID
			currentPlateInfluence := 0.0
			
			// Only check nearby plates for efficiency
			nearbyPlates := make(map[int]bool)
			nearbyPlates[v.PlateID] = true
			
			// Add plates of neighbors
			if neighbors, ok := planet.NeighborCache[i]; ok {
				for _, n := range neighbors {
					if n < len(planet.Vertices) {
						nearbyPlates[planet.Vertices[n].PlateID] = true
					}
				}
			}
			
			for pIdx := range nearbyPlates {
				if pIdx >= len(planet.Plates) {
					continue
				}
				plate := planet.Plates[pIdx]
				
				// Distance-based influence
				dist := distance(v.Position, plate.Center)
				influence := 1.0 / (1.0 + dist*dist*10.0)
				
				// Bonus for current plate (inertia)
				if pIdx == v.PlateID {
					influence *= 1.5
					currentPlateInfluence = influence
				}
				
				// Continental plates have stronger influence over oceanic
				if plate.Type == Continental && v.PlateID < len(planet.Plates) && 
				   planet.Plates[v.PlateID].Type == Oceanic {
					influence *= 1.3
				}
				
				if influence > maxInfluence {
					maxInfluence = influence
					bestPlate = pIdx
				}
			}
			
			// Only change if significantly better
			if bestPlate != v.PlateID && maxInfluence > currentPlateInfluence * 1.2 {
				v.PlateID = bestPlate
			}
		}
	}
	
	// Update plate vertex lists
	for i := range planet.Plates {
		planet.Plates[i].Vertices = []int{}
	}
	for i, v := range planet.Vertices {
		if v.PlateID >= 0 && v.PlateID < len(planet.Plates) {
			planet.Plates[v.PlateID].Vertices = append(planet.Plates[v.PlateID].Vertices, i)
		}
	}
	
	return planet
}

// applyBoundaryInteractions handles subduction, collision, rifting
func applyBoundaryInteractions(planet Planet, deltaYears float64) Planet {
	yearScale := deltaYears / 1000000.0
	
	// Build neighbor cache if not exists
	if planet.NeighborCache == nil {
		planet = buildNeighborCache(planet)
	}
	
	// Track height changes to apply them once at the end
	heightDeltas := make([]float64, len(planet.Vertices))
	
	// Find and process boundaries
	for i := range planet.Vertices {
		v := &planet.Vertices[i]
		
		// Check neighbors for different plates
		neighbors, ok := planet.NeighborCache[i]
		if !ok {
			continue
		}
		for _, nIdx := range neighbors {
			if nIdx >= len(planet.Vertices) {
				continue
			}
			
			n := &planet.Vertices[nIdx]
			if n.PlateID != v.PlateID {
				plate1 := &planet.Plates[v.PlateID]
				plate2 := &planet.Plates[n.PlateID]
				
				// Calculate relative motion
				relVel := plate1.Velocity.Add(plate2.Velocity.Scale(-1))
				direction := n.Position.Add(v.Position.Scale(-1)).Normalize()
				convergence := relVel.Dot(direction)
				
				// Add deterministic variation based on position for varied terrain
				// This prevents frame-to-frame randomness that causes spikes
				posVar := math.Abs(v.Position.X*3.14159 + n.Position.Y*2.71828 + v.Position.Z*1.41421)
				randomFactor := 0.8 + (math.Sin(posVar)*0.5+0.5)*0.4 // 0.8 to 1.2 deterministic
				
				// Subduction: oceanic under continental
				if plate1.Type == Oceanic && plate2.Type == Continental && convergence > 0 {
					// Oceanic plate goes down to form deep trench
					heightDeltas[i] -= 0.00008 * yearScale * convergence * randomFactor
					// Volcanic arc and varied mountain building on continental side
					// Use continuous probability scaling to avoid discrete jumps
					volcanicStrength := 0.4 * convergence * math.Min(yearScale, 1.0)
					if volcanicStrength > 0.001 {
						// Create varied heights for interesting terrain
						mountainHeight := (0.00005 + rand.Float64() * 0.00015) * volcanicStrength
						heightDeltas[nIdx] += mountainHeight * yearScale
					}
				} else if plate1.Type == Continental && plate2.Type == Oceanic && convergence < 0 {
					// Other direction
					heightDeltas[nIdx] -= 0.00008 * yearScale * math.Abs(convergence) * randomFactor
					// Use continuous probability scaling to avoid discrete jumps
					volcanicStrength := 0.4 * math.Abs(convergence) * math.Min(yearScale, 1.0)
					if volcanicStrength > 0.001 {
						mountainHeight := (0.00005 + rand.Float64() * 0.00015) * volcanicStrength
						heightDeltas[i] += mountainHeight * yearScale
					}
				} else if plate1.Type == Continental && plate2.Type == Continental && math.Abs(convergence) > 0.0001 {
					// Continental collision - major mountain building (like Himalayas)
					// Create mountain ranges with varied peaks
					baseUplift := 0.0001 * yearScale * math.Abs(convergence)
					// Use vertex position to create deterministic variation instead of random
					posHash := math.Abs(v.Position.X*7919 + v.Position.Y*7927 + v.Position.Z*7933)
					peakVariation := 1.0 + (math.Sin(posHash)*0.5+0.5)*0.5 // Deterministic 1.0-1.5
					heightDeltas[i] += baseUplift * peakVariation * randomFactor
					heightDeltas[nIdx] += baseUplift * (2.0 - peakVariation) * randomFactor // Complementary variation
				} else if convergence < -0.0001 {
					// Divergent boundary - rifting
					subsidence := 0.00004 * yearScale * math.Abs(convergence) * randomFactor
					heightDeltas[i] -= subsidence
					heightDeltas[nIdx] -= subsidence
				}
			}
		}
	}
	
	// Apply accumulated height changes with limits to prevent spikes
	for i := range planet.Vertices {
		// Limit the maximum height change per frame to prevent spikes
		maxDelta := 0.005 * yearScale // Max 5mm per million years
		if heightDeltas[i] > maxDelta {
			heightDeltas[i] = maxDelta
		} else if heightDeltas[i] < -maxDelta {
			heightDeltas[i] = -maxDelta
		}
		
		planet.Vertices[i].Height += heightDeltas[i]
	}
	
	return planet
}

// getSimpleNeighbors finds neighboring vertices from triangles
func getSimpleNeighbors(planet Planet, vIdx int) []int {
	// This function is now mostly unused since we cache neighbors
	// Kept for compatibility
	if planet.NeighborCache != nil {
		if neighbors, ok := planet.NeighborCache[vIdx]; ok {
			return neighbors
		}
	}
	
	neighbors := make(map[int]bool)
	
	for i := 0; i < len(planet.Indices); i += 3 {
		v0, v1, v2 := int(planet.Indices[i]), int(planet.Indices[i+1]), int(planet.Indices[i+2])
		
		if v0 == vIdx {
			neighbors[v1] = true
			neighbors[v2] = true
		} else if v1 == vIdx {
			neighbors[v0] = true
			neighbors[v2] = true
		} else if v2 == vIdx {
			neighbors[v0] = true
			neighbors[v1] = true
		}
	}
	
	result := make([]int, 0, len(neighbors))
	for n := range neighbors {
		result = append(result, n)
	}
	return result
}