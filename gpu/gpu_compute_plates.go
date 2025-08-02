package gpu

import (
	"fmt"
	"worldgenerator/core"
	"worldgenerator/simulation"

	"github.com/go-gl/gl/v4.3-core/gl"
)

// Plate motion compute shader - calculates plate velocities from mantle flow
const plateDynamicsShader = `
#version 430 core

layout(local_size_x = 32, local_size_y = 1, local_size_z = 1) in;

// Voxel data SSBO
layout(std430, binding = 0) buffer VoxelData {
    int type;
    float density;
    float temperature;
    float pressure;
    float velTheta;
    float velPhi;
    float velR;
    float age;
    float viscosity;
    float heatCapacity;
    float thermalConductivity;
    float radioactiveHeat;
    int plateID;
    int isBoundary;
    float padding[2];
} voxels[];

// Shell metadata SSBO
layout(std430, binding = 1) readonly buffer ShellData {
    float innerRadius;
    float outerRadius;
    int latBands;
    int voxelOffset;
    int lonCountOffset;
    float padding[3];
} shells[];

// Longitude counts SSBO
layout(std430, binding = 2) readonly buffer LonCounts {
    int counts[];
} lonCounts;

// Plate data SSBO
layout(std430, binding = 3) buffer PlateData {
    vec4 eulerPole;      // xyz = pole position, w = angular velocity
    vec4 ridgePush;      // xyz = force vector, w = magnitude
    vec4 slabPull;       // xyz = force vector, w = magnitude
    vec4 basalDrag;      // xyz = force vector, w = magnitude
    vec4 properties;     // x = total mass, y = area, z = avg thickness, w = type (0=oceanic, 1=continental, 2=mixed)
    vec4 motion;         // xyz = linear velocity at centroid, w = strain rate
    int memberCount;
    int boundaryCount;
    float avgAge;
    float convergence;
} plates[];

// Uniforms
uniform float deltaTime;
uniform int shellCount;
uniform int plateCount;
uniform float planetRadius;
uniform int surfaceShell;    // Which shell index is the surface
uniform int lithosphereDepth; // How many shells down to include in plate

// Constants
const float EARTH_RADIUS = 6371000.0;
const float MAX_PLATE_VELOCITY = 0.2; // m/year = 20 cm/year
const float MANTLE_VISCOSITY = 1e21; // Pa·s

// Find voxel index from shell/lat/lon
int getVoxelIndex(int shell, int lat, int lon) {
    if (shell < 0 || shell >= shellCount) return -1;
    
    int offset = shells[shell].voxelOffset;
    int latBands = shells[shell].latBands;
    
    if (lat < 0 || lat >= latBands) return -1;
    
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    if (lon < 0 || lon >= lonCount) return -1;
    
    for (int i = 0; i < lat; i++) {
        offset += lonCounts.counts[lonCountOffset + i];
    }
    
    return offset + lon;
}

// Convert lat/lon indices to spherical coordinates
vec3 getSphericalPosition(int shell, int lat, int lon) {
    float radius = (shells[shell].innerRadius + shells[shell].outerRadius) * 0.5;
    int latBands = shells[shell].latBands;
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    // Latitude from -90 to +90
    float latitude = -90.0 + (180.0 * float(lat) / float(latBands - 1));
    float latRad = radians(latitude);
    
    // Longitude from 0 to 360
    float longitude = 360.0 * float(lon) / float(lonCount);
    float lonRad = radians(longitude);
    
    // Convert to Cartesian
    vec3 pos;
    pos.x = radius * cos(latRad) * cos(lonRad);
    pos.y = radius * cos(latRad) * sin(lonRad);
    pos.z = radius * sin(latRad);
    
    return pos;
}

// Calculate coupling between mantle flow and plate motion
vec3 calculateMantleCoupling(int voxelIdx, int plateID) {
    vec3 coupling = vec3(0.0);
    
    // Get voxel beneath this lithosphere point
    // Find shell/lat/lon from index
    int shell = -1;
    int lat = -1; 
    int lon = -1;
    
    // Reverse lookup (inefficient but necessary)
    int accumIdx = 0;
    for (int s = 0; s < shellCount; s++) {
        int shellStart = shells[s].voxelOffset;
        int shellEnd = (s < shellCount - 1) ? shells[s + 1].voxelOffset : int(voxels.length());
        
        if (voxelIdx >= shellStart && voxelIdx < shellEnd) {
            shell = s;
            int offsetInShell = voxelIdx - shellStart;
            int lonCountOffset = shells[s].lonCountOffset;
            int latBands = shells[s].latBands;
            
            accumIdx = 0;
            for (int l = 0; l < latBands; l++) {
                int lonCount = lonCounts.counts[lonCountOffset + l];
                if (offsetInShell < accumIdx + lonCount) {
                    lat = l;
                    lon = offsetInShell - accumIdx;
                    break;
                }
                accumIdx += lonCount;
            }
            break;
        }
    }
    
    if (shell < 0 || lat < 0 || lon < 0) return coupling;
    
    // Sample mantle velocity from deeper shells
    float totalWeight = 0.0;
    vec3 mantleVel = vec3(0.0);
    
    for (int depthOffset = 1; depthOffset <= 3; depthOffset++) {
        int mantleShell = shell - depthOffset;
        if (mantleShell < 0) continue;
        
        int mantleIdx = getVoxelIndex(mantleShell, lat, lon);
        if (mantleIdx < 0) continue;
        
        // Weight by proximity
        float weight = 1.0 / float(depthOffset);
        
        mantleVel += vec3(voxels[mantleIdx].velPhi, voxels[mantleIdx].velTheta, voxels[mantleIdx].velR) * weight;
        totalWeight += weight;
    }
    
    if (totalWeight > 0.0) {
        mantleVel /= totalWeight;
        
        // Coupling strength depends on temperature and viscosity
        float temp = voxels[voxelIdx].temperature;
        float viscosity = voxels[voxelIdx].viscosity;
        
        // Higher temperature = stronger coupling
        float tempFactor = clamp((temp - 1000.0) / 2000.0, 0.0, 1.0);
        
        // Lower viscosity = stronger coupling
        float viscFactor = clamp(1.0 - log(viscosity / 1e19) / 10.0, 0.0, 1.0);
        
        float couplingStrength = tempFactor * viscFactor * 0.5;
        
        coupling = mantleVel * couplingStrength;
    }
    
    return coupling;
}

void main() {
    uint plateID = gl_GlobalInvocationID.x;
    
    if (plateID >= plateCount) return;
    
    // Reset forces
    plates[plateID].ridgePush = vec4(0.0);
    plates[plateID].slabPull = vec4(0.0);
    plates[plateID].basalDrag = vec4(0.0);
    
    // Accumulate forces from all voxels in this plate
    vec3 totalRidgePush = vec3(0.0);
    vec3 totalSlabPull = vec3(0.0);
    vec3 totalBasalDrag = vec3(0.0);
    vec3 plateVelocity = vec3(0.0);
    vec3 plateCentroid = vec3(0.0);
    float totalMass = 0.0;
    int memberCount = 0;
    int boundaryCount = 0;
    
    // Scan all surface voxels
    for (int s = surfaceShell; s >= max(surfaceShell - lithosphereDepth, 0); s--) {
        int shellStart = shells[s].voxelOffset;
        int shellEnd = (s < shellCount - 1) ? shells[s + 1].voxelOffset : int(voxels.length());
        
        for (int idx = shellStart; idx < shellEnd; idx++) {
            if (voxels[idx].plateID != int(plateID)) continue;
            
            memberCount++;
            
            // Get position for force calculations
            vec3 pos = normalize(vec3(idx * 0.1, idx * 0.2, idx * 0.3)); // Placeholder - need proper position
            
            // Mass contribution
            float voxelMass = voxels[idx].density * planetRadius * planetRadius * 0.001; // Simplified
            totalMass += voxelMass;
            
            // Velocity contribution
            plateVelocity += vec3(voxels[idx].velPhi, voxels[idx].velTheta, voxels[idx].velR) * voxelMass;
            
            // Boundary forces
            if (voxels[idx].isBoundary > 0) {
                boundaryCount++;
                
                // Ridge push at divergent boundaries
                if (voxels[idx].temperature > 1500.0 && voxels[idx].velR > 0.0) {
                    // Elevated temperature and upwelling = ridge
                    float ridgeForce = voxels[idx].velR * 1e12;
                    totalRidgePush += pos * ridgeForce;
                }
                
                // Slab pull at subduction zones
                if (voxels[idx].type == 2 && voxels[idx].velR < 0.0) { // Basalt going down
                    float slabForce = -voxels[idx].velR * voxels[idx].density * 1e13;
                    totalSlabPull += vec3(0.0, 0.0, -slabForce);
                }
            }
            
            // Basal drag from mantle coupling
            vec3 mantleVel = calculateMantleCoupling(idx, int(plateID));
            vec3 relativeVel = vec3(voxels[idx].velPhi, voxels[idx].velTheta, 0.0) - mantleVel;
            totalBasalDrag -= relativeVel * MANTLE_VISCOSITY * 0.001; // Simplified drag
        }
    }
    
    // Normalize by total mass
    if (totalMass > 0.0) {
        plateVelocity /= totalMass;
        
        // Update plate forces
        plates[plateID].ridgePush = vec4(totalRidgePush / totalMass, length(totalRidgePush));
        plates[plateID].slabPull = vec4(totalSlabPull / totalMass, length(totalSlabPull));
        plates[plateID].basalDrag = vec4(totalBasalDrag / totalMass, length(totalBasalDrag));
        
        // Calculate net force
        vec3 netForce = totalRidgePush + totalSlabPull + totalBasalDrag;
        
        // Update angular velocity based on torque
        // Simplified - assumes force acts at edge of plate
        float torque = length(cross(plateCentroid, netForce));
        float momentOfInertia = totalMass * planetRadius * planetRadius * 0.4; // Sphere approximation
        
        float angularAccel = torque / momentOfInertia;
        plates[plateID].eulerPole.w += angularAccel * deltaTime;
        
        // Limit angular velocity
        plates[plateID].eulerPole.w = clamp(plates[plateID].eulerPole.w, 
                                           -MAX_PLATE_VELOCITY / planetRadius,
                                            MAX_PLATE_VELOCITY / planetRadius);
        
        // Update motion
        plates[plateID].motion = vec4(plateVelocity, 0.0);
        plates[plateID].memberCount = memberCount;
        plates[plateID].boundaryCount = boundaryCount;
    }
}
`

