package main

import (
	"fmt"
	"math"
	"math/rand"
)

func generateTectonicPlates(planet Planet, numPlates int) Planet {
	// First, generate realistic terrain using fractal noise
	fmt.Println("Generating fractal terrain...")
	planet = generateRealisticContinents(planet)
	
	// Then create plates organically based on the terrain
	fmt.Println("Creating organic plates from terrain...")
	planet = createOrganicPlates(planet, numPlates)
	
	// Find and classify plate boundaries
	planet.Boundaries = findPlateBoundaries(planet)
	
	// Debug output
	minH, maxH := planet.Vertices[0].Height, planet.Vertices[0].Height
	landCount, oceanCount := 0, 0
	for _, v := range planet.Vertices {
		if v.Height < minH { minH = v.Height }
		if v.Height > maxH { maxH = v.Height }
		if v.Height > 0 { landCount++ } else { oceanCount++ }
	}
	fmt.Printf("Terrain: %.6f to %.6f (Land: %d, Ocean: %d, Plates: %d)\n", minH, maxH, landCount, oceanCount, len(planet.Plates))
	
	return planet
}

func generatePlateVelocity(center Vector3) Vector3 {
	// Generate random tangent vector to sphere at this point
	// Use cross product with a non-parallel vector
	up := Vector3{0, 1, 0}
	if math.Abs(center.Dot(up)) > 0.9 {
		up = Vector3{1, 0, 0} // Use different vector if center is too close to Y axis
	}
	
	// Create tangent vector
	tangent := Vector3{
		X: up.Y*center.Z - up.Z*center.Y,
		Y: up.Z*center.X - up.X*center.Z,
		Z: up.X*center.Y - up.Y*center.X,
	}.Normalize()
	
	// Random speed and slight rotation (increased for visible simulation)
	speed := 0.01 + rand.Float64()*0.02 // 0.01-0.03 units per time
	angle := rand.Float64() * 2 * math.Pi
	
	// Rotate tangent vector around the normal (center)
	cos_a, sin_a := math.Cos(angle), math.Sin(angle)
	
	// Another tangent perpendicular to the first
	tangent2 := Vector3{
		X: center.Y*tangent.Z - center.Z*tangent.Y,
		Y: center.Z*tangent.X - center.X*tangent.Z,
		Z: center.X*tangent.Y - center.Y*tangent.X,
	}.Normalize()
	
	// Combine the two tangents with rotation
	velocity := tangent.Scale(cos_a).Add(tangent2.Scale(sin_a)).Scale(speed)
	
	return velocity
}

func findPlateBoundaries(planet Planet) []PlateBoundary {
	var boundaries []PlateBoundary
	boundaryMap := make(map[[2]int][]int) // [plate1, plate2] -> vertex indices
	
	// Find edges between different plates
	for i := 0; i < len(planet.Indices); i += 3 {
		v1 := int(planet.Indices[i])
		v2 := int(planet.Indices[i+1])
		v3 := int(planet.Indices[i+2])
		
		vertices := []int{v1, v2, v3}
		
		// Check each edge of the triangle
		for j := 0; j < 3; j++ {
			va := vertices[j]
			vb := vertices[(j+1)%3]
			
			plateA := planet.Vertices[va].PlateID
			plateB := planet.Vertices[vb].PlateID
			
			if plateA != plateB {
				// Create boundary key (smaller plate ID first)
				key := [2]int{plateA, plateB}
				if plateA > plateB {
					key = [2]int{plateB, plateA}
				}
				
				// Add vertices to boundary
				boundaryMap[key] = append(boundaryMap[key], va, vb)
			}
		}
	}
	
	// Convert map to boundary structs
	for key, vertices := range boundaryMap {
		boundaryType := determineBoundaryType(planet.Plates[key[0]], planet.Plates[key[1]])
		
		boundary := PlateBoundary{
			Plate1:       key[0],
			Plate2:       key[1],
			Type:         boundaryType,
			EdgeVertices: removeDuplicates(vertices),
		}
		
		boundaries = append(boundaries, boundary)
	}
	
	return boundaries
}

