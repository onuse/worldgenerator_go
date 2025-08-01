package main

import (
	"math"
)

// smoothHeights applies spatial smoothing to prevent spikes
func smoothHeights(planet Planet, iterations int) Planet {
	// Build neighbor map if not already built
	if planet.NeighborCache == nil {
		planet = buildNeighborCache(planet)
	}
	
	for iter := 0; iter < iterations; iter++ {
		// Create temporary height array
		newHeights := make([]float64, len(planet.Vertices))
		
		for i, v := range planet.Vertices {
			// Average with neighbors
			sum := v.Height * 2.0 // Weight current height more
			count := 2.0
			
			if neighbors, ok := planet.NeighborCache[i]; ok {
				for _, nIdx := range neighbors {
					if nIdx < len(planet.Vertices) {
						sum += planet.Vertices[nIdx].Height
						count += 1.0
					}
				}
			}
			
			newHeights[i] = sum / count
		}
		
		// Apply smoothed heights
		for i := range planet.Vertices {
			planet.Vertices[i].Height = newHeights[i]
		}
	}
	
	return planet
}

// clampHeightChanges prevents single-frame spikes
func clampHeightChanges(planet Planet, oldPlanet Planet, maxChange float64) Planet {
	if len(oldPlanet.Vertices) != len(planet.Vertices) {
		return planet
	}
	
	for i := range planet.Vertices {
		oldHeight := oldPlanet.Vertices[i].Height
		newHeight := planet.Vertices[i].Height
		change := newHeight - oldHeight
		
		// Clamp the change
		if math.Abs(change) > maxChange {
			if change > 0 {
				planet.Vertices[i].Height = oldHeight + maxChange
			} else {
				planet.Vertices[i].Height = oldHeight - maxChange
			}
		}
		
		// Enforce more generous absolute limits for emergent features
		if planet.Vertices[i].Height > 0.15 { // Allow taller mountains
			planet.Vertices[i].Height = 0.15
		} else if planet.Vertices[i].Height < -0.10 { // Allow deeper ocean trenches
			planet.Vertices[i].Height = -0.10
		}
	}
	
	return planet
}

// buildNeighborCache creates vertex adjacency information
func buildNeighborCache(planet Planet) Planet {
	if planet.NeighborCache == nil {
		planet.NeighborCache = make(map[int][]int)
	}
	
	// Initialize neighbor lists for all vertices
	for i := range planet.Vertices {
		planet.NeighborCache[i] = []int{}
	}
	
	// Build neighbor lists directly from triangles
	// Use a set to avoid duplicates
	neighborSets := make([]map[int]bool, len(planet.Vertices))
	for i := range neighborSets {
		neighborSets[i] = make(map[int]bool)
	}
	
	// Process each triangle
	for i := 0; i < len(planet.Indices); i += 3 {
		v0 := int(planet.Indices[i])
		v1 := int(planet.Indices[i+1])
		v2 := int(planet.Indices[i+2])
		
		// Each vertex in a triangle is a neighbor of the other two
		neighborSets[v0][v1] = true
		neighborSets[v0][v2] = true
		neighborSets[v1][v0] = true
		neighborSets[v1][v2] = true
		neighborSets[v2][v0] = true
		neighborSets[v2][v1] = true
	}
	
	// Convert sets to slices
	for i, neighbors := range neighborSets {
		neighborList := make([]int, 0, len(neighbors))
		for n := range neighbors {
			neighborList = append(neighborList, n)
		}
		planet.NeighborCache[i] = neighborList
	}
	
	return planet
}