// Plate boundary interaction shader - handles collisions, subduction, spreading
const plateBoundaryShader = `
#version 430 core

layout(local_size_x = 32, local_size_y = 1, local_size_z = 1) in;

// Same buffer bindings as plate dynamics shader
layout(std430, binding = 0) buffer VoxelData {
    int type;
    float density;
    float temperature;
    float pressure;
    float velTheta;
    float velPhi;
    float velR;
    float age;
    float viscosity;
    float heatCapacity;
    float thermalConductivity;
    float radioactiveHeat;
    int plateID;
    int isBoundary;
    float padding[2];
} voxels[];

layout(std430, binding = 1) readonly buffer ShellData {
    float innerRadius;
    float outerRadius;
    int latBands;
    int voxelOffset;
    int lonCountOffset;
    float padding[3];
} shells[];

layout(std430, binding = 2) readonly buffer LonCounts {
    int counts[];
} lonCounts;

layout(std430, binding = 3) readonly buffer PlateData {
    vec4 eulerPole;
    vec4 ridgePush;
    vec4 slabPull;
    vec4 basalDrag;
    vec4 properties;
    vec4 motion;
    int memberCount;
    int boundaryCount;
    float avgAge;
    float convergence;
} plates[];

// Boundary type classification
const int BOUNDARY_NONE = 0;
const int BOUNDARY_DIVERGENT = 1;
const int BOUNDARY_CONVERGENT = 2;
const int BOUNDARY_TRANSFORM = 3;

// Material types
const int MAT_AIR = 0;
const int MAT_WATER = 1;
const int MAT_BASALT = 2;
const int MAT_GRANITE = 3;
const int MAT_PERIDOTITE = 4;
const int MAT_MAGMA = 5;

uniform float deltaTime;
uniform int shellCount;
uniform int surfaceShell;
uniform float planetRadius;

// Find voxel index from shell/lat/lon (same as in dynamics shader)
int getVoxelIndex(int shell, int lat, int lon) {
    if (shell < 0 || shell >= shellCount) return -1;
    
    int offset = shells[shell].voxelOffset;
    int latBands = shells[shell].latBands;
    
    if (lat < 0 || lat >= latBands) return -1;
    
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    if (lon < 0 || lon >= lonCount) return -1;
    
    for (int i = 0; i < lat; i++) {
        offset += lonCounts.counts[lonCountOffset + i];
    }
    
    return offset + lon;
}

// Get neighbor indices
void getNeighbors(int idx, int shell, int lat, int lon, out int neighbors[4]) {
    int latBands = shells[shell].latBands;
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    // East/West with wrapping
    int eastLon = (lon + 1) % lonCount;
    int westLon = (lon - 1 + lonCount) % lonCount;
    
    neighbors[0] = getVoxelIndex(shell, lat, eastLon);
    neighbors[1] = getVoxelIndex(shell, lat, westLon);
    
    // North/South
    neighbors[2] = (lat > 0) ? getVoxelIndex(shell, lat - 1, lon) : -1;
    neighbors[3] = (lat < latBands - 1) ? getVoxelIndex(shell, lat + 1, lon) : -1;
}

// Classify boundary type based on relative motion
int classifyBoundary(vec3 vel1, vec3 vel2, vec3 normal) {
    vec3 relVel = vel2 - vel1;
    float normalComponent = dot(relVel, normal);
    float tangentialComponent = length(relVel - normalComponent * normal);
    
    if (abs(normalComponent) < 1e-6) {
        return BOUNDARY_NONE;
    } else if (normalComponent > 1e-6) {
        return BOUNDARY_DIVERGENT;
    } else if (normalComponent < -1e-6) {
        // Check angle for transform vs convergent
        if (tangentialComponent > abs(normalComponent) * 0.5) {
            return BOUNDARY_TRANSFORM;
        } else {
            return BOUNDARY_CONVERGENT;
        }
    }
    
    return BOUNDARY_NONE;
}

void main() {
    uint idx = gl_GlobalInvocationID.x;
    
    if (idx >= voxels.length()) return;
    
    // Only process boundary voxels
    if (voxels[idx].isBoundary == 0) return;
    
    // Find shell/lat/lon
    int shell = -1;
    int lat = -1;
    int lon = -1;
    
    for (int s = 0; s < shellCount; s++) {
        int shellStart = shells[s].voxelOffset;
        int shellEnd = (s < shellCount - 1) ? shells[s + 1].voxelOffset : int(voxels.length());
        
        if (idx >= shellStart && idx < shellEnd) {
            shell = s;
            int offsetInShell = int(idx) - shellStart;
            int lonCountOffset = shells[s].lonCountOffset;
            int latBands = shells[s].latBands;
            
            int accumOffset = 0;
            for (int l = 0; l < latBands; l++) {
                int lonCount = lonCounts.counts[lonCountOffset + l];
                if (offsetInShell < accumOffset + lonCount) {
                    lat = l;
                    lon = offsetInShell - accumOffset;
                    break;
                }
                accumOffset += lonCount;
            }
            break;
        }
    }
    
    if (shell < 0 || lat < 0 || lon < 0) return;
    
    // Get neighbors
    int neighbors[4];
    getNeighbors(int(idx), shell, lat, lon, neighbors);
    
    // Process each neighbor
    for (int i = 0; i < 4; i++) {
        if (neighbors[i] < 0) continue;
        
        int neighborPlate = voxels[neighbors[i]].plateID;
        int myPlate = voxels[idx].plateID;
        
        if (neighborPlate == myPlate || neighborPlate < 0) continue;
        
        // Different plates - this is a boundary
        vec3 myVel = vec3(voxels[idx].velPhi, voxels[idx].velTheta, voxels[idx].velR);
        vec3 neighborVel = vec3(voxels[neighbors[i]].velPhi, voxels[neighbors[i]].velTheta, voxels[neighbors[i]].velR);
        
        // Normal direction (simplified - should be proper spherical)
        vec3 normal = normalize(vec3(float(i < 2 ? 1 : 0), float(i >= 2 ? 1 : 0), 0));
        
        int boundaryType = classifyBoundary(myVel, neighborVel, normal);
        
        // Apply boundary processes
        switch (boundaryType) {
            case BOUNDARY_DIVERGENT:
                // Seafloor spreading - create new basalt
                if (voxels[idx].type == MAT_BASALT || voxels[idx].type == MAT_WATER) {
                    voxels[idx].age = 0.0; // New crust
                    voxels[idx].temperature = 1500.0; // Hot from mantle
                    if (voxels[idx].type == MAT_WATER) {
                        voxels[idx].type = MAT_BASALT;
                    }
                }
                break;
                
            case BOUNDARY_CONVERGENT:
                // Subduction or collision
                bool iOceanic = (voxels[idx].type == MAT_BASALT);
                bool neighborOceanic = (voxels[neighbors[i]].type == MAT_BASALT);
                
                if (iOceanic && !neighborOceanic) {
                    // Oceanic plate subducts under continental
                    voxels[idx].velR = -0.01; // Downward motion
                    voxels[idx].temperature += 10.0 * deltaTime; // Heating from friction
                    
                    // Partial melting creates magma
                    if (voxels[idx].temperature > 1200.0 && voxels[idx].pressure > 1e9) {
                        // Small chance to create magma
                        if (fract(sin(float(idx) * 12.9898) * 43758.5453) < 0.001 * deltaTime) {
                            voxels[idx].type = MAT_MAGMA;
                        }
                    }
                } else if (!iOceanic && !neighborOceanic) {
                    // Continental collision - thicken crust
                    voxels[idx].velR = 0.001; // Slight uplift
                    voxels[idx].density *= 1.0001; // Compression
                }
                break;
                
            case BOUNDARY_TRANSFORM:
                // Strike-slip motion - accumulate stress
                float shearStress = length(myVel - neighborVel) * 1e9;
                voxels[idx].pressure += shearStress * deltaTime;
                
                // Earthquake potential (simplified)
                if (voxels[idx].pressure > 1e10) {
                    // Release stress
                    voxels[idx].pressure *= 0.1;
                    // Could trigger actual earthquake mechanics here
                }
                break;
        }
    }
}
`

