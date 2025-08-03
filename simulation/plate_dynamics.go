package simulation

import (
	"fmt"
	"math"
	"worldgenerator/core"
)

// PlateBoundaryType represents the type of interaction between plates
type PlateBoundaryType int

const (
	BoundaryNone PlateBoundaryType = iota
	BoundaryDivergent  // Plates moving apart
	BoundaryConvergent // Plates colliding
	BoundaryTransform  // Plates sliding past
)

// PlateBoundary represents the interaction between two plates
type PlateBoundary struct {
	PlateA, PlateB int              // Plate IDs
	Type           PlateBoundaryType
	RelativeVel    core.Vector3    // Relative velocity at boundary
	StressAccum    float64         // Accumulated stress
	BoundaryVoxels []core.VoxelCoord // Voxels along this boundary
}

// PlateForceCalculator handles sophisticated plate dynamics
type PlateForceCalculator struct {
	planet     *core.VoxelPlanet
	manager    *PlateManager
	boundaries map[string]*PlateBoundary // Key: "plateA_plateB" (lower ID first)
}

// NewPlateForceCalculator creates a new force calculator
func NewPlateForceCalculator(planet *core.VoxelPlanet, manager *PlateManager) *PlateForceCalculator {
	return &PlateForceCalculator{
		planet:     planet,
		manager:    manager,
		boundaries: make(map[string]*PlateBoundary),
	}
}

// getPlateByID finds a plate by its ID
func (pfc *PlateForceCalculator) getPlateByID(id int) *TectonicPlate {
	for _, plate := range pfc.manager.Plates {
		if plate.ID == id {
			return plate
		}
	}
	return nil
}

// CalculatePlateForces computes all forces acting on plates
func (pfc *PlateForceCalculator) CalculatePlateForces(dt float64) {
	// First, identify and classify boundaries
	pfc.identifyPlateBoundaries()
	
	// Then calculate forces for each plate
	for _, plate := range pfc.manager.Plates {
		pfc.calculateRidgePush(plate)
		pfc.calculateSlabPull(plate)
		pfc.calculateBasalDrag(plate)
		pfc.calculateBoundaryForces(plate)
	}
}

// identifyPlateBoundaries finds and classifies all plate boundaries
func (pfc *PlateForceCalculator) identifyPlateBoundaries() {
	// Clear existing boundaries
	pfc.boundaries = make(map[string]*PlateBoundary)
	
	// Check all boundary voxels
	for plateID, plate := range pfc.manager.Plates {
		for _, coord := range plate.BoundaryVoxels {
			// Check neighbors
			neighbors := pfc.getNeighborCoords(coord)
			
			for _, neighborCoord := range neighbors {
				neighborPlateID, exists := pfc.manager.VoxelPlateMap[neighborCoord]
				if !exists || neighborPlateID == plateID {
					continue
				}
				
				// Create boundary key (lower ID first)
				var key string
				if plateID < neighborPlateID {
					key = fmt.Sprintf("%d_%d", plateID, neighborPlateID)
				} else {
					key = fmt.Sprintf("%d_%d", neighborPlateID, plateID)
				}
				
				// Add to boundary
				boundary, exists := pfc.boundaries[key]
				if !exists {
					boundary = &PlateBoundary{
						PlateA: min(plateID, neighborPlateID),
						PlateB: max(plateID, neighborPlateID),
						BoundaryVoxels: []core.VoxelCoord{},
					}
					pfc.boundaries[key] = boundary
				}
				
				// Add this voxel to boundary
				boundary.BoundaryVoxels = append(boundary.BoundaryVoxels, coord)
			}
		}
	}
	
	// Classify each boundary based on relative motion
	for _, boundary := range pfc.boundaries {
		pfc.classifyBoundary(boundary)
	}
}

