package core

import (
	"math"
	"math/rand"
)

// PlanetGenerationParams controls random planet generation
type PlanetGenerationParams struct {
	Seed               int64
	ContinentCount     int
	OceanFraction      float64
	MinContinentSize   float64 // Minimum size as fraction of surface
	MaxContinentSize   float64 // Maximum size as fraction of surface
	ContinentRoughness float64 // How irregular continent shapes are (0=smooth, 1=very rough)
}

// CreateRandomizedPlanet creates a planet with randomly placed continents
func CreateRandomizedPlanet(radius float64, shellCount int, params PlanetGenerationParams) *VoxelPlanet {
	// Initialize random generator with seed
	rng := rand.New(rand.NewSource(params.Seed))

	// Create base planet structure
	planet := CreateVoxelPlanet(radius, shellCount)

	// Generate random continents on the surface
	generateRandomContinents(planet, rng, params)

	// Add initial plate velocities with random patterns
	addRandomPlateVelocities(planet, rng)

	return planet
}

// generateRandomContinents creates randomly positioned continental masses
func generateRandomContinents(planet *VoxelPlanet, rng *rand.Rand, params PlanetGenerationParams) {
	if len(planet.Shells) < 2 {
		return
	}

	// Work with the surface shell (second from top)
	surfaceShell := len(planet.Shells) - 2
	shell := &planet.Shells[surfaceShell]

	// Calculate total surface area
	totalVoxels := 0
	for _, latBand := range shell.Voxels {
		totalVoxels += len(latBand)
	}

	// Create continent seeds
	type continentSeed struct {
		lat    float64
		lon    float64
		radius float64 // Angular radius in degrees
		shape  float64 // Shape factor (0=circular, 1=very irregular)
	}

	seeds := make([]continentSeed, params.ContinentCount)

	// Generate random continent positions and sizes
	for i := 0; i < params.ContinentCount; i++ {
		// Random position
		lat := (rng.Float64() - 0.5) * 180.0 // -90 to 90
		lon := (rng.Float64() - 0.5) * 360.0 // -180 to 180

		// Random size (angular radius)
		sizeRange := params.MaxContinentSize - params.MinContinentSize
		sizeFraction := params.MinContinentSize + rng.Float64()*sizeRange

		// Convert size fraction to angular radius
		// Approximate: total surface area = 4πr², continent area = πR²
		// So R = sqrt(sizeFraction * 4) * 180/π degrees
		angularRadius := math.Sqrt(sizeFraction*4) * 180.0 / math.Pi

		// Random shape factor
		shape := rng.Float64() * params.ContinentRoughness

		seeds[i] = continentSeed{
			lat:    lat,
			lon:    lon,
			radius: angularRadius,
			shape:  shape,
		}
	}

	// First, clear all existing surface material to ensure clean generation
	for latIdx, latBand := range shell.Voxels {
		for lonIdx := range latBand {
			voxel := &shell.Voxels[latIdx][lonIdx]
			// Reset to default ocean state
			voxel.Type = MatWater
			voxel.Density = MaterialProperties[MatWater].DefaultDensity
			voxel.Temperature = 288.15 // Default ocean temperature
			voxel.Elevation = -1000    // Default ocean depth
			voxel.Age = 0
			voxel.PlateID = 0
			voxel.IsBrittle = false
			// Clear any velocities from previous initialization
			voxel.VelNorth = 0
			voxel.VelEast = 0
			voxel.VelR = 0
		}
	}

	// Fill in the voxels based on continent seeds
	for latIdx, latBand := range shell.Voxels {
		lat := GetLatitudeForBand(latIdx, shell.LatBands)

		for lonIdx := range latBand {
			voxel := &shell.Voxels[latIdx][lonIdx]
			lon := float64(lonIdx)/float64(len(latBand))*360.0 - 180.0

			// Check if this voxel is within any continent
			isLand := false
			minDistance := math.MaxFloat64

			for _, seed := range seeds {
				// Calculate angular distance to continent center
				distance := angularDistance(lat, lon, seed.lat, seed.lon)

				// Add noise for irregular shapes
				noiseFactor := 1.0
				if seed.shape > 0 {
					// Use position-based noise for consistent continent shapes
					noise1 := math.Sin(lat*0.1+lon*0.1) * math.Cos(lat*0.2-lon*0.15)
					noise2 := math.Sin(lat*0.3-lon*0.2) * math.Cos(lat*0.15+lon*0.25)
					noiseFactor = 1.0 + seed.shape*(noise1*0.3+noise2*0.2)
				}

				effectiveRadius := seed.radius * noiseFactor

				if distance < effectiveRadius {
					isLand = true
					if distance < minDistance {
						minDistance = distance
					}
				}
			}

			// Set voxel properties
			if isLand {
				voxel.Type = MatGranite
				voxel.Density = MaterialProperties[MatGranite].DefaultDensity
				voxel.IsBrittle = true

				// Age based on distance from continent center (older at center)
				ageFactor := 1.0 - minDistance/30.0
				if ageFactor < 0 {
					ageFactor = 0
				}
				voxel.Age = float32(50000000 + 150000000*ageFactor) // 50-200 My

				// Elevation based on distance from edge
				// Find distance to nearest continent edge
				edgeDistance := math.MaxFloat64
				for _, seed := range seeds {
					dist := angularDistance(lat, lon, seed.lat, seed.lon)
					edgeDist := seed.radius - dist
					if edgeDist < edgeDistance && edgeDist > 0 {
						edgeDistance = edgeDist
					}
				}
				
				// Create elevation gradient from coast to interior
				if edgeDistance < 5.0 { // Within 5 degrees of coast
					// Coastal lowlands with smooth transition
					coastFactor := edgeDistance / 5.0
					voxel.Elevation = float32(coastFactor * 500.0) // 0-500m coastal plain
				} else {
					// Interior with varied elevation
					voxel.Elevation = 500 + float32(rng.Float64()*1000) // 500-1500m interior
				}
			} else {
				voxel.Type = MatWater
				voxel.Density = MaterialProperties[MatWater].DefaultDensity
				// Ocean depth gradient based on distance from land
				oceanDepth := float32(-1000 - rng.Float64()*3000) // -1 to -4km
				voxel.Elevation = oceanDepth
			}

			// Set temperature
			voxel.Temperature = 288.15 - float32(math.Abs(lat)*0.5)
		}
	}

	// Also update the crust layer below
	if surfaceShell > 0 {
		crustShell := &planet.Shells[surfaceShell-1]
		for latIdx, latBand := range crustShell.Voxels {
			for lonIdx := range latBand {
				voxel := &crustShell.Voxels[latIdx][lonIdx]

				// Check what's above
				if latIdx < len(shell.Voxels) && lonIdx < len(shell.Voxels[latIdx]) {
					surfaceVoxel := &shell.Voxels[latIdx][lonIdx]
					if surfaceVoxel.Type == MatGranite {
						// Continental crust
						voxel.Type = MatGranite
						voxel.Density = MaterialProperties[MatGranite].DefaultDensity
						voxel.IsBrittle = true
						voxel.Age = surfaceVoxel.Age
					} else {
						// Oceanic crust
						voxel.Type = MatBasalt
						voxel.Density = MaterialProperties[MatBasalt].DefaultDensity
						voxel.IsBrittle = true
						voxel.Age = float32(rng.Float64() * 100000000) // 0-100 My
					}
				}

				voxel.Temperature = 1000 - float32(700*(crustShell.OuterRadius-planet.Radius*0.85)/(planet.Radius*0.14))
			}
		}
	}
}

