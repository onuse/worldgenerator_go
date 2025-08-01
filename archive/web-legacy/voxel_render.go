package main

import (
	"math"
)

// SimplifiedSurfaceExtraction creates a simple sphere mesh colored by surface material
func (p *VoxelPlanet) SimplifiedSurfaceExtraction() *TriangleMesh {
	// Create mesh
	mesh := &TriangleMesh{
		Vertices:  make([]Vector3, 0, 10000),
		Normals:   make([]Vector3, 0, 10000),
		Colors:    make([]Vector3, 0, 10000),
		Triangles: make([]int32, 0, 30000),
	}
	
	// Generate sphere vertices
	latDivisions := 60
	lonDivisions := 120
	
	vertexMap := make(map[[2]int]int)
	
	// Create vertices
	for latIdx := 0; latIdx <= latDivisions; latIdx++ {
		lat := -90.0 + float64(latIdx)*180.0/float64(latDivisions)
		latRad := lat * math.Pi / 180.0
		
		for lonIdx := 0; lonIdx <= lonDivisions; lonIdx++ {
			lon := -180.0 + float64(lonIdx)*360.0/float64(lonDivisions)
			lonRad := lon * math.Pi / 180.0
			
			// Convert to cartesian
			x := math.Cos(latRad) * math.Cos(lonRad)
			y := math.Sin(latRad)
			z := math.Cos(latRad) * math.Sin(lonRad)
			
			// Find surface voxel
			voxelLat := int((lat + 90.0) * float64(p.Shells[len(p.Shells)-1].LatBands) / 180.0)
			voxelLon := int((lon + 180.0) * 360.0 / 360.0)
			
			if voxelLat >= p.Shells[len(p.Shells)-1].LatBands {
				voxelLat = p.Shells[len(p.Shells)-1].LatBands - 1
			}
			
			surfaceVoxel, shellIdx := p.GetSurfaceVoxel(voxelLat, voxelLon)
			
			// Set height based on material
			height := 1.0
			if surfaceVoxel != nil && surfaceVoxel.Type != MatAir {
				// Small height variation based on material
				switch surfaceVoxel.Type {
				case MatGranite:
					height = 1.02 // Continental
				case MatBasalt:
					height = 0.995 // Ocean floor
				case MatWater:
					height = 1.0
				case MatSediment:
					height = 1.01
				case MatMagma:
					height = 1.025 // Volcanic peaks
				default:
					height = 1.0 + float64(shellIdx-len(p.Shells)+5)*0.001
				}
			}
			
			vertex := Vector3{x * height, y * height, z * height}
			normal := Vector3{x, y, z}
			
			// Color based on material with age variation
			var color Vector3
			if surfaceVoxel != nil {
				baseColor := getMaterialColor(surfaceVoxel)
				// Add age-based color variation to show movement
				// Younger material is brighter, older is darker
				ageModifier := 1.0 - float64(surfaceVoxel.Age) / 500000000.0 // Darken over 500My
				if ageModifier < 0.3 {
					ageModifier = 0.3 // Don't go too dark
				}
				color = Vector3{
					baseColor.X * ageModifier,
					baseColor.Y * ageModifier,
					baseColor.Z * ageModifier,
				}
			} else {
				color = Vector3{0.5, 0.5, 0.5}
			}
			
			vertexMap[[2]int{latIdx, lonIdx}] = len(mesh.Vertices)
			mesh.Vertices = append(mesh.Vertices, vertex)
			mesh.Normals = append(mesh.Normals, normal)
			mesh.Colors = append(mesh.Colors, color)
		}
	}
	
	// Create triangles
	for latIdx := 0; latIdx < latDivisions; latIdx++ {
		for lonIdx := 0; lonIdx < lonDivisions; lonIdx++ {
			// Get the four corners of this quad
			v0 := vertexMap[[2]int{latIdx, lonIdx}]
			v1 := vertexMap[[2]int{latIdx, lonIdx + 1}]
			v2 := vertexMap[[2]int{latIdx + 1, lonIdx + 1}]
			v3 := vertexMap[[2]int{latIdx + 1, lonIdx}]
			
			// Create two triangles (with correct winding)
			mesh.Triangles = append(mesh.Triangles, int32(v0), int32(v1), int32(v2))
			mesh.Triangles = append(mesh.Triangles, int32(v0), int32(v2), int32(v3))
		}
	}
	
	p.SurfaceMesh = mesh
	p.MeshDirty = false
	return mesh
}

// getMaterialColor returns a color for visualization
func getMaterialColor(voxel *VoxelMaterial) Vector3 {
	switch voxel.Type {
	case MatAir:
		return Vector3{0.8, 0.8, 1.0} // Light blue
	case MatWater:
		// Ocean depth coloring
		depth := (voxel.Pressure - 101325) / 1000000.0
		if depth < 0 {
			depth = 0
		}
		blue := 0.3 + 0.4*math.Exp(-float64(depth))
		return Vector3{0.1, 0.3, blue}
	case MatBasalt:
		return Vector3{0.2, 0.2, 0.3} // Dark gray (oceanic crust)
	case MatGranite:
		// Continental elevation coloring
		temp := float64(voxel.Temperature)
		if temp < 273 {
			// Snow/ice
			return Vector3{0.9, 0.9, 1.0}
		} else if temp < 288 {
			// Cold/mountain
			return Vector3{0.6, 0.4, 0.2}
		} else {
			// Temperate land
			return Vector3{0.2, 0.6, 0.2}
		}
	case MatMagma:
		// Hot lava colors
		heat := float64(voxel.Temperature) / 2000.0
		if heat > 1.0 {
			heat = 1.0
		}
		return Vector3{heat, heat * 0.3, 0.0}
	case MatSediment:
		return Vector3{0.7, 0.6, 0.4} // Sandy/tan
	case MatIce:
		return Vector3{0.85, 0.85, 0.95} // Ice white-blue
	case MatPeridotite:
		return Vector3{0.3, 0.5, 0.3} // Deep mantle green
	default:
		return Vector3{0.5, 0.5, 0.5} // Gray
	}
}