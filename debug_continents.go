package main

import (
	"fmt"
	"math"
)

// DebugContinentalness prints continental values for debugging
func DebugContinentalness() {
	// Test a few key locations
	locations := []struct {
		name string
		lat, lon float64
	}{
		{"Europe Center", 50.0, 10.0},
		{"Europe Edge", 50.0, 35.0},
		{"Africa Center", 0.0, 20.0},
		{"Ocean Atlantic", 30.0, -40.0},
		{"Americas Center", 20.0, -100.0},
		{"Asia Center", 40.0, 90.0},
		{"Australia Center", -25.0, 135.0},
		{"Pacific Ocean", 0.0, -170.0},
	}
	
	for _, loc := range locations {
		lat := loc.lat
		lon := loc.lon
		
		// Calculate using the same logic as in voxel_planet.go
		europe := 0.0
		if lat > 35 && lat < 70 && lon > -10 && lon < 40 {
			latDist := math.Abs(lat-52.5) / 17.5
			lonDist := math.Abs(lon-15) / 25
			europe = math.Max(0, 1.0 - math.Sqrt(latDist*latDist + lonDist*lonDist))
		}
		
		africa := 0.0
		if lat > -35 && lat < 35 && lon > -20 && lon < 50 {
			latDist := math.Abs(lat) / 35
			lonDist := math.Abs(lon-15) / 35
			africa = math.Max(0, 1.0 - 0.8*math.Sqrt(latDist*latDist + lonDist*lonDist))
		}
		
		americas := 0.0
		if lon > -170 && lon < -30 {
			lonDist := math.Abs(lon+100) / 70
			americas = math.Max(0, 0.9 - lonDist) * (0.8 + 0.2*math.Sin((lat+20)*0.02))
		}
		
		asia := 0.0
		if lat > 0 && lat < 80 && lon > 40 && lon < 180 {
			latDist := math.Abs(lat-40) / 40
			lonDist := math.Abs(lon-110) / 70
			asia = math.Max(0, 1.0 - 0.7*math.Sqrt(latDist*latDist + lonDist*lonDist))
		}
		
		australia := 0.0
		if lat > -45 && lat < -10 && lon > 110 && lon < 155 {
			latDist := math.Abs(lat+27.5) / 17.5
			lonDist := math.Abs(lon-132.5) / 22.5
			australia = math.Max(0, 1.0 - math.Sqrt(latDist*latDist + lonDist*lonDist))
		}
		
		continentalness := math.Max(europe, math.Max(africa, math.Max(americas, math.Max(asia, australia))))
		variation := 0.05 * math.Sin(lat*0.1) * math.Cos(lon*0.1)
		continentalness += variation
		
		material := "water"
		if continentalness > 0.5 {
			material = "land"
		} else if continentalness > 0.2 {
			elevation := continentalness - 0.2 + 0.1*math.Sin(lat*0.05)*math.Cos(lon*0.05)
			if elevation > 0.15 {
				material = "land"
			}
		}
		
		fmt.Printf("%s (%.1f, %.1f): cont=%.3f, var=%.3f, total=%.3f -> %s\n", 
			loc.name, lat, lon, continentalness-variation, variation, continentalness, material)
	}
}