// addRandomPlateVelocities adds initial velocities to create plate motion
func addRandomPlateVelocities(planet *VoxelPlanet, rng *rand.Rand) {
	if len(planet.Shells) < 2 {
		return
	}

	// Create several velocity "cells" that will become plates
	numCells := 8 + rng.Intn(5) // 8-12 velocity cells

	type velocityCell struct {
		lat       float64
		lon       float64
		VelNorth  float32 // North-south velocity
		VelEast   float32 // East-west velocity
		influence float64 // How far this cell's influence extends
	}

	cells := make([]velocityCell, numCells)

	// Generate random velocity cells
	for i := 0; i < numCells; i++ {
		// Random position
		lat := (rng.Float64() - 0.5) * 180.0
		lon := (rng.Float64() - 0.5) * 360.0

		// Random velocity (in m/s)
		// Typical plate velocities are 2-10 cm/year = 6e-10 to 3e-9 m/s
		// Start with realistic speeds - fractional system will handle smooth movement
		speed := float32(rng.Float64()*2e-9 + 1e-9) // 3-10 cm/year realistic
		angle := rng.Float64() * 2 * math.Pi

		cells[i] = velocityCell{
			lat:       lat,
			lon:       lon,
			VelNorth:  speed * float32(math.Sin(angle)),
			VelEast:   speed * float32(math.Cos(angle)),
			influence: 20.0 + rng.Float64()*20.0, // 20-40 degree influence radius
		}
	}

	// Apply velocities to crustal shells
	for shellIdx := len(planet.Shells) - 3; shellIdx < len(planet.Shells)-1; shellIdx++ {
		if shellIdx < 0 {
			continue
		}

		shell := &planet.Shells[shellIdx]

		for latIdx, latBand := range shell.Voxels {
			lat := GetLatitudeForBand(latIdx, shell.LatBands)

			for lonIdx := range latBand {
				voxel := &shell.Voxels[latIdx][lonIdx]

				// Only apply to solid crustal material
				if voxel.Type != MatGranite && voxel.Type != MatBasalt {
					continue
				}

				lon := float64(lonIdx)/float64(len(latBand))*360.0 - 180.0

				// Find the nearest velocity cell
				var nearestCell *velocityCell
				minDistance := math.MaxFloat64

				for i := range cells {
					distance := angularDistance(lat, lon, cells[i].lat, cells[i].lon)
					if distance < cells[i].influence && distance < minDistance {
						minDistance = distance
						nearestCell = &cells[i]
					}
				}

				if nearestCell != nil {
					// Apply velocity with some attenuation based on distance
					attenuation := float32(1.0 - minDistance/nearestCell.influence)
					voxel.VelNorth = nearestCell.VelNorth * attenuation
					voxel.VelEast = nearestCell.VelEast * attenuation
				}
			}
		}
	}
}

// angularDistance calculates the angular distance between two points on a sphere
func angularDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180.0
	lon1Rad := lon1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	lon2Rad := lon2 * math.Pi / 180.0

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// Convert back to degrees
	return c * 180.0 / math.Pi
}
