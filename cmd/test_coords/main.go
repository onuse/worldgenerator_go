package main

import (
	"fmt"
	"math"
	"worldgenerator/core"
)

func main() {
	fmt.Println("=== Coordinate System Test ===\n")

	// Test 1: Basic conversions
	fmt.Println("Test 1: Geographic to Cartesian conversions")
	testPositions := []struct {
		name     string
		lat, lon float64 // degrees
	}{
		{"North Pole", 90, 0},
		{"South Pole", -90, 0},
		{"Equator 0°", 0, 0},
		{"Equator 90°E", 0, 90},
		{"45°N 45°E", 45, 45},
	}

	radius := 6371000.0 // Earth radius in meters

	for _, pos := range testPositions {
		geo := core.Geographic{
			Lat: core.DegreesToRadians(pos.lat),
			Lon: core.DegreesToRadians(pos.lon),
			Alt: 0,
		}

		cart := core.GeographicToCartesian(geo, radius)
		geoBack := core.CartesianToGeographic(cart, radius)

		fmt.Printf("%s (%.0f°, %.0f°):\n", pos.name, pos.lat, pos.lon)
		fmt.Printf("  Cartesian: X=%.0f, Y=%.0f, Z=%.0f\n", cart.X, cart.Y, cart.Z)
		fmt.Printf("  Back to Geo: %.2f°, %.2f°\n",
			core.RadiansToDegrees(geoBack.Lat),
			core.RadiansToDegrees(geoBack.Lon))
		fmt.Println()
	}

	// Test 2: Velocity transformations
	fmt.Println("\nTest 2: Velocity transformations")
	velocityTests := []struct {
		name               string
		lat, lon           float64 // degrees
		vNorth, vEast, vUp float64 // m/s
	}{
		{"Northward at equator", 0, 0, 10, 0, 0},
		{"Eastward at equator", 0, 0, 0, 10, 0},
		{"Northward at 45°N", 45, 0, 10, 0, 0},
		{"Eastward at 60°N", 60, 0, 0, 10, 0},
	}

	for _, test := range velocityTests {
		pos := core.Geographic{
			Lat: core.DegreesToRadians(test.lat),
			Lon: core.DegreesToRadians(test.lon),
			Alt: 0,
		}

		geoVel := core.GeographicVelocity{
			VNorth: test.vNorth,
			VEast:  test.vEast,
			VUp:    test.vUp,
		}

		cartVel := core.GeographicVelocityToCartesian(geoVel, pos)
		geoVelBack := core.CartesianVelocityToGeographic(cartVel, pos)

		fmt.Printf("%s at (%.0f°, %.0f°):\n", test.name, test.lat, test.lon)
		fmt.Printf("  Geographic vel: N=%.1f, E=%.1f, U=%.1f m/s\n",
			geoVel.VNorth, geoVel.VEast, geoVel.VUp)
		fmt.Printf("  Cartesian vel: X=%.1f, Y=%.1f, Z=%.1f m/s\n",
			cartVel.VX, cartVel.VY, cartVel.VZ)
		fmt.Printf("  Back to Geo: N=%.1f, E=%.1f, U=%.1f m/s\n",
			geoVelBack.VNorth, geoVelBack.VEast, geoVelBack.VUp)

		// Verify magnitude is preserved
		geoMag := math.Sqrt(geoVel.VNorth*geoVel.VNorth +
			geoVel.VEast*geoVel.VEast + geoVel.VUp*geoVel.VUp)
		cartMag := math.Sqrt(cartVel.VX*cartVel.VX +
			cartVel.VY*cartVel.VY + cartVel.VZ*cartVel.VZ)
		fmt.Printf("  Magnitude: Geo=%.1f, Cart=%.1f\n", geoMag, cartMag)
		fmt.Println()
	}

	// Test 3: Grid mapping
	fmt.Println("\nTest 3: Grid index mapping")
	latBands := 180
	gridTests := []float64{-90, -45, 0, 45, 90}

	for _, lat := range gridTests {
		band := core.GetBandForLatitude(lat, latBands)
		latBack := core.GetLatitudeForBand(band, latBands)
		fmt.Printf("Latitude %.0f° -> Band %d -> %.1f°\n", lat, band, latBack)
	}

	// Test 4: Compatibility with VoxelMaterial
	fmt.Println("\nTest 4: VoxelMaterial compatibility")
	voxel := &core.VoxelMaterial{
		VelR:     1.0,
		VelNorth: 10.0, // North velocity
		VelEast:  5.0,  // East velocity
	}

	geoVel := voxel.GetVelocityGeographic()
	fmt.Printf("VoxelMaterial velocity:\n")
	fmt.Printf("  Spherical: VR=%.1f, VTheta=%.1f, VPhi=%.1f\n",
		voxel.VelR, voxel.VelNorth, voxel.VelEast)
	fmt.Printf("  Geographic: N=%.1f, E=%.1f, U=%.1f\n",
		geoVel.VNorth, geoVel.VEast, geoVel.VUp)
}
