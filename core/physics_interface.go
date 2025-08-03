package core

// PhysicsInterface defines the interface that physics systems must implement
// This allows the core package to interact with physics without import cycles
type PhysicsInterface interface {
	// GetPlateManager returns the plate manager if available
	GetPlateManager() PlateManagerInterface
}

// PlateManagerInterface defines what the core package needs from plate management
type PlateManagerInterface interface {
	// GetVoxelPlateID returns the plate ID for a given voxel coordinate
	GetVoxelPlateID(coord VoxelCoord) (int, bool)
	
	// IsBoundaryVoxel checks if a voxel is on a plate boundary
	IsBoundaryVoxel(coord VoxelCoord) bool
	
	// GetPlateCount returns the number of identified plates
	GetPlateCount() int
	
	// UpdatePlates recalculates plate boundaries and properties
	UpdatePlates()
}

// GetPhysics safely retrieves the physics interface from a planet
func GetPhysics(planet *VoxelPlanet) PhysicsInterface {
	if planet.Physics == nil {
		return nil
	}
	if physics, ok := planet.Physics.(PhysicsInterface); ok {
		return physics
	}
	return nil
}

// GetPlateManager safely retrieves the plate manager from a planet
func GetPlateManager(planet *VoxelPlanet) PlateManagerInterface {
	physics := GetPhysics(planet)
	if physics == nil {
		return nil
	}
	return physics.GetPlateManager()
}