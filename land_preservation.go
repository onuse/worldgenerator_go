package main

// preserveLandmass ensures a minimum amount of land remains above sea level
func preserveLandmass(planet Planet, minLandFraction float64) Planet {
	// Count current land vertices
	landCount := 0
	for _, v := range planet.Vertices {
		if v.Height > 0.0 {
			landCount++
		}
	}
	
	currentLandFraction := float64(landCount) / float64(len(planet.Vertices))
	
	// If we have too little land, make a small adjustment
	if currentLandFraction < minLandFraction {
		deficit := minLandFraction - currentLandFraction
		uplift := deficit * 0.001 // Much smaller adjustment
		
		for i := range planet.Vertices {
			v := &planet.Vertices[i]
			if v.PlateID >= 0 && v.PlateID < len(planet.Plates) {
				if planet.Plates[v.PlateID].Type == Continental {
					// Only boost areas very close to sea level
					if v.Height > -0.002 && v.Height < 0.002 {
						v.Height += uplift
						if v.Height > 0.002 {
							v.Height = 0.002 // Just barely above sea level
						}
					}
				}
			}
		}
	}
	
	return planet
}