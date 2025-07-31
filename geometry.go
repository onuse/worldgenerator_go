package main

import "math"

func sqrt(x float64) float64 {
	return math.Sqrt(x)
}

func distance(a, b Vector3) float64 {
	return sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
}

func generateIcosphere(subdivisions int) Planet {
	// Golden ratio
	t := (1.0 + math.Sqrt(5.0)) / 2.0

	// Initial icosahedron vertices
	vertices := []Vertex{
		{Vector3{-1, t, 0}, 0, 0, 0, 0}, {Vector3{1, t, 0}, 0, 0, 0, 0}, {Vector3{-1, -t, 0}, 0, 0, 0, 0}, {Vector3{1, -t, 0}, 0, 0, 0, 0},
		{Vector3{0, -1, t}, 0, 0, 0, 0}, {Vector3{0, 1, t}, 0, 0, 0, 0}, {Vector3{0, -1, -t}, 0, 0, 0, 0}, {Vector3{0, 1, -t}, 0, 0, 0, 0},
		{Vector3{t, 0, -1}, 0, 0, 0, 0}, {Vector3{t, 0, 1}, 0, 0, 0, 0}, {Vector3{-t, 0, -1}, 0, 0, 0, 0}, {Vector3{-t, 0, 1}, 0, 0, 0, 0},
	}

	// Initial icosahedron faces
	indices := []int32{
		0, 11, 5, 0, 5, 1, 0, 1, 7, 0, 7, 10, 0, 10, 11,
		1, 5, 9, 5, 11, 4, 11, 10, 2, 10, 7, 6, 7, 1, 8,
		3, 9, 4, 3, 4, 2, 3, 2, 6, 3, 6, 8, 3, 8, 9,
		4, 9, 5, 2, 4, 11, 6, 2, 10, 8, 6, 7, 9, 8, 1,
	}

	planet := Planet{Vertices: vertices, Indices: indices}

	// Subdivide
	for i := 0; i < subdivisions; i++ {
		planet = subdivide(planet)
	}

	// Normalize all vertices to unit sphere, then apply polar flattening
	for i := range planet.Vertices {
		planet.Vertices[i].Position = planet.Vertices[i].Position.Normalize()
		
		// Apply Earth-like polar flattening (Y is up axis)
		flattening := 0.003 // Reduced flattening for more spherical planet
		planet.Vertices[i].Position.Y *= (1.0 - flattening)
	}

	return planet
}

func subdivide(planet Planet) Planet {
	midpoints := make(map[[2]int32]int32)
	newVertices := make([]Vertex, len(planet.Vertices))
	copy(newVertices, planet.Vertices)
	var newIndices []int32

	getMidpoint := func(i1, i2 int32) int32 {
		key := [2]int32{i1, i2}
		if i1 > i2 {
			key = [2]int32{i2, i1}
		}
		if mid, exists := midpoints[key]; exists {
			return mid
		}

		v1, v2 := planet.Vertices[i1], planet.Vertices[i2]
		midVertex := Vertex{
			Position: Vector3{
				X: (v1.Position.X + v2.Position.X) / 2,
				Y: (v1.Position.Y + v2.Position.Y) / 2,
				Z: (v1.Position.Z + v2.Position.Z) / 2,
			},
		}
		newVertices = append(newVertices, midVertex)
		midpoints[key] = int32(len(newVertices) - 1)
		return midpoints[key]
	}

	for i := 0; i < len(planet.Indices); i += 3 {
		v1, v2, v3 := planet.Indices[i], planet.Indices[i+1], planet.Indices[i+2]
		m1 := getMidpoint(v1, v2)
		m2 := getMidpoint(v2, v3)
		m3 := getMidpoint(v3, v1)

		newIndices = append(newIndices, v1, m1, m3, v2, m2, m1, v3, m3, m2, m1, m2, m3)
	}

	return Planet{Vertices: newVertices, Indices: newIndices}
}

// Simple but effective noise function
func terrainNoise(x, y, z float64) float64 {
	// Create varied terrain using multiple sine waves
	n1 := math.Sin(x*3.14159)*math.Cos(y*2.71828)*math.Sin(z*1.41421)
	n2 := math.Sin(x*1.73205)*math.Sin(y*2.23607)*math.Cos(z*3.16227)
	n3 := math.Cos(x*2.44949)*math.Sin(y*1.61803)*math.Sin(z*2.64575)
	return (n1 + n2*0.5 + n3*0.25) / 1.75
}

// Ridge noise for mountain ridges
func ridgeNoise(x, y, z float64) float64 {
	return 1.0 - math.Abs(terrainNoise(x, y, z))
}

// Keep old function for compatibility
func simplexNoise(x, y, z float64) float64 {
	return terrainNoise(x, y, z)
}