// Apply plate motion shader - updates voxel velocities based on plate rotation
const applyPlateMotionShader = `
#version 430 core

layout(local_size_x = 32, local_size_y = 1, local_size_z = 1) in;

// Buffer bindings...
layout(std430, binding = 0) buffer VoxelData {
    int type;
    float density;
    float temperature;
    float pressure;
    float velTheta;
    float velPhi;
    float velR;
    float age;
    float viscosity;
    float heatCapacity;
    float thermalConductivity;
    float radioactiveHeat;
    int plateID;
    int isBoundary;
    float padding[2];
} voxels[];

layout(std430, binding = 3) readonly buffer PlateData {
    vec4 eulerPole;
    vec4 ridgePush;
    vec4 slabPull;
    vec4 basalDrag;
    vec4 properties;
    vec4 motion;
    int memberCount;
    int boundaryCount;
    float avgAge;
    float convergence;
} plates[];

uniform float deltaTime;
uniform float planetRadius;

// Convert Euler pole rotation to velocity at a point
vec3 eulerPoleVelocity(vec3 poleAxis, float angularVel, vec3 position) {
    // v = ω × r
    vec3 velocity = cross(poleAxis * angularVel, position);
    return velocity;
}

void main() {
    uint idx = gl_GlobalInvocationID.x;
    
    if (idx >= voxels.length()) return;
    
    int plateID = voxels[idx].plateID;
    if (plateID < 0 || plateID >= plates.length()) return;
    
    // Skip boundary voxels - they have special dynamics
    if (voxels[idx].isBoundary > 0) return;
    
    // Get Euler pole for this plate
    vec3 poleAxis = normalize(plates[plateID].eulerPole.xyz);
    float angularVel = plates[plateID].eulerPole.w;
    
    // Get voxel position (simplified - should use proper spherical coords)
    vec3 position = normalize(vec3(
        cos(float(idx) * 0.1) * cos(float(idx) * 0.2),
        sin(float(idx) * 0.1) * cos(float(idx) * 0.2),
        sin(float(idx) * 0.2)
    )) * planetRadius;
    
    // Calculate velocity from plate rotation
    vec3 plateVel = eulerPoleVelocity(poleAxis, angularVel, position);
    
    // Convert to spherical velocity components
    // This is a simplified conversion - proper spherical coords are complex
    float r = length(position);
    float theta = acos(position.z / r);
    float phi = atan(position.y, position.x);
    
    // Project velocity onto spherical basis vectors
    vec3 r_hat = position / r;
    vec3 theta_hat = vec3(-sin(theta) * cos(phi), -sin(theta) * sin(phi), cos(theta));
    vec3 phi_hat = vec3(-sin(phi), cos(phi), 0.0);
    
    // Update velocities (blend with existing for smooth transition)
    float blendFactor = 0.1; // How quickly to adopt plate motion
    voxels[idx].velTheta = mix(voxels[idx].velTheta, dot(plateVel, theta_hat), blendFactor);
    voxels[idx].velPhi = mix(voxels[idx].velPhi, dot(plateVel, phi_hat), blendFactor);
    // velR is controlled by thermal/convection processes, not plate motion
    
    // Update age
    voxels[idx].age += deltaTime / (365.25 * 24.0 * 3600.0); // Convert to years
}
`

