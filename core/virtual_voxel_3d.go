package core

import (
	"fmt"
	"math"
)

// VirtualVoxel represents a voxel with continuous spherical position
type VirtualVoxel struct {
	ID       int32
	Position struct {
		R     float32 // Radius from planet center
		Theta float32 // Latitude in radians (-π/2 to π/2)
		Phi   float32 // Longitude in radians (-π to π)
	}
	Velocity struct {
		R     float32 // Radial velocity
		Theta float32 // Latitudinal velocity
		Phi   float32 // Longitudinal velocity
	}
	Mass        float32
	Material    MaterialType
	Temperature float32
	PlateID     int32
	BondOffset  int32                  // Index into bonds array
	BondCount   int32                  // Number of bonds
	GridWeights map[VoxelCoord]float32 // Which grid cells this affects
}

// VoxelBond represents a spring connection between virtual voxels
type VoxelBond struct {
	TargetID    int32   // ID of connected voxel
	RestLength  float32 // Natural separation
	Stiffness   float32 // Spring constant
	Strength    float32 // 0-1, can break under stress
	CurrentDist float32 // Current distance (for stress calculation)
}

// VirtualVoxelSystem manages virtual voxels for a planet
type VirtualVoxelSystem struct {
	VirtualVoxels []VirtualVoxel
	Bonds         []VoxelBond
	VoxelMap      map[int32]*VirtualVoxel
	Planet        *VoxelPlanet
	NextID        int32
	UseGPU        bool // If true, skip CPU physics update
}

// NewVirtualVoxelSystem creates a virtual voxel system for a planet
func NewVirtualVoxelSystem(planet *VoxelPlanet) *VirtualVoxelSystem {
	return &VirtualVoxelSystem{
		Planet:   planet,
		VoxelMap: make(map[int32]*VirtualVoxel),
		NextID:   1,
	}
}

// ConvertToVirtualVoxels converts existing surface voxels to virtual voxels
func (vvs *VirtualVoxelSystem) ConvertToVirtualVoxels() {
	// Start with surface shell
	surfaceShell := len(vvs.Planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vvs.Planet.Shells[surfaceShell]

	// Create virtual voxel for each non-water surface voxel
	for latIdx := range shell.Voxels {
		lat := GetLatitudeForBand(latIdx, shell.LatBands)
		latRad := lat * math.Pi / 180.0

		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]

			// Only convert land masses
			if voxel.Type != MatGranite && voxel.Type != MatBasalt {
				continue
			}

			// TEMPORARY: Only create virtual voxels for every Nth voxel to reduce count
			if (latIdx*len(shell.Voxels[latIdx])+lonIdx)%10 != 0 {
				continue
			}

			lon := float64(lonIdx)/float64(len(shell.Voxels[latIdx]))*360.0 - 180.0
			lonRad := lon * math.Pi / 180.0

			vv := VirtualVoxel{
				ID:          vvs.NextID,
				Mass:        float32(voxel.Density * 1000), // Approximate mass
				Material:    voxel.Type,
				Temperature: voxel.Temperature,
				PlateID:     voxel.PlateID,
				GridWeights: make(map[VoxelCoord]float32),
			}

			// Set position
			vv.Position.R = float32((shell.InnerRadius + shell.OuterRadius) / 2)
			vv.Position.Theta = float32(latRad)
			vv.Position.Phi = float32(lonRad)

			// Copy velocity
			vv.Velocity.R = voxel.VelR
			vv.Velocity.Theta = voxel.VelNorth
			vv.Velocity.Phi = voxel.VelEast

			vvs.VirtualVoxels = append(vvs.VirtualVoxels, vv)
			vvs.VoxelMap[vv.ID] = &vvs.VirtualVoxels[len(vvs.VirtualVoxels)-1]
			vvs.NextID++
		}
	}
}