func determineBoundaryType(plate1, plate2 Plate) BoundaryType {
	// Calculate relative velocity
	relVel := plate1.Velocity.Add(plate2.Velocity.Scale(-1))
	
	// Calculate direction between plate centers
	direction := plate2.Center.Add(plate1.Center.Scale(-1)).Normalize()
	
	// Dot product determines boundary type
	dot := relVel.Dot(direction)
	
	// Use more balanced thresholds to ensure variety
	if dot < -0.001 {
		return Convergent // Plates moving toward each other
	} else if dot > 0.001 {
		return Divergent // Plates moving away from each other
	} else {
		return Transform // Plates sliding past each other
	}
}

func removeDuplicates(vertices []int) []int {
	seen := make(map[int]bool)
	result := []int{}
	
	for _, v := range vertices {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	
	return result
}

// createOrganicPlates creates plates based on natural terrain features
func createOrganicPlates(planet Planet, targetPlateCount int) Planet {
	// Find natural boundaries in the terrain
	regions := findNaturalRegions(planet)
	
	// Merge small regions until we get close to target plate count
	regions = mergeSmallRegions(planet, regions, targetPlateCount)
	
	// Convert regions to plates
	plates := []Plate{}
	for i, region := range regions {
		if len(region) < 100 { // Skip tiny regions
			continue
		}
		
		center := calculatePlateCenter(planet, region)
		
		// Determine plate type based on average height
		avgHeight := 0.0
		for _, idx := range region {
			avgHeight += planet.Vertices[idx].Height
		}
		avgHeight /= float64(len(region))
		
		plateType := Oceanic
		if avgHeight > -0.002 { // Mostly above sea level
			plateType = Continental
		}
		
		plates = append(plates, Plate{
			ID:       i,
			Center:   center,
			Velocity: generateRealisticVelocity(center, plateType),
			Type:     plateType,
			Vertices: region,
		})
	}
	
	// Assign vertices to plates
	for i := range planet.Vertices {
		planet.Vertices[i].PlateID = -1
	}
	
	for i, plate := range plates {
		for _, idx := range plate.Vertices {
			if idx < len(planet.Vertices) {
				planet.Vertices[idx].PlateID = i
			}
		}
	}
	
	// Assign any unassigned vertices to nearest plate
	for i := range planet.Vertices {
		if planet.Vertices[i].PlateID == -1 {
			minDist := math.MaxFloat64
			closestPlate := 0
			for j, plate := range plates {
				dist := distance(planet.Vertices[i].Position, plate.Center)
				if dist < minDist {
					minDist = dist
					closestPlate = j
				}
			}
			planet.Vertices[i].PlateID = closestPlate
			plates[closestPlate].Vertices = append(plates[closestPlate].Vertices, i)
		}
	}
	
	planet.Plates = plates
	return planet
}

// findNaturalRegions finds regions based on terrain gradients and features
func findNaturalRegions(planet Planet) [][]int {
	visited := make([]bool, len(planet.Vertices))
	var regions [][]int
	
	// Find regions based on terrain similarity
	for i := range planet.Vertices {
		if !visited[i] {
			region := growRegion(planet, i, visited)
			if len(region) > 50 { // Minimum region size
				regions = append(regions, region)
			}
		}
	}
	
	return regions
}

// growRegion grows a region from a seed based on terrain similarity
func growRegion(planet Planet, seedIdx int, visited []bool) []int {
	if visited[seedIdx] {
		return []int{}
	}
	
	seedHeight := planet.Vertices[seedIdx].Height
	isOcean := seedHeight < -0.001
	
	region := []int{}
	stack := []int{seedIdx}
	
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		
		if visited[idx] {
			continue
		}
		
		h := planet.Vertices[idx].Height
		
		// Check if vertex belongs to same terrain type
		if isOcean && h >= -0.001 { // Ocean to land boundary
			continue
		}
		if !isOcean && h < -0.001 { // Land to ocean boundary
			continue
		}
		
		// Additional check for large height differences (natural boundaries)
		if math.Abs(h - seedHeight) > 0.015 {
			continue
		}
		
		visited[idx] = true
		region = append(region, idx)
		
		// Find neighbors through triangles
		neighbors := findVertexNeighbors(planet, idx)
		for _, neighbor := range neighbors {
			if !visited[neighbor] {
				stack = append(stack, neighbor)
			}
		}
	}
	
	return region
}