// ComputePlateTectonics adds plate tectonics compute shaders to the physics system
type ComputePlateTectonics struct {
	plateDynamicsProgram uint32
	boundaryProgram      uint32
	applyMotionProgram   uint32

	plateDataSSBO uint32
	plateCount    int
	shellCount    int

	workGroupSize int32
	numWorkGroups int
}

// PlateDataGPU matches the GPU struct layout
type PlateDataGPU struct {
	EulerPole     [4]float32 // xyz = axis, w = angular velocity
	RidgePush     [4]float32 // xyz = force, w = magnitude
	SlabPull      [4]float32 // xyz = force, w = magnitude
	BasalDrag     [4]float32 // xyz = force, w = magnitude
	Properties    [4]float32 // x = mass, y = area, z = thickness, w = type
	Motion        [4]float32 // xyz = velocity, w = strain rate
	MemberCount   int32
	BoundaryCount int32
	AvgAge        float32
	Convergence   float32
}

// NewComputePlateTectonics creates plate tectonics compute shaders
func NewComputePlateTectonics(planet *core.VoxelPlanet, plateManager *simulation.PlateManager) (*ComputePlateTectonics, error) {
	cp := &ComputePlateTectonics{
		plateCount:    len(plateManager.Plates),
		shellCount:    len(planet.Shells),
		workGroupSize: 32,
	}

	// Calculate work groups
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
	}
	cp.numWorkGroups = (totalVoxels + int(cp.workGroupSize) - 1) / int(cp.workGroupSize)

	// Compile shaders
	var err error
	cp.plateDynamicsProgram, err = compileComputeShader(plateDynamicsShader)
	if err != nil {
		return nil, fmt.Errorf("failed to compile plate dynamics shader: %v", err)
	}

	cp.boundaryProgram, err = compileComputeShader(plateBoundaryShader)
	if err != nil {
		return nil, fmt.Errorf("failed to compile boundary shader: %v", err)
	}

	cp.applyMotionProgram, err = compileComputeShader(applyPlateMotionShader)
	if err != nil {
		return nil, fmt.Errorf("failed to compile apply motion shader: %v", err)
	}

	// Create plate data SSBO
	gl.GenBuffers(1, &cp.plateDataSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, cp.plateDataSSBO)

	// Initialize plate data
	plateData := make([]PlateDataGPU, cp.plateCount)
	for i, plate := range plateManager.Plates {
		// Convert plate data to GPU format
		plateData[i].EulerPole[0] = float32(plate.EulerPoleLat)
		plateData[i].EulerPole[1] = float32(plate.EulerPoleLon)
		plateData[i].EulerPole[2] = 0.0 // z component
		plateData[i].EulerPole[3] = float32(plate.AngularVelocity)

		plateData[i].Properties[0] = float32(plate.TotalMass)
		plateData[i].Properties[1] = float32(plate.TotalArea)
		plateData[i].Properties[2] = float32(plate.AverageThickness)

		// Plate type encoding
		switch plate.Type {
		case "oceanic":
			plateData[i].Properties[3] = 0
		case "continental":
			plateData[i].Properties[3] = 1
		default:
			plateData[i].Properties[3] = 2
		}

		plateData[i].MemberCount = int32(len(plate.MemberVoxels))
		plateData[i].BoundaryCount = int32(len(plate.BoundaryVoxels))
		plateData[i].AvgAge = float32(plate.AverageAge)
	}

	// Upload plate data
	if len(plateData) > 0 {
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, len(plateData)*64, gl.Ptr(plateData), gl.DYNAMIC_DRAW)
	}

	fmt.Printf("✅ Plate tectonics compute shaders compiled successfully (%d plates)\n", cp.plateCount)

	return cp, nil
}

