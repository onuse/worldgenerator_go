package core

import (
	"fmt"
	"math"
	"math/rand"
)

// CreateVoxelPlanet initializes a new voxel-based planet
func CreateVoxelPlanet(radius float64, shellCount int) *VoxelPlanet {
	planet := &VoxelPlanet{
		Radius:      radius,
		Mass:        5.972e24, // Earth mass in kg
		Time:        0,
		RotationVel: 7.2921e-5, // Earth rotation rate rad/s
		ActiveCells: make(map[VoxelCoord]bool),
		MeshDirty:   true,
	}
	
	// Create shells from core to surface
	// Use exponential spacing to have more detail near surface
	planet.Shells = make([]SphericalShell, shellCount)
	
	coreRadius := radius * 0.2  // Inner core at 20% of radius
	
	for i := 0; i < shellCount; i++ {
		// Exponential spacing for more surface detail
		t := float64(i) / float64(shellCount-1)
		inner := coreRadius + (radius-coreRadius)*math.Pow(t, 2.0)
		
		var outer float64
		if i < shellCount-1 {
			tNext := float64(i+1) / float64(shellCount-1)
			outer = coreRadius + (radius-coreRadius)*math.Pow(tNext, 2.0)
		} else {
			outer = radius * 1.01 // Thin atmosphere layer
		}
		
		// More latitude bands for outer shells
		// Use exponential growth for better surface resolution
		latBands := 20 + i*i*2 // Much higher resolution at surface
		if i >= shellCount-2 {
			// Maximum resolution for surface and atmosphere
			latBands = 360 // 0.5 degree resolution
		}
		if latBands > 360 {
			latBands = 360
		}
		
		planet.Shells[i] = createSphericalShell(inner, outer, latBands, i, shellCount)
	}
	
	// Initialize material composition
	initializePlanetComposition(planet)
	
	// Initialize water conservation tracking
	planet.SeaLevel = 0 // Start at 0m elevation
	planet.TotalWaterVolume = planet.CalculateTotalWaterVolume()
	
	fmt.Printf("Created voxel planet: radius=%.0fm, shells=%d\n", radius, shellCount)
	fmt.Printf("Initial water volume: %.2e m³\n", planet.TotalWaterVolume)
	for i, shell := range planet.Shells {
		totalVoxels := 0
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
		fmt.Printf("  Shell %d: r=%.0f-%.0f km, %dx? grid, %d voxels\n", 
			i, shell.InnerRadius/1000, shell.OuterRadius/1000, 
			shell.LatBands, totalVoxels)
	}
	
	return planet
}

// createSphericalShell creates a single shell with appropriate voxel grid
func createSphericalShell(inner, outer float64, latBands int, shellIndex, totalShells int) SphericalShell {
	shell := SphericalShell{
		InnerRadius: inner,
		OuterRadius: outer,
		LatBands:    latBands,
		Voxels:      make([][]VoxelMaterial, latBands),
		LonCounts:   make([]int, latBands),
	}
	
	// Create voxels for each latitude band
	for lat := 0; lat < latBands; lat++ {
		lonCount := GetLonCount(latBands, lat)
		shell.LonCounts[lat] = lonCount
		shell.Voxels[lat] = make([]VoxelMaterial, lonCount)
		
		// Initialize empty voxels
		for lon := 0; lon < lonCount; lon++ {
			shell.Voxels[lat][lon] = VoxelMaterial{
				Type:          MatAir,
				Density:       MaterialProperties[MatAir].DefaultDensity,
				Temperature:   288.15, // 15°C default
				Pressure:      101325, // 1 atm
				StretchFactor: 1.0,   // Normal, not stretched
				PlateID:       0,     // Not part of any plate initially
			}
		}
	}
	
	return shell
}

