package core

import (
	"math"
)

// GenerateSphereData generates vertex and index data for a UV sphere
// Returns vertices (position, normal, texcoord) and indices
func GenerateSphereData(radius float32, segments, rings int) ([]float32, []uint32) {
	// Use default values if not specified
	if segments <= 0 {
		segments = 64
	}
	if rings <= 0 {
		rings = 32
	}

	// Generate vertices
	var vertices []float32
	var indices []uint32

	// Create vertices
	for ring := 0; ring <= rings; ring++ {
		theta := float32(ring) * math.Pi / float32(rings)
		sinTheta := float32(math.Sin(float64(theta)))
		cosTheta := float32(math.Cos(float64(theta)))

		for seg := 0; seg <= segments; seg++ {
			phi := float32(seg) * 2.0 * math.Pi / float32(segments)
			sinPhi := float32(math.Sin(float64(phi)))
			cosPhi := float32(math.Cos(float64(phi)))

			// Position
			x := cosPhi * sinTheta
			y := cosTheta
			z := sinPhi * sinTheta

			// Scale to radius
			vertices = append(vertices, x*radius, y*radius, z*radius)

			// Normal (same as position for unit sphere)
			vertices = append(vertices, x, y, z)

			// Texture coordinates
			u := float32(seg) / float32(segments)
			v := float32(ring) / float32(rings)
			vertices = append(vertices, u, v)
		}
	}

	// Create indices
	for ring := 0; ring < rings; ring++ {
		for seg := 0; seg < segments; seg++ {
			current := uint32(ring*(segments+1) + seg)
			next := current + uint32(segments) + 1

			// First triangle
			indices = append(indices, current, next, current+1)

			// Second triangle
			indices = append(indices, current+1, next, next+1)
		}
	}

	return vertices, indices
}
