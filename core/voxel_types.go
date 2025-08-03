package core

// MaterialType represents different types of matter in the voxel system
type MaterialType uint8

const (
	MatAir MaterialType = iota
	MatWater
	MatBasalt      // Oceanic crust, volcanic
	MatGranite     // Continental crust  
	MatPeridotite  // Mantle rock
	MatMagma       // Molten rock
	MatSediment    // Accumulated sediments
	MatIce         // Frozen water
	MatSand        // Weathered rock
)

// PhaseType represents the state of matter
type PhaseType uint8

const (
	PhaseSolid PhaseType = iota
	PhaseLiquid
	PhaseGas
	PhasePlasma
)

// VoxelMaterial represents the contents of a single voxel
type VoxelMaterial struct {
	Type        MaterialType
	Density     float32 // kg/m³
	Temperature float32 // Kelvin
	Pressure    float32 // Pascals
	
	// Material flow velocity in spherical coordinates
	// VelR: radial (up/down)
	// VelTheta: latitude direction  
	// VelPhi: longitude direction
	VelR     float32
	VelTheta float32
	VelPhi   float32
	
	// Geological properties
	Age    float32 // Years since formation
	Stress float32 // For fracturing/earthquakes
	
	// Composition (simplified - percentage of endmembers)
	// For rock: felsic (0) to mafic (1)
	// For sediment: clay (0) to sand (1)
	Composition float32
	
	// Mechanical properties
	YieldStrength float32 // Pa - stress needed to deform/break
	IsBrittle     bool    // True if brittle (fractures), false if ductile (flows)
	IsFractured   bool    // True if recently fractured
	
	// Plate tracking and movement
	PlateID       int32   // Which tectonic plate this voxel belongs to
	
	// Sub-cell positioning for smooth movement
	SubPosLat     float32 // Position within cell [0,1) in latitude direction
	SubPosLon     float32 // Position within cell [0,1) in longitude direction
	SubPosR       float32 // Position within shell [0,1) for vertical movement
	
	// Elevation tracking
	Elevation     float32 // Height above/below mean radius in meters (positive = mountains, negative = trenches)
	
	// Melting state
	MeltFraction  float32 // Fraction of material that is molten (0-1)
	
	// Movement accumulation (for backward compatibility)
	FracLon       float32 // Accumulated fractional longitude movement
	FracLat       float32 // Accumulated fractional latitude movement  
	LastMoveTime  float32 // Simulation time when last moved (prevents double moves)
	
	// Continuity tracking
	IsTransient   bool    // Created to fill gaps in plate
	SourcePlateID int32   // Original plate (for transient voxels)
	StretchFactor float32 // How much this voxel is stretched (1.0 = normal)
}

// VoxelCoord represents a position in the spherical voxel grid
type VoxelCoord struct {
	Shell int // Radial layer (0 = innermost)
	Lat   int // Latitude band
	Lon   int // Longitude division
}

// SphericalShell represents one radial layer of the planet
type SphericalShell struct {
	InnerRadius float64
	OuterRadius float64
	LatBands    int // Number of latitude divisions
	
	// Voxels stored as [latitude][longitude]
	// Longitude count varies by latitude to maintain roughly equal volumes
	Voxels [][]VoxelMaterial
	
	// Longitude divisions per latitude band
	LonCounts []int
}

// VoxelPlanet represents the entire planet as a voxel grid
type VoxelPlanet struct {
	// Radial shells from core to atmosphere
	Shells []SphericalShell
	
	// Planet properties
	Radius      float64 // Surface radius in meters
	Mass        float64 // Total mass in kg
	Time        float64 // Simulation time in years
	RotationVel float64 // Radians per second
	
	// Optimization structures
	ActiveCells map[VoxelCoord]bool // Cells needing updates
	
	// Visualization cache
	SurfaceMesh *TriangleMesh // Extracted surface for rendering
	MeshDirty   bool          // Needs remeshing
	
	// Physics subsystems (created on demand)
	Physics interface{} // *physics.VoxelPhysics but avoid import cycle
	
	// Virtual voxel system (optional)
	VirtualVoxelSystem *VirtualVoxelSystem
	UseVirtualVoxels   bool
	
	// Global conservation tracking
	TotalWaterVolume   float64 // Total water volume on planet (m³)
	TotalRockVolume    float64 // Total rock volume (for mass conservation)
	SeaLevel           float64 // Current sea level elevation (m)
}

