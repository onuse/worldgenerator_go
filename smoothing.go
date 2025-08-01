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
		
		// Also enforce absolute limits
		if planet.Vertices[i].Height > 0.08 {
			planet.Vertices[i].Height = 0.08
		} else if planet.Vertices[i].Height < -0.04 {
			planet.Vertices[i].Height = -0.04
		}
	}
	
	return planet
}

// buildNeighborCache creates vertex adjacency information
func buildNeighborCache(planet Planet) Planet {
	if planet.NeighborCache == nil {
		planet.NeighborCache = make(map[int][]int)
	}
	
	// Build edge map from triangles
	edgeMap := make(map[[2]int]bool)
	
	for i := 0; i < len(planet.Indices); i += 3 {
		v0 := int(planet.Indices[i])
		v1 := int(planet.Indices[i+1])
		v2 := int(planet.Indices[i+2])
		
		// Add edges (always store smaller index first)
		addEdge := func(a, b int) {
			if a > b {
				a, b = b, a
			}
			edgeMap[[2]int{a, b}] = true
		}
		
		addEdge(v0, v1)
		addEdge(v1, v2)
		addEdge(v2, v0)
	}
	
	// Convert edge map to neighbor lists
	for i := range planet.Vertices {
		neighbors := []int{}
		
		// Check all possible edges
		for edge := range edgeMap {
			if edge[0] == i {
				neighbors = append(neighbors, edge[1])
			} else if edge[1] == i {
				neighbors = append(neighbors, edge[0])
			}
		}
		
		planet.NeighborCache[i] = neighbors
	}
	
	return planet
}