// CreateBonds creates spring connections between nearby virtual voxels
func (vvs *VirtualVoxelSystem) CreateBonds() {
	fmt.Printf("Creating bonds for %d voxels...\n", len(vvs.VirtualVoxels))

	// Create spatial hash grid for efficient neighbor finding
	// Use ~2 degree cells for the grid
	gridSize := float32(0.035) // radians, about 2 degrees
	spatialGrid := make(map[SpatialKey][]*VirtualVoxel)

	// First pass: insert all voxels into spatial grid
	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]
		key := vvs.getSpatialKey(voxel.Position.Theta, voxel.Position.Phi, gridSize)
		spatialGrid[key] = append(spatialGrid[key], voxel)
	}

	// Second pass: create bonds between nearby voxels
	maxBondDistance := float32(0.052) // ~3 degrees
	maxBondsPerVoxel := 6             // Hexagonal packing

	// Track bond offsets for each voxel
	voxelBonds := make(map[int32][]VoxelBond)

	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]

		// Get neighboring cells
		neighbors := vvs.getNeighborCells(voxel.Position.Theta, voxel.Position.Phi, gridSize, spatialGrid)

		// Find nearby voxels and create bonds
		bondCandidates := make([]BondCandidate, 0)

		for _, neighbor := range neighbors {
			// Skip self and different plates
			if neighbor.ID == voxel.ID || neighbor.PlateID != voxel.PlateID {
				continue
			}

			// Calculate distance
			dist := vvs.angularDistance(voxel, neighbor)
			if dist <= maxBondDistance && dist > 0.001 {
				bondCandidates = append(bondCandidates, BondCandidate{
					Target:   neighbor,
					Distance: dist,
				})
			}
		}

		// Sort by distance and take closest neighbors
		if len(bondCandidates) > maxBondsPerVoxel {
			// Simple selection of closest neighbors
			for i := 0; i < len(bondCandidates)-1; i++ {
				for j := i + 1; j < len(bondCandidates); j++ {
					if bondCandidates[j].Distance < bondCandidates[i].Distance {
						bondCandidates[i], bondCandidates[j] = bondCandidates[j], bondCandidates[i]
					}
				}
			}
			bondCandidates = bondCandidates[:maxBondsPerVoxel]
		}

		// Create bonds
		for _, candidate := range bondCandidates {
			// Check if bond already exists (avoid duplicates)
			exists := false
			for _, existingBond := range voxelBonds[candidate.Target.ID] {
				if existingBond.TargetID == voxel.ID {
					exists = true
					break
				}
			}

			if !exists {
				bond := VoxelBond{
					TargetID:   candidate.Target.ID,
					RestLength: candidate.Distance,
					Stiffness:  100.0, // Reduced from 1000 for less rigid connections
					Strength:   0.9,
				}
				voxelBonds[voxel.ID] = append(voxelBonds[voxel.ID], bond)
			}
		}
	}

	// Third pass: consolidate bonds into array and set offsets
	vvs.Bonds = make([]VoxelBond, 0)
	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]
		bonds := voxelBonds[voxel.ID]

		if len(bonds) > 0 {
			voxel.BondOffset = int32(len(vvs.Bonds))
			voxel.BondCount = int32(len(bonds))
			vvs.Bonds = append(vvs.Bonds, bonds...)
		} else {
			voxel.BondOffset = 0
			voxel.BondCount = 0
		}
	}

	fmt.Printf("Created %d bonds using spatial hashing (avg %.2f per voxel)\n",
		len(vvs.Bonds), float64(len(vvs.Bonds))/float64(len(vvs.VirtualVoxels)))
}

// SpatialKey represents a cell in the spatial grid
type SpatialKey struct {
	ThetaCell int32
	PhiCell   int32
}

// BondCandidate holds a potential bond target and its distance
type BondCandidate struct {
	Target   *VirtualVoxel
	Distance float32
}

// getSpatialKey returns the grid cell for a given position
func (vvs *VirtualVoxelSystem) getSpatialKey(theta, phi, gridSize float32) SpatialKey {
	return SpatialKey{
		ThetaCell: int32((theta + math.Pi/2) / gridSize),
		PhiCell:   int32((phi + math.Pi) / gridSize),
	}
}

// getNeighborCells returns all voxels in neighboring grid cells
func (vvs *VirtualVoxelSystem) getNeighborCells(theta, phi, gridSize float32,
	spatialGrid map[SpatialKey][]*VirtualVoxel) []*VirtualVoxel {

	neighbors := make([]*VirtualVoxel, 0)
	centerKey := vvs.getSpatialKey(theta, phi, gridSize)

	// Check 3x3 grid of cells
	for dTheta := int32(-1); dTheta <= 1; dTheta++ {
		for dPhi := int32(-1); dPhi <= 1; dPhi++ {
			key := SpatialKey{
				ThetaCell: centerKey.ThetaCell + dTheta,
				PhiCell:   centerKey.PhiCell + dPhi,
			}

			if voxels, ok := spatialGrid[key]; ok {
				neighbors = append(neighbors, voxels...)
			}
		}
	}

	return neighbors
}

// angularDistance calculates distance between two virtual voxels on sphere
func (vvs *VirtualVoxelSystem) angularDistance(v1, v2 *VirtualVoxel) float32 {
	// Haversine formula
	dTheta := v2.Position.Theta - v1.Position.Theta
	dPhi := v2.Position.Phi - v1.Position.Phi

	a := float32(math.Sin(float64(dTheta/2))*math.Sin(float64(dTheta/2)) +
		math.Cos(float64(v1.Position.Theta))*math.Cos(float64(v2.Position.Theta))*
			math.Sin(float64(dPhi/2))*math.Sin(float64(dPhi/2)))

	c := 2 * float32(math.Atan2(math.Sqrt(float64(a)), math.Sqrt(float64(1-a))))

	return c
}