// initializePlanetComposition sets up initial material distribution
func initializePlanetComposition(planet *VoxelPlanet) {
	earthRadius := planet.Radius
	
	for shellIdx, shell := range planet.Shells {
		avgRadius := (shell.InnerRadius + shell.OuterRadius) / 2
		
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				// Determine material based on depth
				if avgRadius < earthRadius*0.55 {
					// Inner and outer core - simplified as hot peridotite
					voxel.Type = MatPeridotite
					voxel.Density = 11000 // Very dense core
					voxel.Temperature = 5000 + float32(1000*(1-avgRadius/(earthRadius*0.55)))
				} else if avgRadius < earthRadius*0.85 {
					// Mantle
					voxel.Type = MatPeridotite
					voxel.Density = MaterialProperties[MatPeridotite].DefaultDensity
					// Temperature gradient from core to surface
					t := (avgRadius - earthRadius*0.55) / (earthRadius * 0.3)
					voxel.Temperature = 4000 - float32(3000*t) // 4000K to 1000K
				} else if shellIdx < len(planet.Shells)-2 && avgRadius < earthRadius*0.99 {
					// Crust - create realistic continental distribution (but not the surface shell)
					lat := getLatitudeForBand(latIdx, shell.LatBands)
					lon := float64(lonIdx) / float64(shell.LonCounts[latIdx]) * 360.0 - 180.0
					
					// Create several continental masses using multi-scale noise
					// Europe/Africa/Asia
					europe := 0.0
					if lat > 35 && lat < 70 && lon > -10 && lon < 40 {
						europe = 1.0 - 0.3*math.Abs(lat-50)/20 - 0.3*math.Abs(lon-15)/25
						// Add Mediterranean
						if lat > 35 && lat < 45 && lon > -5 && lon < 35 {
							europe *= 0.7
						}
					}
					
					// Africa
					africa := 0.0
					if lat > -35 && lat < 35 && lon > -20 && lon < 50 {
						africa = 0.8 - 0.2*math.Abs(lat)/35 - 0.2*math.Abs(lon-15)/35
					}
					
					// Americas
					americas := 0.0
					if lon > -170 && lon < -30 {
						americas = 0.7*math.Sin((lat+20)*0.02) * (1-math.Abs(lon+100)/70)
					}
					
					// Asia
					asia := 0.0
					if lat > 0 && lat < 80 && lon > 40 && lon < 180 {
						asia = 0.8 - 0.3*math.Abs(lat-40)/40 - 0.2*math.Abs(lon-90)/90
					}
					
					// Australia
					australia := 0.0
					if lat > -45 && lat < -10 && lon > 110 && lon < 155 {
						australia = 0.8 - 0.3*math.Abs(lat+25)/20 - 0.3*math.Abs(lon-135)/25
					}
					
					// Combine continents
					continentalness := math.Max(europe, math.Max(africa, math.Max(americas, math.Max(asia, australia))))
					
					// Add smaller scale features
					continentalness += 0.1 * math.Sin(lat*0.2) * math.Cos(lon*0.2)
					continentalness += 0.05 * math.Sin(lat*0.4) * math.Cos(lon*0.4)
					
					if continentalness > 0.3 {
						// Continental crust
						voxel.Type = MatGranite
						voxel.Density = MaterialProperties[MatGranite].DefaultDensity
						// Create longitude-based age bands to visualize movement
						ageBand := float64(lonIdx) / float64(shell.LonCounts[latIdx]) * 10.0
						voxel.Age = float32(100000000 * (1 + math.Sin(ageBand))) // 0-200My in bands
					} else {
						// Oceanic crust
						voxel.Type = MatBasalt
						voxel.Density = MaterialProperties[MatBasalt].DefaultDensity
						// Create different age pattern for oceans
						ageBand := float64(lonIdx) / float64(shell.LonCounts[latIdx]) * 8.0
						voxel.Age = float32(50000000 * (1 + math.Cos(ageBand))) // 0-100My in bands
					}
					voxel.Temperature = 1000 - float32(700*(avgRadius-earthRadius*0.85)/(earthRadius*0.14))
					
					// Mark crustal rocks as brittle for plate tectonics
					voxel.IsBrittle = true
					
					// Add initial plate velocities (simple eastward drift)
					// This gives plates something to work with initially
					voxel.VelPhi = 1e-9 * float32(1 + 0.5*math.Sin(lat*0.1)) // ~3 cm/year at equator
				} else if shellIdx == len(planet.Shells)-2 {
					// Surface shell - recalculate continentalness here
					lat := getLatitudeForBand(latIdx, shell.LatBands)
					lon := float64(lonIdx) / float64(shell.LonCounts[latIdx]) * 360.0 - 180.0
					
					// Simple continent generation - just basic shapes, no noise
					isLand := false
					
					// Simple rectangular continents for testing
					// Eurasia
					if lat > 20 && lat < 75 && lon > -10 && lon < 140 {
						isLand = true
					}
					// Africa  
					if lat > -35 && lat < 35 && lon > -20 && lon < 50 {
						isLand = true
					}
					// Americas
					if lon > -170 && lon < -30 {
						if lat > -55 && lat < 70 {
							isLand = true
						}
					}
					// Australia
					if lat > -40 && lat < -10 && lon > 110 && lon < 155 {
						isLand = true
					}
					
					if isLand {
						voxel.Type = MatGranite
						voxel.Density = MaterialProperties[MatGranite].DefaultDensity
						voxel.IsBrittle = true
						voxel.VelPhi = 1e-9 * float32(1 + 0.5*math.Sin(lat*0.1))
						
						// Add initial elevation variation based on simple patterns
						// Use lat/lon to create mountain ranges
						mountainNoise := math.Sin(lat*0.1) * math.Cos(lon*0.05)
						baseElevation := float32(500) // Base continental elevation
						
						// Add mountain ranges
						if mountainNoise > 0.7 {
							voxel.Elevation = baseElevation + float32(1000 + rand.Float64()*2000) // 1.5-3.5km mountains
						} else if mountainNoise > 0.3 {
							voxel.Elevation = baseElevation + float32(200 + rand.Float64()*800) // 0.7-1.3km highlands
						} else {
							voxel.Elevation = baseElevation + float32(rand.Float64()*200 - 100) // 0.4-0.6km lowlands
						}
					} else {
						voxel.Type = MatWater
						voxel.Density = MaterialProperties[MatWater].DefaultDensity
						// Ocean depth based on distance from continents
						oceanDepth := float32(-1000 - rand.Float64()*3000) // -1 to -4km
						voxel.Elevation = oceanDepth
					}
					voxel.Temperature = 288.15 - float32(math.Abs(lat)*0.5)
				} else {
					// Atmosphere
					lat := getLatitudeForBand(latIdx, shell.LatBands)
					voxel.Type = MatAir
					voxel.Density = MaterialProperties[MatAir].DefaultDensity * 
						float32(math.Exp(-(avgRadius-earthRadius)/8000)) // Exponential atmosphere
					voxel.Temperature = 288.15 - float32(math.Abs(lat)*0.7)
				}
				
				// Set pressure based on overlying material (simplified)
				if shellIdx > 0 {
					depthFromSurface := earthRadius - avgRadius
					voxel.Pressure = 101325 + float32(depthFromSurface*3000) // ~3000 Pa/m
				}
			}
		}
	}
}