// getNeighborCoords returns coordinates of neighboring voxels
func (pfc *PlateForceCalculator) getNeighborCoords(coord core.VoxelCoord) []core.VoxelCoord {
	var neighbors []core.VoxelCoord
	shell := &pfc.planet.Shells[coord.Shell]
	
	// North neighbor
	if coord.Lat > 0 {
		neighbors = append(neighbors, core.VoxelCoord{
			Shell: coord.Shell,
			Lat:   coord.Lat - 1,
			Lon:   pfc.adjustLonForLatitude(coord.Lon, coord.Lat, coord.Lat-1, shell),
		})
	}
	
	// South neighbor
	if coord.Lat < shell.LatBands-1 {
		neighbors = append(neighbors, core.VoxelCoord{
			Shell: coord.Shell,
			Lat:   coord.Lat + 1,
			Lon:   pfc.adjustLonForLatitude(coord.Lon, coord.Lat, coord.Lat+1, shell),
		})
	}
	
	// East neighbor
	lonCount := len(shell.Voxels[coord.Lat])
	neighbors = append(neighbors, core.VoxelCoord{
		Shell: coord.Shell,
		Lat:   coord.Lat,
		Lon:   (coord.Lon + 1) % lonCount,
	})
	
	// West neighbor
	westLon := coord.Lon - 1
	if westLon < 0 {
		westLon = lonCount - 1
	}
	neighbors = append(neighbors, core.VoxelCoord{
		Shell: coord.Shell,
		Lat:   coord.Lat,
		Lon:   westLon,
	})
	
	return neighbors
}

// adjustLonForLatitude adjusts longitude index when moving between latitude bands
func (pfc *PlateForceCalculator) adjustLonForLatitude(oldLon, oldLat, newLat int, shell *core.SphericalShell) int {
	oldLonCount := len(shell.Voxels[oldLat])
	newLonCount := len(shell.Voxels[newLat])
	
	// Scale longitude to maintain approximate position
	lonFraction := float64(oldLon) / float64(oldLonCount)
	newLon := int(lonFraction * float64(newLonCount))
	
	if newLon >= newLonCount {
		newLon = newLonCount - 1
	}
	
	return newLon
}

// classifyBoundary determines the type of plate boundary
func (pfc *PlateForceCalculator) classifyBoundary(boundary *PlateBoundary) {
	// Find plates by ID
	plateA := pfc.getPlateByID(boundary.PlateA)
	plateB := pfc.getPlateByID(boundary.PlateB)
	
	// Skip if we can't find both plates
	if plateA == nil || plateB == nil {
		boundary.Type = BoundaryNone
		return
	}
	
	// Calculate average relative velocity at boundary
	var relVel core.Vector3
	count := 0
	
	for _, coord := range boundary.BoundaryVoxels {
		// Get velocities from plate motion
		velA := pfc.getPlateVelocityAt(plateA, coord)
		velB := pfc.getPlateVelocityAt(plateB, coord)
		
		relVel.X += velB.X - velA.X
		relVel.Y += velB.Y - velA.Y
		relVel.Z += velB.Z - velA.Z
		count++
	}
	
	if count > 0 {
		relVel.X /= float64(count)
		relVel.Y /= float64(count)
		relVel.Z /= float64(count)
	}
	
	boundary.RelativeVel = relVel
	
	// Classify based on relative motion
	// For now, use simplified classification
	// TODO: Implement proper vector analysis for boundary classification
	
	speed := relVel.Length()
	if speed < 0.01 { // Very slow relative motion
		boundary.Type = BoundaryTransform
	} else {
		// Check if plates are moving apart or together
		// This is simplified - real implementation would consider boundary orientation
		if relVel.X*relVel.X + relVel.Y*relVel.Y > relVel.Z*relVel.Z {
			// Horizontal motion dominates
			boundary.Type = BoundaryTransform
		} else if relVel.Z > 0 {
			boundary.Type = BoundaryDivergent
		} else {
			boundary.Type = BoundaryConvergent
		}
	}
}

