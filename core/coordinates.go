package core

import (
	"math"
)

// Geographic represents a position in geographic coordinates
type Geographic struct {
	Lat float64 // Latitude in radians [-π/2, π/2], positive = north
	Lon float64 // Longitude in radians [-π, π], positive = east
	Alt float64 // Altitude above reference radius in meters
}

// Cartesian represents a position in Cartesian coordinates
// Origin at planet center, Y points to north pole
type Cartesian struct {
	X float64 // Points to 0° longitude at equator
	Y float64 // Points to north pole
	Z float64 // Points to 90° longitude at equator
}

// GeographicVelocity represents velocity in the local geographic frame
type GeographicVelocity struct {
	VNorth float64 // Northward velocity (m/s)
	VEast  float64 // Eastward velocity (m/s)
	VUp    float64 // Upward/radial velocity (m/s)
}

// CartesianVelocity represents velocity in Cartesian coordinates
type CartesianVelocity struct {
	VX float64 // X velocity (m/s)
	VY float64 // Y velocity (m/s)
	VZ float64 // Z velocity (m/s)
}

// SphericalVelocity represents velocity in spherical coordinates
// This matches the current VoxelMaterial fields
type SphericalVelocity struct {
	VR     float64 // Radial velocity (m/s)
	VTheta float64 // Latitudinal velocity (m/s) - currently named VelNorth
	VPhi   float64 // Longitudinal velocity (m/s) - currently named VelEast
}

// DegreesToRadians converts degrees to radians
func DegreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180.0
}

// RadiansToDegrees converts radians to degrees
func RadiansToDegrees(radians float64) float64 {
	return radians * 180.0 / math.Pi
}

// GeographicToCartesian converts geographic coordinates to Cartesian
func GeographicToCartesian(g Geographic, radius float64) Cartesian {
	r := radius + g.Alt
	cosLat := math.Cos(g.Lat)

	return Cartesian{
		X: r * cosLat * math.Cos(g.Lon),
		Y: r * math.Sin(g.Lat),
		Z: r * cosLat * math.Sin(g.Lon),
	}
}

// CartesianToGeographic converts Cartesian coordinates to geographic
func CartesianToGeographic(c Cartesian, radius float64) Geographic {
	r := math.Sqrt(c.X*c.X + c.Y*c.Y + c.Z*c.Z)

	// Handle special case of origin
	if r < 1e-10 {
		return Geographic{Lat: 0, Lon: 0, Alt: -radius}
	}

	return Geographic{
		Lat: math.Asin(c.Y / r),
		Lon: math.Atan2(c.Z, c.X),
		Alt: r - radius,
	}
}

// GeographicVelocityToCartesian converts velocity from geographic to Cartesian frame
// at a given position
func GeographicVelocityToCartesian(vel GeographicVelocity, pos Geographic) CartesianVelocity {
	// Transformation matrix from geographic to Cartesian
	sinLat := math.Sin(pos.Lat)
	cosLat := math.Cos(pos.Lat)
	sinLon := math.Sin(pos.Lon)
	cosLon := math.Cos(pos.Lon)

	// Transform velocity components
	return CartesianVelocity{
		VX: -sinLat*cosLon*vel.VNorth - sinLon*vel.VEast + cosLat*cosLon*vel.VUp,
		VY: cosLat*vel.VNorth + sinLat*vel.VUp,
		VZ: -sinLat*sinLon*vel.VNorth + cosLon*vel.VEast + cosLat*sinLon*vel.VUp,
	}
}

// CartesianVelocityToGeographic converts velocity from Cartesian to geographic frame
// at a given position
func CartesianVelocityToGeographic(vel CartesianVelocity, pos Geographic) GeographicVelocity {
	// Transformation matrix from Cartesian to geographic (transpose of above)
	sinLat := math.Sin(pos.Lat)
	cosLat := math.Cos(pos.Lat)
	sinLon := math.Sin(pos.Lon)
	cosLon := math.Cos(pos.Lon)

	return GeographicVelocity{
		VNorth: -sinLat*cosLon*vel.VX + cosLat*vel.VY - sinLat*sinLon*vel.VZ,
		VEast:  -sinLon*vel.VX + cosLon*vel.VZ,
		VUp:    cosLat*cosLon*vel.VX + sinLat*vel.VY + cosLat*sinLon*vel.VZ,
	}
}

// SphericalToGeographic converts the current spherical velocity representation
// to the clearer geographic velocity
func SphericalToGeographic(svel SphericalVelocity) GeographicVelocity {
	// Current system: VTheta is latitude direction, VPhi is longitude direction
	return GeographicVelocity{
		VNorth: svel.VTheta,
		VEast:  svel.VPhi,
		VUp:    svel.VR,
	}
}

// GeographicToSpherical converts geographic velocity to the current spherical representation
func GeographicToSpherical(gvel GeographicVelocity) SphericalVelocity {
	return SphericalVelocity{
		VR:     gvel.VUp,
		VTheta: gvel.VNorth,
		VPhi:   gvel.VEast,
	}
}

// GetLatitudeForBand returns the latitude in degrees for a given latitude band index
// This matches the existing function but documents the conversion
func GetLatitudeForBand(bandIndex int, totalBands int) float64 {
	// Linear mapping from band index to latitude
	// Band 0 = -90° (south pole)
	// Band (totalBands-1) = +90° (north pole)
	return -90.0 + float64(bandIndex)*180.0/float64(totalBands-1)
}

// GetBandForLatitude returns the latitude band index for a given latitude in degrees
func GetBandForLatitude(latDegrees float64, totalBands int) int {
	// Inverse of GetLatitudeForBand
	band := int((latDegrees+90.0)*float64(totalBands-1)/180.0 + 0.5)

	// Clamp to valid range
	if band < 0 {
		return 0
	}
	if band >= totalBands {
		return totalBands - 1
	}
	return band
}

// GetLongitudeForIndex returns the longitude in degrees for a given index
// in a latitude band with the specified number of divisions
func GetLongitudeForIndex(lonIndex int, lonCount int) float64 {
	// Uniform spacing around the circle
	return -180.0 + float64(lonIndex)*360.0/float64(lonCount)
}

// GetIndexForLongitude returns the longitude index for a given longitude
// in a latitude band with the specified number of divisions
func GetIndexForLongitude(lonDegrees float64, lonCount int) int {
	// Normalize to [0, 360)
	lon := lonDegrees
	for lon < -180.0 {
		lon += 360.0
	}
	for lon >= 180.0 {
		lon -= 360.0
	}

	// Convert to index
	index := int((lon + 180.0) * float64(lonCount) / 360.0)

	// Wrap around
	return (index + lonCount) % lonCount
}

// ValidateCoordinates checks if coordinates are within valid ranges
func ValidateCoordinates(g Geographic) bool {
	return g.Lat >= -math.Pi/2 && g.Lat <= math.Pi/2 &&
		g.Lon >= -math.Pi && g.Lon <= math.Pi
}

// NormalizeCoordinates ensures coordinates are within valid ranges
func NormalizeCoordinates(g Geographic) Geographic {
	// Clamp latitude
	if g.Lat > math.Pi/2 {
		g.Lat = math.Pi / 2
	} else if g.Lat < -math.Pi/2 {
		g.Lat = -math.Pi / 2
	}

	// Wrap longitude
	for g.Lon > math.Pi {
		g.Lon -= 2 * math.Pi
	}
	for g.Lon < -math.Pi {
		g.Lon += 2 * math.Pi
	}

	return g
}