// getLatitudeForBand converts a latitude band index to degrees
func getLatitudeForBand(latIdx int, latBands int) float64 {
	// Map from 0..latBands-1 to -90..+90 degrees
	return (float64(latIdx)+0.5)/float64(latBands)*180.0 - 90.0
}

// GetVoxel retrieves a voxel at the specified coordinates
func (p *VoxelPlanet) GetVoxel(coord VoxelCoord) *VoxelMaterial {
	if coord.Shell < 0 || coord.Shell >= len(p.Shells) {
		return nil
	}
	
	shell := &p.Shells[coord.Shell]
	if coord.Lat < 0 || coord.Lat >= shell.LatBands {
		return nil
	}
	
	// Handle longitude wrapping
	lonCount := shell.LonCounts[coord.Lat]
	coord.Lon = ((coord.Lon % lonCount) + lonCount) % lonCount
	
	return &shell.Voxels[coord.Lat][coord.Lon]
}

// GetSurfaceVoxel finds the topmost non-air voxel at a given lat/lon
func (p *VoxelPlanet) GetSurfaceVoxel(lat, lon int) (*VoxelMaterial, int) {
	// Search from top shell downward
	for shellIdx := len(p.Shells) - 1; shellIdx >= 0; shellIdx-- {
		shell := &p.Shells[shellIdx]
		
		// Map lat/lon to this shell's resolution
		shellLat := lat * shell.LatBands / 180 // Assuming input is in degrees
		if shellLat >= shell.LatBands {
			shellLat = shell.LatBands - 1
		}
		
		lonCount := shell.LonCounts[shellLat]
		shellLon := lon * lonCount / 360
		if shellLon >= lonCount {
			shellLon = lonCount - 1
		}
		
		voxel := &shell.Voxels[shellLat][shellLon]
		
		// Found non-air voxel
		if voxel.Type != MatAir {
			return voxel, shellIdx
		}
	}
	
	return nil, -1
}

// MarkCellActive marks a voxel as needing update in the next simulation step
func (p *VoxelPlanet) MarkCellActive(coord VoxelCoord) {
	p.ActiveCells[coord] = true
}