// RunPlateDynamics calculates plate forces and velocities
func (cp *ComputePlateTectonics) RunPlateDynamics(deltaTime float32, planetRadius float32, surfaceShell int32) {
	gl.UseProgram(cp.plateDynamicsProgram)

	// Bind plate data SSBO
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, cp.plateDataSSBO)

	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("deltaTime\x00")), deltaTime)
	gl.Uniform1i(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("shellCount\x00")), int32(cp.shellCount))
	gl.Uniform1i(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("plateCount\x00")), int32(cp.plateCount))
	gl.Uniform1f(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("planetRadius\x00")), planetRadius)
	gl.Uniform1i(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("surfaceShell\x00")), surfaceShell)
	gl.Uniform1i(gl.GetUniformLocation(cp.plateDynamicsProgram, gl.Str("lithosphereDepth\x00")), 2) // 2 shells deep

	// Dispatch one work group per plate
	plateWorkGroups := (cp.plateCount + int(cp.workGroupSize) - 1) / int(cp.workGroupSize)
	gl.DispatchCompute(uint32(plateWorkGroups), 1, 1)

	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// RunBoundaryInteractions handles collisions, subduction, spreading
func (cp *ComputePlateTectonics) RunBoundaryInteractions(deltaTime float32, surfaceShell int32, planetRadius float32) {
	gl.UseProgram(cp.boundaryProgram)

	// Bind plate data SSBO
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, cp.plateDataSSBO)

	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(cp.boundaryProgram, gl.Str("deltaTime\x00")), deltaTime)
	gl.Uniform1i(gl.GetUniformLocation(cp.boundaryProgram, gl.Str("shellCount\x00")), int32(cp.shellCount))
	gl.Uniform1i(gl.GetUniformLocation(cp.boundaryProgram, gl.Str("surfaceShell\x00")), surfaceShell)
	gl.Uniform1f(gl.GetUniformLocation(cp.boundaryProgram, gl.Str("planetRadius\x00")), planetRadius)

	// Process all voxels
	gl.DispatchCompute(uint32(cp.numWorkGroups), 1, 1)

	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// ApplyPlateMotion updates voxel velocities based on plate rotation
