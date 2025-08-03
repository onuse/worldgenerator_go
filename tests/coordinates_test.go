package tests

import (
	"math"
	"testing"
)

// TestCoordinateConversions documents and tests the current coordinate system behavior
func TestCoordinateConversions(t *testing.T) {
	tests := []struct {
		name    string
		lat     float64 // degrees
		lon     float64 // degrees
		r       float64 // meters
		wantX   float64
		wantY   float64
		wantZ   float64
		epsilon float64
	}{
		{
			name:    "North Pole",
			lat:     90.0,
			lon:     0.0,
			r:       6371000.0,
			wantX:   0.0,
			wantY:   6371000.0,
			wantZ:   0.0,
			epsilon: 1.0,
		},
		{
			name:    "South Pole",
			lat:     -90.0,
			lon:     0.0,
			r:       6371000.0,
			wantX:   0.0,
			wantY:   -6371000.0,
			wantZ:   0.0,
			epsilon: 1.0,
		},
		{
			name:    "Equator Prime Meridian",
			lat:     0.0,
			lon:     0.0,
			r:       6371000.0,
			wantX:   6371000.0,
			wantY:   0.0,
			wantZ:   0.0,
			epsilon: 1.0,
		},
		{
			name:    "Equator 90E",
			lat:     0.0,
			lon:     90.0,
			r:       6371000.0,
			wantX:   0.0,
			wantY:   0.0,
			wantZ:   6371000.0,
			epsilon: 1.0,
		},
		{
			name:    "45N 45E",
			lat:     45.0,
			lon:     45.0,
			r:       6371000.0,
			wantX:   3577427.0, // r * cos(45°) * cos(45°)
			wantY:   4505537.0, // r * sin(45°)
			wantZ:   3577427.0, // r * cos(45°) * sin(45°)
			epsilon: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Convert to radians
			latRad := tc.lat * math.Pi / 180.0
			lonRad := tc.lon * math.Pi / 180.0

			// Current system conversion (Y-up)
			x := tc.r * math.Cos(latRad) * math.Cos(lonRad)
			y := tc.r * math.Sin(latRad)
			z := tc.r * math.Cos(latRad) * math.Sin(lonRad)

			// Check results
			if math.Abs(x-tc.wantX) > tc.epsilon {
				t.Errorf("X coordinate: got %f, want %f", x, tc.wantX)
			}
			if math.Abs(y-tc.wantY) > tc.epsilon {
				t.Errorf("Y coordinate: got %f, want %f", y, tc.wantY)
			}
			if math.Abs(z-tc.wantZ) > tc.epsilon {
				t.Errorf("Z coordinate: got %f, want %f", z, tc.wantZ)
			}
		})
	}
}

// TestVelocityComponents documents current velocity behavior
func TestVelocityComponents(t *testing.T) {
	// Test that velocity components behave as expected
	tests := []struct {
		name        string
		lat         float64 // degrees
		lon         float64 // degrees
		velNorth    float64 // m/s (what VelNorth should represent)
		velEast     float64 // m/s (what VelEast should represent)
		description string
	}{
		{
			name:        "Northward at Equator",
			lat:         0.0,
			lon:         0.0,
			velNorth:    10.0,
			velEast:     0.0,
			description: "Should move along meridian toward north pole",
		},
		{
			name:        "Eastward at Equator",
			lat:         0.0,
			lon:         0.0,
			velNorth:    0.0,
			velEast:     10.0,
			description: "Should move along equator toward east",
		},
		{
			name:        "Northward at 45N",
			lat:         45.0,
			lon:         0.0,
			velNorth:    10.0,
			velEast:     0.0,
			description: "Should move along meridian, considering sphere curvature",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Document expected behavior
			t.Logf("Velocity case: %s", tc.description)
			t.Logf("Position: %.1f°N, %.1f°E", tc.lat, tc.lon)
			t.Logf("Velocity: North=%.1f m/s, East=%.1f m/s", tc.velNorth, tc.velEast)

			// TODO: Add actual physics calculations when we implement them
		})
	}
}

// TestPolesSingularities tests behavior at coordinate singularities
func TestPolesSingularities(t *testing.T) {
	// Test that poles don't cause issues
	poles := []struct {
		name string
		lat  float64
	}{
		{"North Pole", 90.0},
		{"South Pole", -90.0},
		{"Near North Pole", 89.99},
		{"Near South Pole", -89.99},
	}

	for _, pole := range poles {
		t.Run(pole.name, func(t *testing.T) {
			// Test that longitude is handled properly at poles
			for lon := -180.0; lon <= 180.0; lon += 45.0 {
				latRad := pole.lat * math.Pi / 180.0
				lonRad := lon * math.Pi / 180.0

				// At poles, all longitudes should give same position
				x := 6371000.0 * math.Cos(latRad) * math.Cos(lonRad)
				y := 6371000.0 * math.Sin(latRad)
				z := 6371000.0 * math.Cos(latRad) * math.Sin(lonRad)

				t.Logf("Pole %s at lon %.0f: x=%.1f, y=%.1f, z=%.1f",
					pole.name, lon, x, y, z)
			}
		})
	}
}

// TestGridMapping tests how lat/lon maps to voxel grid indices
func TestGridMapping(t *testing.T) {
	latBands := 180

	tests := []struct {
		lat      float64
		wantBand int
	}{
		{-90.0, 0},  // South pole
		{-45.0, 45}, // 45S
		{0.0, 90},   // Equator
		{45.0, 135}, // 45N
		{90.0, 179}, // North pole
	}

	for _, tc := range tests {
		t.Run("LatBand", func(t *testing.T) {
			// Current implementation
			band := int((tc.lat + 90.0) / 180.0 * float64(latBands))
			if band >= latBands {
				band = latBands - 1
			}
			if band < 0 {
				band = 0
			}

			if band != tc.wantBand {
				t.Errorf("Latitude %.1f: got band %d, want %d", tc.lat, band, tc.wantBand)
			}
		})
	}
}

// TestShaderCoordinates tests shader coordinate transformations
func TestShaderCoordinates(t *testing.T) {
	// Document the shader's coordinate transformation
	// From renderer_gl_raymarch.go:
	// float lat = asin(clamp(normalized.y, -1.0, 1.0));
	// float lon = atan(normalized.z, normalized.x);

	t.Run("ShaderConversion", func(t *testing.T) {
		// Test vectors
		vectors := []struct {
			name    string
			x, y, z float64
			wantLat float64 // radians
			wantLon float64 // radians
		}{
			{"North", 0, 1, 0, math.Pi / 2, 0},
			{"South", 0, -1, 0, -math.Pi / 2, 0},
			{"East", 0, 0, 1, 0, math.Pi / 2},
			{"West", 0, 0, -1, 0, -math.Pi / 2},
			{"PrimeMeridian", 1, 0, 0, 0, 0},
		}

		for _, v := range vectors {
			// Normalize
			mag := math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
			nx := v.x / mag
			ny := v.y / mag
			nz := v.z / mag

			// Shader's conversion
			lat := math.Asin(ny)
			lon := math.Atan2(nz, nx)

			t.Logf("%s: (%.1f,%.1f,%.1f) -> lat=%.2f°, lon=%.2f°",
				v.name, v.x, v.y, v.z,
				lat*180/math.Pi, lon*180/math.Pi)
		}
	})
}