// getPlateVelocityAt calculates plate velocity at a specific location
func (pfc *PlateForceCalculator) getPlateVelocityAt(plate *TectonicPlate, coord core.VoxelCoord) core.Vector3 {
	shell := &pfc.planet.Shells[coord.Shell]
	
	// Convert to geographic coordinates
	lat := core.GetLatitudeForBand(coord.Lat, shell.LatBands) * math.Pi / 180.0
	lon := float64(coord.Lon) * 2.0 * math.Pi / float64(len(shell.Voxels[coord.Lat]))
	
	// Euler pole in radians
	poleLat := plate.EulerPoleLat * math.Pi / 180.0
	poleLon := plate.EulerPoleLon * math.Pi / 180.0
	
	// Calculate velocity from rigid body rotation
	// v = ω × r
	omega := plate.AngularVelocity
	
	// Convert to Cartesian for cross product
	radius := (shell.InnerRadius + shell.OuterRadius) / 2.0
	
	// Position vector
	px := radius * math.Cos(lat) * math.Cos(lon)
	py := radius * math.Cos(lat) * math.Sin(lon)
	pz := radius * math.Sin(lat)
	
	// Rotation axis (Euler pole)
	ax := math.Cos(poleLat) * math.Cos(poleLon)
	ay := math.Cos(poleLat) * math.Sin(poleLon)
	az := math.Sin(poleLat)
	
	// Cross product: v = ω * (axis × position)
	vx := omega * (ay*pz - az*py)
	vy := omega * (az*px - ax*pz)
	vz := omega * (ax*py - ay*px)
	
	return core.Vector3{X: vx, Y: vy, Z: vz}
}

// calculateRidgePush computes ridge push force
func (pfc *PlateForceCalculator) calculateRidgePush(plate *TectonicPlate) {
	plate.RidgePushForce = core.Vector3{}
	
	// Ridge push acts at divergent boundaries
	for _, boundary := range pfc.boundaries {
		if boundary.Type != BoundaryDivergent {
			continue
		}
		
		// Check if this plate is involved
		if boundary.PlateA != plate.ID && boundary.PlateB != plate.ID {
			continue
		}
		
		// Ridge push force depends on:
		// - Age contrast across ridge
		// - Thermal structure
		// - Elevation difference
		
		for _, coord := range boundary.BoundaryVoxels {
			shell := &pfc.planet.Shells[coord.Shell]
			voxel := &shell.Voxels[coord.Lat][coord.Lon]
			
			if voxel.Type == core.MatBasalt && voxel.Age < 10000000 { // Young oceanic crust
				// Simple ridge push model
				// Force proportional to elevation and age
				forceMagnitude := 3e12 * (1.0 - float64(voxel.Age)/50000000.0) // N/m
				
				// Direction: away from ridge (use boundary normal)
				// Simplified: use relative velocity direction
				normal := boundary.RelativeVel.Normalize()
				
				plate.RidgePushForce.X += forceMagnitude * normal.X
				plate.RidgePushForce.Y += forceMagnitude * normal.Y
				plate.RidgePushForce.Z += forceMagnitude * normal.Z
			}
		}
	}
}

