package simulation

import (
	"fmt"
	"math"

	"worldgenerator/core"
)

// TectonicPlate represents a coherent lithospheric plate
type TectonicPlate struct {
	ID   int
	Name string
	Type string // "continental", "oceanic", "mixed"

	// Plate motion (rigid body)
	EulerPoleLat    float64 // Rotation pole latitude
	EulerPoleLon    float64 // Rotation pole longitude
	AngularVelocity float64 // Radians per year

	// Plate properties
	AverageAge       float64
	AverageThickness float64
	TotalArea        float64
	TotalMass        float64

	// Member voxels (surface shell indices)
	MemberVoxels   []core.VoxelCoord
	BoundaryVoxels []core.VoxelCoord // Edge voxels

	// Forces acting on plate
	RidgePushForce core.Vector3
	SlabPullForce  core.Vector3
	BasalDragForce core.Vector3
	CollisionForce core.Vector3

	// Deformation tracking
	StrainRate  float64
	Convergence float64 // Net convergence/divergence
}

// PlateManager handles plate identification and motion
type PlateManager struct {
	Plates        []*TectonicPlate
	VoxelPlateMap map[core.VoxelCoord]int  // Which plate each voxel belongs to
	BoundaryMap   map[core.VoxelCoord]bool // Quick lookup for boundary voxels
	planet        *core.VoxelPlanet
	nextPlateID   int
	
	// Advanced plate dynamics
	forceCalculator *PlateForceCalculator
}

// NewPlateManager creates a plate manager
func NewPlateManager(planet *core.VoxelPlanet) *PlateManager {
	return &PlateManager{
		planet:        planet,
		VoxelPlateMap: make(map[core.VoxelCoord]int),
		BoundaryMap:   make(map[core.VoxelCoord]bool),
		nextPlateID:   1,
	}
}

// Implement core.PlateManagerInterface

// GetVoxelPlateID returns the plate ID for a given voxel coordinate
func (pm *PlateManager) GetVoxelPlateID(coord core.VoxelCoord) (int, bool) {
	plateID, exists := pm.VoxelPlateMap[coord]
	return plateID, exists
}

// IsBoundaryVoxel checks if a voxel is on a plate boundary
func (pm *PlateManager) IsBoundaryVoxel(coord core.VoxelCoord) bool {
	return pm.BoundaryMap[coord]
}

// GetPlateCount returns the number of identified plates
func (pm *PlateManager) GetPlateCount() int {
	return len(pm.Plates)
}

// UpdatePlates recalculates plate boundaries and properties
func (pm *PlateManager) UpdatePlates() {
	pm.IdentifyPlates()
}

// GetPlateBoundaries returns information about plate boundaries for visualization
func (pm *PlateManager) GetPlateBoundaries() []*PlateBoundary {
	if pm.forceCalculator == nil {
		return nil
	}
	
	var boundaries []*PlateBoundary
	for _, boundary := range pm.forceCalculator.boundaries {
		boundaries = append(boundaries, boundary)
	}
	return boundaries
}

// IdentifyPlates segments the lithosphere into discrete plates
func (pm *PlateManager) IdentifyPlates() {
	// Clear existing plates
	pm.Plates = nil
	pm.VoxelPlateMap = make(map[core.VoxelCoord]int)
	pm.BoundaryMap = make(map[core.VoxelCoord]bool)

	// Work with surface shell (lithosphere)
	surfaceShell := len(pm.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}

	shell := &pm.planet.Shells[surfaceShell]
	visited := make(map[core.VoxelCoord]bool)

	// Flood fill to identify connected regions with similar velocities
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			coord := core.VoxelCoord{Shell: surfaceShell, Lat: latIdx, Lon: lonIdx}

			// Skip if already assigned to a plate
			if visited[coord] {
				continue
			}

			voxel := &shell.Voxels[latIdx][lonIdx]

			// Skip ocean/air
			if voxel.Type == core.MatAir || voxel.Type == core.MatWater {
				visited[coord] = true
				continue
			}

			// Only process brittle lithosphere
			if !voxel.IsBrittle {
				continue
			}

			// Start a new plate from this seed
			plate := pm.createPlateFromSeed(coord, visited)
			if len(plate.MemberVoxels) > 100 { // Minimum size threshold
				pm.Plates = append(pm.Plates, plate)
			}
		}
	}

	// Calculate plate properties
	for _, plate := range pm.Plates {
		pm.calculatePlateProperties(plate)
		pm.identifyPlateBoundaries(plate)
	}
}