func (cp *ComputePlateTectonics) ApplyPlateMotion(deltaTime float32, planetRadius float32) {
	gl.UseProgram(cp.applyMotionProgram)

	// Bind plate data SSBO
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, cp.plateDataSSBO)

	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(cp.applyMotionProgram, gl.Str("deltaTime\x00")), deltaTime)
	gl.Uniform1f(gl.GetUniformLocation(cp.applyMotionProgram, gl.Str("planetRadius\x00")), planetRadius)

	// Process all voxels
	gl.DispatchCompute(uint32(cp.numWorkGroups), 1, 1)

	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// RunFullPlateStep runs complete plate tectonics simulation step
func (cp *ComputePlateTectonics) RunFullPlateStep(deltaTime float32, planetRadius float32, surfaceShell int32) {
	// 1. Calculate plate dynamics from mantle coupling
	cp.RunPlateDynamics(deltaTime, planetRadius, surfaceShell)

	// 2. Process boundary interactions
	cp.RunBoundaryInteractions(deltaTime, surfaceShell, planetRadius)

	// 3. Apply plate motion to voxels
	cp.ApplyPlateMotion(deltaTime, planetRadius)
}

// Release cleans up GPU resources
func (cp *ComputePlateTectonics) Release() {
	if cp.plateDynamicsProgram != 0 {
		gl.DeleteProgram(cp.plateDynamicsProgram)
	}
	if cp.boundaryProgram != 0 {
		gl.DeleteProgram(cp.boundaryProgram)
	}
	if cp.applyMotionProgram != 0 {
		gl.DeleteProgram(cp.applyMotionProgram)
	}
	if cp.plateDataSSBO != 0 {
		gl.DeleteBuffers(1, &cp.plateDataSSBO)
	}
}