// findVertexNeighbors finds neighboring vertices through shared triangles
func findVertexNeighbors(planet Planet, vertexIdx int) []int {
	neighborSet := make(map[int]bool)
	
	// Look through all triangles to find neighbors
	for i := 0; i < len(planet.Indices); i += 3 {
		v1, v2, v3 := int(planet.Indices[i]), int(planet.Indices[i+1]), int(planet.Indices[i+2])
		
		if v1 == vertexIdx {
			neighborSet[v2] = true
			neighborSet[v3] = true
		} else if v2 == vertexIdx {
			neighborSet[v1] = true
			neighborSet[v3] = true
		} else if v3 == vertexIdx {
			neighborSet[v1] = true
			neighborSet[v2] = true
		}
	}
	
	neighbors := []int{}
	for n := range neighborSet {
		neighbors = append(neighbors, n)
	}
	
	return neighbors
}

// mergeSmallRegions merges regions until we get close to target count
func mergeSmallRegions(planet Planet, regions [][]int, targetCount int) [][]int {
	for len(regions) > targetCount*2 { // Allow some variation
		// Find smallest region
		smallestIdx := 0
		smallestSize := len(regions[0])
		for i, r := range regions {
			if len(r) < smallestSize {
				smallestIdx = i
				smallestSize = len(r)
			}
		}
		
		// Find best neighbor to merge with
		bestNeighbor := findBestNeighborRegion(planet, regions, smallestIdx)
		if bestNeighbor >= 0 && bestNeighbor != smallestIdx {
			// Merge regions
			regions[bestNeighbor] = append(regions[bestNeighbor], regions[smallestIdx]...)
			// Remove smallest region
			regions = append(regions[:smallestIdx], regions[smallestIdx+1:]...)
		} else {
			break // No suitable merge found
		}
	}
	
	return regions
}

// findBestNeighborRegion finds the best neighboring region to merge with
func findBestNeighborRegion(planet Planet, regions [][]int, regionIdx int) int {
	region := regions[regionIdx]
	if len(region) == 0 {
		return -1
	}
	
	// Count borders with other regions
	borderCounts := make(map[int]int)
	
	for _, vertexIdx := range region {
		neighbors := findVertexNeighbors(planet, vertexIdx)
		for _, n := range neighbors {
			// Find which region this neighbor belongs to
			for i, r := range regions {
				if i == regionIdx {
					continue
				}
				for _, v := range r {
					if v == n {
						borderCounts[i]++
						break
					}
				}
			}
		}
	}
	
	// Find region with most shared border
	bestRegion := -1
	maxBorder := 0
	for r, count := range borderCounts {
		if count > maxBorder {
			maxBorder = count
			bestRegion = r
		}
	}
	
	return bestRegion
}

// generateRealisticVelocity generates plate velocity based on real tectonics
func generateRealisticVelocity(center Vector3, plateType PlateType) Vector3 {
	// Create tangent vector to sphere
	up := Vector3{0, 1, 0}
	if math.Abs(center.Dot(up)) > 0.9 {
		up = Vector3{1, 0, 0}
	}
	
	tangent1 := Vector3{
		X: up.Y*center.Z - up.Z*center.Y,
		Y: up.Z*center.X - up.X*center.Z,
		Z: up.X*center.Y - up.Y*center.X,
	}.Normalize()
	
	tangent2 := Vector3{
		X: center.Y*tangent1.Z - center.Z*tangent1.Y,
		Y: center.Z*tangent1.X - center.X*tangent1.Z,
		Z: center.X*tangent1.Y - center.Y*tangent1.X,
	}.Normalize()
	
	// Realistic plate speeds (scaled for simulation)
	var speed float64
	if plateType == Oceanic {
		speed = 0.002 + rand.Float64()*0.003 // Oceanic plates move faster
	} else {
		speed = 0.001 + rand.Float64()*0.002 // Continental plates move slower
	}
	
	// Random direction
	angle := rand.Float64() * 2 * math.Pi
	cos_a, sin_a := math.Cos(angle), math.Sin(angle)
	
	return tangent1.Scale(cos_a * speed).Add(tangent2.Scale(sin_a * speed))
}

// calculatePlateCenter calculates the center of mass for a region
func calculatePlateCenter(planet Planet, vertices []int) Vector3 {
	if len(vertices) == 0 {
		return Vector3{0, 0, 0}
	}
	
	center := Vector3{0, 0, 0}
	for _, idx := range vertices {
		if idx < len(planet.Vertices) {
			center = center.Add(planet.Vertices[idx].Position)
		}
	}
	
	return center.Scale(1.0 / float64(len(vertices))).Normalize()
}