// ClearActiveCells resets the active cell list
func (p *VoxelPlanet) ClearActiveCells() {
	p.ActiveCells = make(map[VoxelCoord]bool)
}

// CalculateTotalWaterVolume computes the total water volume on the planet
func (p *VoxelPlanet) CalculateTotalWaterVolume() float64 {
	totalVolume := 0.0
	
	// Only check surface shell for water
	if len(p.Shells) < 2 {
		return 0
	}
	
	surfaceShell := len(p.Shells) - 2
	shell := &p.Shells[surfaceShell]
	
	for latIdx := range shell.Voxels {
		lat := getLatitudeForBand(latIdx, shell.LatBands)
		latRad := lat * math.Pi / 180.0
		
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			if voxel.Type == MatWater {
				// Calculate voxel volume considering spherical geometry
				// Volume = r²Δr × ΔθΔφ × sin(θ)
				r := (shell.InnerRadius + shell.OuterRadius) / 2
				dr := shell.OuterRadius - shell.InnerRadius
				dTheta := math.Pi / float64(shell.LatBands)
				dPhi := 2 * math.Pi / float64(shell.LonCounts[latIdx])
				
				// Volume in m³
				volume := r * r * dr * dTheta * dPhi * math.Abs(math.Cos(latRad))
				
				// Consider water depth (elevation below sea level)
				if voxel.Elevation < 0 {
					// Water column height = sea level - ocean floor
					waterDepth := p.SeaLevel - float64(voxel.Elevation)
					// Adjust volume based on actual water depth vs shell thickness
					depthRatio := waterDepth / dr
					if depthRatio > 1 {
						depthRatio = 1 // Cap at shell thickness
					}
					volume *= depthRatio
				}
				
				totalVolume += volume
			}
		}
	}
	
	return totalVolume
}

// UpdateSeaLevel recalculates sea level to maintain constant water volume
func (p *VoxelPlanet) UpdateSeaLevel() {
	if p.TotalWaterVolume <= 0 {
		// Initialize on first call
		p.TotalWaterVolume = p.CalculateTotalWaterVolume()
		p.SeaLevel = 0 // Initial sea level at 0m
		return
	}
	
	// Binary search for the sea level that gives us the target water volume
	minLevel := -5000.0 // Deepest ocean
	maxLevel := 1000.0   // Potential high sea level
	tolerance := 1.0     // 1 meter tolerance
	
	for maxLevel-minLevel > tolerance {
		testLevel := (minLevel + maxLevel) / 2
		p.SeaLevel = testLevel
		
		// Calculate volume at this sea level
		currentVolume := p.CalculateWaterVolumeAtSeaLevel(testLevel)
		
		if currentVolume < p.TotalWaterVolume {
			// Need higher sea level
			minLevel = testLevel
		} else {
			// Need lower sea level
			maxLevel = testLevel
		}
	}
	
	p.SeaLevel = (minLevel + maxLevel) / 2
}

// CalculateWaterVolumeAtSeaLevel calculates water volume if sea level was at given elevation
func (p *VoxelPlanet) CalculateWaterVolumeAtSeaLevel(seaLevel float64) float64 {
	totalVolume := 0.0
	
	if len(p.Shells) < 2 {
		return 0
	}
	
	surfaceShell := len(p.Shells) - 2
	shell := &p.Shells[surfaceShell]
	
	for latIdx := range shell.Voxels {
		lat := getLatitudeForBand(latIdx, shell.LatBands)
		latRad := lat * math.Pi / 180.0
		
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Any voxel below sea level contributes to water volume
			if float64(voxel.Elevation) < seaLevel {
				r := (shell.InnerRadius + shell.OuterRadius) / 2
				dr := shell.OuterRadius - shell.InnerRadius
				dTheta := math.Pi / float64(shell.LatBands)
				dPhi := 2 * math.Pi / float64(shell.LonCounts[latIdx])
				
				// Base voxel volume
				volume := r * r * dr * dTheta * dPhi * math.Abs(math.Cos(latRad))
				
				// Water depth at this location
				waterDepth := seaLevel - float64(voxel.Elevation)
				depthRatio := waterDepth / dr
				if depthRatio > 1 {
					depthRatio = 1
				}
				volume *= depthRatio
				
				totalVolume += volume
			}
		}
	}
	
	return totalVolume
}