// UpdatePhysics performs one physics timestep
func (vvs *VirtualVoxelSystem) UpdatePhysics(dt float32) {
	// Skip if GPU physics is being used
	if vvs.UseGPU {
		return
	}

	// This would be implemented as GPU compute shaders
	// For now, simplified CPU version

	// Calculate spring forces
	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]

		// Process bonds
		for b := int32(0); b < voxel.BondCount; b++ {
			bond := &vvs.Bonds[voxel.BondOffset+b]
			target := vvs.VoxelMap[bond.TargetID]
			if target == nil {
				continue
			}

			// Calculate spring force (simplified)
			dist := vvs.angularDistance(voxel, target)
			bond.CurrentDist = dist

			// Apply forces based on spring physics
			// This is where smooth movement happens!
		}
	}

	// Update positions
	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]

		// Update position based on velocity
		voxel.Position.Theta += voxel.Velocity.Theta * dt
		voxel.Position.Phi += voxel.Velocity.Phi * dt

		// Wrap longitude
		if voxel.Position.Phi > math.Pi {
			voxel.Position.Phi -= 2 * math.Pi
		} else if voxel.Position.Phi < -math.Pi {
			voxel.Position.Phi += 2 * math.Pi
		}

		// Clamp latitude
		if voxel.Position.Theta > math.Pi/2 {
			voxel.Position.Theta = math.Pi / 2
		} else if voxel.Position.Theta < -math.Pi/2 {
			voxel.Position.Theta = -math.Pi / 2
		}
	}
}

// MapToGrid updates the regular voxel grid based on virtual voxel positions
func (vvs *VirtualVoxelSystem) MapToGrid() {
	surfaceShell := len(vvs.Planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &vvs.Planet.Shells[surfaceShell]

	// Clear surface to water
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			if voxel.Type == MatGranite || voxel.Type == MatBasalt {
				voxel.Type = MatWater
				voxel.Density = MaterialProperties[MatWater].DefaultDensity
				voxel.PlateID = 0
			}
		}
	}

	// Map virtual voxels to grid with interpolation
	for i := range vvs.VirtualVoxels {
		voxel := &vvs.VirtualVoxels[i]

		// Convert spherical position to grid indices
		lat := voxel.Position.Theta * 180.0 / math.Pi
		lon := voxel.Position.Phi * 180.0 / math.Pi

		// Find affected grid cells (bilinear interpolation)
		latIdx := (lat + 90.0) / 180.0 * float32(shell.LatBands)
		lat0 := int(latIdx)
		lat1 := lat0 + 1
		latFrac := latIdx - float32(lat0)

		if lat0 >= 0 && lat0 < shell.LatBands {
			lonCount := float32(len(shell.Voxels[lat0]))
			lonIdx := (lon + 180.0) / 360.0 * lonCount
			lon0 := int(lonIdx)
			lon1 := (lon0 + 1) % int(lonCount)
			lonFrac := lonIdx - float32(lon0)

			// Update grid cells with weights
			cells := []struct {
				lat, lon int
				weight   float32
			}{
				{lat0, lon0, (1 - latFrac) * (1 - lonFrac)},
				{lat0, lon1, (1 - latFrac) * lonFrac},
			}

			if lat1 < shell.LatBands {
				lonCount1 := float32(len(shell.Voxels[lat1]))
				lonIdx1 := (lon + 180.0) / 360.0 * lonCount1
				lon0_1 := int(lonIdx1)
				lon1_1 := (lon0_1 + 1) % int(lonCount1)
				lonFrac1 := lonIdx1 - float32(lon0_1)

				cells = append(cells,
					struct {
						lat, lon int
						weight   float32
					}{lat1, lon0_1, latFrac * (1 - lonFrac1)},
					struct {
						lat, lon int
						weight   float32
					}{lat1, lon1_1, latFrac * lonFrac1},
				)
			}

			// Apply to grid
			for _, cell := range cells {
				if cell.weight > 0.1 { // Threshold to avoid too much spreading
					gridVoxel := &shell.Voxels[cell.lat][cell.lon]
					gridVoxel.Type = voxel.Material
					gridVoxel.Density = MaterialProperties[voxel.Material].DefaultDensity
					gridVoxel.Temperature = voxel.Temperature
					gridVoxel.PlateID = voxel.PlateID

					// Store weight for later use
					coord := VoxelCoord{Shell: surfaceShell, Lat: cell.lat, Lon: cell.lon}
					voxel.GridWeights[coord] = cell.weight
				}
			}
		}
	}

	// Mark as dirty for rendering
	vvs.Planet.MeshDirty = true
}