// TriangleMesh for rendering
type TriangleMesh struct {
	Vertices  []Vector3
	Normals   []Vector3
	Colors    []Vector3
	Triangles []int32
}

// Material properties database
var MaterialProperties = map[MaterialType]struct {
	DefaultDensity     float32
	MeltingPoint       float32 // Kelvin at 1 atm
	SpecificHeat       float32 // J/(kg·K)
	ThermalConductivity float32 // W/(m·K)
	Viscosity          float32 // Pa·s (for liquids/gases)
	Strength           float32 // Pa (yield strength for solids)
}{
	MatAir: {
		DefaultDensity:     1.225,
		MeltingPoint:       0, // N/A
		SpecificHeat:       1005,
		ThermalConductivity: 0.024,
		Viscosity:          0.00002,
		Strength:           0,
	},
	MatWater: {
		DefaultDensity:     1000,
		MeltingPoint:       273.15,
		SpecificHeat:       4186,
		ThermalConductivity: 0.6,
		Viscosity:          0.001,
		Strength:           0,
	},
	MatBasalt: {
		DefaultDensity:     2900,
		MeltingPoint:       1473, // ~1200°C
		SpecificHeat:       840,
		ThermalConductivity: 2.0,
		Viscosity:          1e20, // Solid
		Strength:           200e6, // 200 MPa
	},
	MatGranite: {
		DefaultDensity:     2700,
		MeltingPoint:       1473, // Similar to basalt for simplicity
		SpecificHeat:       790,
		ThermalConductivity: 2.5,
		Viscosity:          1e20,
		Strength:           250e6,
	},
	MatPeridotite: {
		DefaultDensity:     3300,
		MeltingPoint:       1673, // ~1400°C
		SpecificHeat:       1000,
		ThermalConductivity: 3.0,
		Viscosity:          1e21, // Very stiff
		Strength:           300e6,
	},
	MatMagma: {
		DefaultDensity:     2700, // Less dense than solid rock
		MeltingPoint:       0,    // Already molten
		SpecificHeat:       1200,
		ThermalConductivity: 1.0,
		Viscosity:          100, // Flows like thick honey
		Strength:           0,
	},
	MatSediment: {
		DefaultDensity:     2500,
		MeltingPoint:       1373,
		SpecificHeat:       800,
		ThermalConductivity: 1.5,
		Viscosity:          1e18, // Deforms slowly
		Strength:           50e6, // Weaker than solid rock
	},
	MatIce: {
		DefaultDensity:     917,
		MeltingPoint:       273.15,
		SpecificHeat:       2090,
		ThermalConductivity: 2.2,
		Viscosity:          1e13, // Glacial flow
		Strength:           5e6,  // Relatively weak
	},
}

// Helper methods

// GetLatitudeForBand returns the latitude in degrees for a given latitude band index
func GetLatitudeForBand(latIndex int, latBands int) float64 {
	// Calculate the center latitude of the band
	return (float64(latIndex) + 0.5) / float64(latBands) * 180.0 - 90.0
}

// GetLonCount returns the number of longitude divisions for a given latitude band
// Uses equal-area division to maintain roughly constant voxel volumes
// GetLonCount calculates the number of longitude divisions for a given latitude band
func GetLonCount(latBands int, latIndex int) int {
	// Simple equal-area: longitude divisions proportional to cos(latitude)
	lat := GetLatitudeForBand(latIndex, latBands)
	latRad := lat * (3.14159265359 / 180.0)
	
	baseLonDivisions := latBands * 2 // Base number at equator
	lonCount := int(float64(baseLonDivisions) * cos(latRad))
	
	if lonCount < 4 {
		lonCount = 4 // Minimum divisions even at poles
	}
	
	return lonCount
}

// cos returns cosine of x
func cos(x float64) float64 {
	// Simple implementation - replace with math.Cos
	return 1.0 - x*x/2.0 + x*x*x*x/24.0
}