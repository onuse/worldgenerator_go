package main

import (
	"fmt"
	"math"
)



func generateRealisticContinents(planet Planet) Planet {
	// Generate terrain using multi-scale fractal noise
	planet = applyFractalTerrain(planet)
	
	// Add tectonic features like ridges and trenches
	planet = addTectonicFeatures(planet)
	
	// Apply isostatic adjustment
	planet = applyIsostasy(planet)
	
	// Don't smooth initial terrain - preserve the fractal detail
	// planet = smoothHeights(planet, 2)
	
	return planet
}



func applyFractalTerrain(planet Planet) Planet {
	progressInterval := len(planet.Vertices) / 20
	if progressInterval == 0 {
		progressInterval = 1
	}
	
	for i := range planet.Vertices {
		if i%progressInterval == 0 {
			progress := float64(i) / float64(len(planet.Vertices)) * 100
			fmt.Printf("\r    Applying fractal terrain: %3.0f%%", progress)
		}
		
		pos := planet.Vertices[i].Position // Should already be normalized
		
		// Continental-scale features (very low frequency)
		continental := terrainNoise(pos.X*1.5, pos.Y*1.5, pos.Z*1.5)
		
		// Regional features (medium frequency)
		regional := terrainNoise(pos.X*4, pos.Y*4, pos.Z*4) * 0.5
		
		// Local features (high frequency)
		local := terrainNoise(pos.X*10, pos.Y*10, pos.Z*10) * 0.2
		
		// Fine detail
		detail := terrainNoise(pos.X*25, pos.Y*25, pos.Z*25) * 0.05
		
		// Ridge noise for mountain chains
		ridges := ridgeNoise(pos.X*3, pos.Y*3, pos.Z*3) * 0.3
		
		// Combine all scales
		totalNoise := continental + regional + local + detail
		
		// Add ridges only in high areas
		if totalNoise > 0.2 {
			totalNoise += ridges * (totalNoise - 0.2)
		}
		
		// Convert noise to realistic elevations
		// Using Earth-like hypsography
		if totalNoise > 0.4 {
			// High mountains (rare)
			planet.Vertices[i].Height = 0.02 + (totalNoise-0.4)*0.04
		} else if totalNoise > 0.15 {
			// Hills and plateaus
			t := (totalNoise - 0.15) / 0.25
			planet.Vertices[i].Height = 0.002 + t*0.018
		} else if totalNoise > 0.0 {
			// Coastal plains and lowlands
			t := totalNoise / 0.15
			planet.Vertices[i].Height = -0.001 + t*0.003
		} else if totalNoise > -0.3 {
			// Continental shelf
			t := (totalNoise + 0.3) / 0.3
			planet.Vertices[i].Height = -0.005 + t*0.004
		} else if totalNoise > -0.6 {
			// Ocean slopes
			t := (totalNoise + 0.6) / 0.3
			planet.Vertices[i].Height = -0.02 + t*0.015
		} else {
			// Deep ocean basins
			planet.Vertices[i].Height = -0.025 + (totalNoise+0.6)*0.01
		}
		
		// Keep position normalized - height is stored separately
		// Don't modify the position - it should remain on the unit sphere
	}
	
	fmt.Printf("\r    Applying fractal terrain: 100%%\n")
	return planet
}

func addTectonicFeatures(planet Planet) Planet {
	// Add some predefined tectonic features for realism
	// These will later be modified by plate tectonics
	
	// Add mid-ocean ridges (elevated seafloor)
	for i := range planet.Vertices {
		pos := planet.Vertices[i].Position.Normalize()
		
		// Only affect ocean floor
		if planet.Vertices[i].Height < -0.005 && planet.Vertices[i].Height > -0.02 {
			// Create ridge-like features
			ridge := ridgeNoise(pos.X*2.5, pos.Y*2.5, pos.Z*2.5)
			if ridge > 0.7 {
				// Raise seafloor at ridges
				planet.Vertices[i].Height += (ridge - 0.7) * 0.008
			}
		}
		
		// Add deep ocean trenches
		if planet.Vertices[i].Height < -0.015 {
			trench := math.Abs(terrainNoise(pos.X*4, pos.Y*4, pos.Z*4))
			if trench < 0.1 {
				// Deepen trenches
				planet.Vertices[i].Height -= (0.1 - trench) * 0.01
			}
		}
	}
	
	return planet
}

func applyIsostasy(planet Planet) Planet {
	// Simple isostatic adjustment - higher mountains sink deeper roots
	// This creates more realistic elevation distributions
	
	for i := range planet.Vertices {
		if planet.Vertices[i].Height > 0.01 {
			// Mountains are supported by deep roots
			// Reduce extreme heights slightly
			planet.Vertices[i].Height *= 0.85
		} else if planet.Vertices[i].Height > 0.005 {
			// Hills - minor adjustment
			planet.Vertices[i].Height *= 0.95
		}
		
		// Position should remain on unit sphere - height is applied during rendering
	}
	
	return planet
}

func getHeights(planet Planet) []float64 {
	heights := make([]float64, len(planet.Vertices))
	for i, v := range planet.Vertices {
		heights[i] = v.Height
	}
	return heights
}