// createPlateFromSeed grows a plate from a seed voxel using flood fill
func (pm *PlateManager) createPlateFromSeed(seed core.VoxelCoord, visited map[core.VoxelCoord]bool) *TectonicPlate {
	plate := &TectonicPlate{
		ID:   pm.nextPlateID,
		Name: fmt.Sprintf("Plate_%d", pm.nextPlateID),
	}
	pm.nextPlateID++

	shell := &pm.planet.Shells[seed.Shell]
	seedVoxel := &shell.Voxels[seed.Lat][seed.Lon]

	// Reference velocity for this plate
	refVelNorth := seedVoxel.VelNorth
	refVelEast := seedVoxel.VelEast

	// Flood fill queue
	queue := []core.VoxelCoord{seed}
	visited[seed] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Add to plate
		plate.MemberVoxels = append(plate.MemberVoxels, current)
		pm.VoxelPlateMap[current] = plate.ID

		// Set PlateID in the actual voxel
		if current.Shell < len(pm.planet.Shells) &&
			current.Lat < len(pm.planet.Shells[current.Shell].Voxels) &&
			current.Lon < len(pm.planet.Shells[current.Shell].Voxels[current.Lat]) {
			pm.planet.Shells[current.Shell].Voxels[current.Lat][current.Lon].PlateID = int32(plate.ID)
		}

		// Check neighbors
		neighbors := pm.getNeighborCoords(current)
		for _, neighbor := range neighbors {
			if visited[neighbor] {
				continue
			}

			// Check if neighbor should be part of this plate
			if pm.shouldBelongToPlate(neighbor, refVelNorth, refVelEast) {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return plate
}

// shouldBelongToPlate checks if a voxel has similar velocity to the plate
func (pm *PlateManager) shouldBelongToPlate(coord core.VoxelCoord, refVelNorth, refVelEast float32) bool {
	if coord.Shell >= len(pm.planet.Shells) {
		return false
	}

	shell := &pm.planet.Shells[coord.Shell]
	if coord.Lat >= len(shell.Voxels) || coord.Lon >= len(shell.Voxels[coord.Lat]) {
		return false
	}

	voxel := &shell.Voxels[coord.Lat][coord.Lon]

	// Must be solid lithosphere
	if voxel.Type == core.MatAir || voxel.Type == core.MatWater || !voxel.IsBrittle {
		return false
	}

	// Check velocity similarity (within threshold)
	velThreshold := float32(1e-6) // m/s
	thetaDiff := float32(math.Abs(float64(voxel.VelNorth - refVelNorth)))
	phiDiff := float32(math.Abs(float64(voxel.VelEast - refVelEast)))

	return thetaDiff < velThreshold && phiDiff < velThreshold
}

// getNeighborCoords returns the coordinates of neighboring voxels
func (pm *PlateManager) getNeighborCoords(coord core.VoxelCoord) []core.VoxelCoord {
	var neighbors []core.VoxelCoord
	shell := &pm.planet.Shells[coord.Shell]

	// East/West neighbors
	lonCount := len(shell.Voxels[coord.Lat])
	eastLon := (coord.Lon + 1) % lonCount
	westLon := (coord.Lon - 1 + lonCount) % lonCount

	neighbors = append(neighbors,
		core.VoxelCoord{Shell: coord.Shell, Lat: coord.Lat, Lon: eastLon},
		core.VoxelCoord{Shell: coord.Shell, Lat: coord.Lat, Lon: westLon},
	)

	// North/South neighbors (if they exist)
	if coord.Lat > 0 {
		neighbors = append(neighbors, core.VoxelCoord{Shell: coord.Shell, Lat: coord.Lat - 1, Lon: coord.Lon})
	}
	if coord.Lat < len(shell.Voxels)-1 {
		neighbors = append(neighbors, core.VoxelCoord{Shell: coord.Shell, Lat: coord.Lat + 1, Lon: coord.Lon})
	}

	return neighbors
}

// calculatePlateProperties computes aggregate properties
func (pm *PlateManager) calculatePlateProperties(plate *TectonicPlate) {
	if len(plate.MemberVoxels) == 0 {
		return
	}

	totalAge := float64(0)
	totalThickness := float64(0)
	continentalCount := 0
	oceanicCount := 0

	// Center of mass for Euler pole calculation
	var centerLat, centerLon float64

	for _, coord := range plate.MemberVoxels {
		shell := &pm.planet.Shells[coord.Shell]
		voxel := &shell.Voxels[coord.Lat][coord.Lon]

		// Age and thickness
		totalAge += float64(voxel.Age)

		// Lithosphere thickness at this point
		// TODO: Fix this when physics integration is complete
		// mechanics := pm.planet.Physics.(*physics.VoxelPhysics).mechanics
		// For now, use a default thickness
		thickness := 100000.0 // 100 km default lithosphere thickness
		totalThickness += thickness

		// Plate type
		if voxel.Type == core.MatGranite {
			continentalCount++
		} else if voxel.Type == core.MatBasalt {
			oceanicCount++
		}

		// Position for center calculation
		lat := core.GetLatitudeForBand(coord.Lat, shell.LatBands)
		lon := float64(coord.Lon)*360.0/float64(len(shell.Voxels[coord.Lat])) - 180.0
		centerLat += lat
		centerLon += lon
	}

	// Average properties
	n := float64(len(plate.MemberVoxels))
	plate.AverageAge = totalAge / n
	plate.AverageThickness = totalThickness / n

	// Plate type
	if continentalCount > oceanicCount*2 {
		plate.Type = "continental"
	} else if oceanicCount > continentalCount*2 {
		plate.Type = "oceanic"
	} else {
		plate.Type = "mixed"
	}

	// Initial Euler pole at center of plate
	plate.EulerPoleLat = centerLat / n
	plate.EulerPoleLon = centerLon / n
}

// identifyPlateBoundaries finds edge voxels
func (pm *PlateManager) identifyPlateBoundaries(plate *TectonicPlate) {
	plate.BoundaryVoxels = nil

	for _, coord := range plate.MemberVoxels {
		neighbors := pm.getNeighborCoords(coord)

		// Check if any neighbor belongs to a different plate
		isBoundary := false
		for _, neighbor := range neighbors {
			neighborPlateID, exists := pm.VoxelPlateMap[neighbor]
			if !exists || neighborPlateID != plate.ID {
				isBoundary = true
				break
			}
		}

		if isBoundary {
			plate.BoundaryVoxels = append(plate.BoundaryVoxels, coord)
			pm.BoundaryMap[coord] = true
		}
	}
}

// UpdatePlateMotion calculates and applies plate-wide motion
func (pm *PlateManager) UpdatePlateMotion(dt float64) {
	// Initialize force calculator if needed
	if pm.forceCalculator == nil {
		pm.forceCalculator = NewPlateForceCalculator(pm.planet, pm)
	}
	
	// Use advanced force calculation
	pm.forceCalculator.CalculatePlateForces(dt)
	
	// Then update plate motion based on forces
	for _, plate := range pm.Plates {
		pm.updatePlateVelocity(plate, dt)
	}

	// Finally, apply plate motion to member voxels
	for _, plate := range pm.Plates {
		pm.applyPlateMotion(plate)
	}
}

// calculatePlateForces is now handled by PlateForceCalculator
// Kept for backward compatibility - delegates to force calculator

// updatePlateVelocity updates the plate's Euler pole rotation
func (pm *PlateManager) updatePlateVelocity(plate *TectonicPlate, dt float64) {
	// Total force
	totalForce := core.Vector3{
		X: plate.RidgePushForce.X + plate.SlabPullForce.X + plate.BasalDragForce.X + plate.CollisionForce.X,
		Y: plate.RidgePushForce.Y + plate.SlabPullForce.Y + plate.BasalDragForce.Y + plate.CollisionForce.Y,
		Z: plate.RidgePushForce.Z + plate.SlabPullForce.Z + plate.BasalDragForce.Z + plate.CollisionForce.Z,
	}

	// Convert force to angular velocity change
	// Simplified - in reality this involves moment of inertia tensor
	effectiveMass := plate.TotalMass
	if effectiveMass == 0 {
		effectiveMass = float64(len(plate.MemberVoxels)) * 1e15 // Approximate
	}

	// Angular acceleration
	angularAccel := math.Sqrt(totalForce.X*totalForce.X+totalForce.Y*totalForce.Y) / (effectiveMass * pm.planet.Radius)

	// Update angular velocity
	plate.AngularVelocity += angularAccel * dt

	// Limit to realistic plate velocities (max ~20 cm/year at surface)
	maxAngularVel := 0.2 / pm.planet.Radius // 20 cm/year
	if math.Abs(plate.AngularVelocity) > maxAngularVel {
		plate.AngularVelocity = maxAngularVel * math.Abs(plate.AngularVelocity) / plate.AngularVelocity
	}

	// Update Euler pole position based on torques
	// Simplified - in reality poles migrate slowly
}

// GetPlateVelocities returns angular velocities for all plates
func (pm *PlateManager) GetPlateVelocities() map[int32][3]float32 {
	velocities := make(map[int32][3]float32)

	for _, plate := range pm.Plates {
		// Convert Euler pole to radians
		poleLat := plate.EulerPoleLat * math.Pi / 180.0
		poleLon := plate.EulerPoleLon * math.Pi / 180.0

		// Euler pole unit vector * angular velocity
		poleX := float32(math.Cos(poleLat) * math.Cos(poleLon) * plate.AngularVelocity)
		poleY := float32(math.Cos(poleLat) * math.Sin(poleLon) * plate.AngularVelocity)
		poleZ := float32(math.Sin(poleLat) * plate.AngularVelocity)

		velocities[int32(plate.ID)] = [3]float32{poleX, poleY, poleZ}
	}

	return velocities
}

// applyPlateMotion applies rigid body rotation to all voxels in the plate
func (pm *PlateManager) applyPlateMotion(plate *TectonicPlate) {
	// Convert Euler pole to radians
	poleLat := plate.EulerPoleLat * math.Pi / 180.0
	poleLon := plate.EulerPoleLon * math.Pi / 180.0

	// Euler pole unit vector
	poleX := math.Cos(poleLat) * math.Cos(poleLon)
	poleY := math.Cos(poleLat) * math.Sin(poleLon)
	poleZ := math.Sin(poleLat)

	for _, coord := range plate.MemberVoxels {
		shell := &pm.planet.Shells[coord.Shell]
		voxel := &shell.Voxels[coord.Lat][coord.Lon]

		// Skip boundary voxels (they deform instead of rigid motion)
		isBoundary := false
		for _, boundary := range plate.BoundaryVoxels {
			if boundary == coord {
				isBoundary = true
				break
			}
		}

		if isBoundary {
			// Boundary voxels experience deformation
			// Their velocities are set by local forces
			continue
		}

		// Get voxel position
		lat := core.GetLatitudeForBand(coord.Lat, shell.LatBands) * math.Pi / 180.0
		lon := float64(coord.Lon) * 2.0 * math.Pi / float64(len(shell.Voxels[coord.Lat]))

		// Position unit vector
		voxelX := math.Cos(lat) * math.Cos(lon)
		voxelY := math.Cos(lat) * math.Sin(lon)
		voxelZ := math.Sin(lat)

		// Velocity from rotation = omega × r
		// v = ω * R * sin(angle between pole and position)
		dotProduct := poleX*voxelX + poleY*voxelY + poleZ*voxelZ
		sinAngle := math.Sqrt(1.0 - dotProduct*dotProduct)

		velocity := plate.AngularVelocity * pm.planet.Radius * sinAngle

		// Cross product to get velocity direction
		// v = pole × position
		velX := poleY*voxelZ - poleZ*voxelY
		velY := poleZ*voxelX - poleX*voxelZ
		velZ := poleX*voxelY - poleY*voxelX

		// Normalize and scale
		velMag := math.Sqrt(velX*velX + velY*velY + velZ*velZ)
		if velMag > 0 {
			velX = velX / velMag * velocity
			velY = velY / velMag * velocity
			velZ = velZ / velMag * velocity
		}

		// Convert to spherical velocity components
		// This is approximate - proper conversion is complex
		voxel.VelNorth = float32(-velX*math.Sin(lon) + velY*math.Cos(lon))
		voxel.VelEast = float32(-velX*math.Sin(lat)*math.Cos(lon) - velY*math.Sin(lat)*math.Sin(lon) + velZ*math.Cos(lat))

		// Radial velocity unchanged (set by convection/thermal effects)
	}
}

// getAverageVelocity calculates mean velocity of plate
func (pm *PlateManager) getAverageVelocity(plate *TectonicPlate) core.Vector3 {
	if len(plate.MemberVoxels) == 0 {
		return core.Vector3{}
	}

	var sumVel core.Vector3
	for _, coord := range plate.MemberVoxels {
		shell := &pm.planet.Shells[coord.Shell]
		voxel := &shell.Voxels[coord.Lat][coord.Lon]

		sumVel.X += float64(voxel.VelEast)
		sumVel.Y += float64(voxel.VelNorth)
		sumVel.Z += float64(voxel.VelR)
	}

	n := float64(len(plate.MemberVoxels))
	return core.Vector3{
		X: sumVel.X / n,
		Y: sumVel.Y / n,
		Z: sumVel.Z / n,
	}
}

// core.Vector3 is already defined in types.go

// getLatitudeForBand is already defined in voxel_planet.go