// calculateSlabPull computes slab pull force
func (pfc *PlateForceCalculator) calculateSlabPull(plate *TectonicPlate) {
	plate.SlabPullForce = core.Vector3{}
	
	// Slab pull acts at convergent boundaries where oceanic plate subducts
	for _, boundary := range pfc.boundaries {
		if boundary.Type != BoundaryConvergent {
			continue
		}
		
		// Check if this plate is involved and is oceanic
		if (boundary.PlateA != plate.ID && boundary.PlateB != plate.ID) || plate.Type != "oceanic" {
			continue
		}
		
		// Slab pull force depends on:
		// - Slab length and thickness
		// - Density contrast
		// - Slab dip angle
		
		for _, coord := range boundary.BoundaryVoxels {
			shell := &pfc.planet.Shells[coord.Shell]
			voxel := &shell.Voxels[coord.Lat][coord.Lon]
			
			if voxel.Type == core.MatBasalt && voxel.Age > 20000000 { // Old oceanic crust
				// Simple slab pull model
				// Force proportional to age (proxy for density)
				forceMagnitude := 5e12 * (float64(voxel.Age) / 100000000.0) // N/m
				
				// Direction: downward and in direction of subduction
				// Simplified: use -Z direction with some horizontal component
				plate.SlabPullForce.Z -= forceMagnitude * 0.8
				
				// Add horizontal component based on plate motion
				vel := pfc.getPlateVelocityAt(plate, coord)
				if vel.Length() > 0 {
					velNorm := vel.Normalize()
					plate.SlabPullForce.X += forceMagnitude * 0.2 * velNorm.X
					plate.SlabPullForce.Y += forceMagnitude * 0.2 * velNorm.Y
				}
			}
		}
	}
}

// calculateBasalDrag computes mantle drag force
func (pfc *PlateForceCalculator) calculateBasalDrag(plate *TectonicPlate) {
	// Basal drag opposes plate motion
	// F = -μ * A * v
	
	// Get average plate velocity
	avgVel := core.Vector3{}
	count := 0
	
	for _, coord := range plate.MemberVoxels {
		vel := pfc.getPlateVelocityAt(plate, coord)
		avgVel = avgVel.Add(vel)
		count++
	}
	
	if count > 0 {
		avgVel.X /= float64(count)
		avgVel.Y /= float64(count)
		avgVel.Z /= float64(count)
	}
	
	// Drag coefficient depends on mantle viscosity and lithosphere thickness
	// Typical values: 1e6 to 1e7 Pa·s/m
	dragCoeff := 5e6 // Pa·s/m
	
	// Force = -drag_coefficient * area * velocity
	plate.BasalDragForce = core.Vector3{
		X: -dragCoeff * plate.TotalArea * avgVel.X,
		Y: -dragCoeff * plate.TotalArea * avgVel.Y,
		Z: -dragCoeff * plate.TotalArea * avgVel.Z,
	}
}

// calculateBoundaryForces computes collision and transform forces
func (pfc *PlateForceCalculator) calculateBoundaryForces(plate *TectonicPlate) {
	plate.CollisionForce = core.Vector3{}
	
	for _, boundary := range pfc.boundaries {
		// Check if this plate is involved
		if boundary.PlateA != plate.ID && boundary.PlateB != plate.ID {
			continue
		}
		
		switch boundary.Type {
		case BoundaryConvergent:
			// Collision force opposes convergence
			// Stronger for continental-continental collision
			otherPlateID := boundary.PlateA
			if otherPlateID == plate.ID {
				otherPlateID = boundary.PlateB
			}
			
			// Find the other plate by ID
			otherPlate := pfc.getPlateByID(otherPlateID)
			
			// Check if continental collision
			if otherPlate != nil && plate.Type == "continental" && otherPlate.Type == "continental" {
				// Strong collision force
				forceMagnitude := 1e13 * float64(len(boundary.BoundaryVoxels))
				
				// Direction: oppose relative motion
				if boundary.RelativeVel.Length() > 0 {
					direction := boundary.RelativeVel.Normalize()
					plate.CollisionForce.X -= forceMagnitude * direction.X
					plate.CollisionForce.Y -= forceMagnitude * direction.Y
					plate.CollisionForce.Z -= forceMagnitude * direction.Z
				}
			}
			
		case BoundaryTransform:
			// Transform boundaries accumulate shear stress
			// Add small resisting force
			if boundary.RelativeVel.Length() > 0 {
				forceMagnitude := 1e11 * float64(len(boundary.BoundaryVoxels))
				direction := boundary.RelativeVel.Normalize()
				
				// Resist lateral motion
				plate.CollisionForce.X -= forceMagnitude * direction.X * 0.1
				plate.CollisionForce.Y -= forceMagnitude * direction.Y * 0.1
			}
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}