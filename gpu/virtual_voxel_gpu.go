package gpu

import (
	"fmt"
	"worldgenerator/core"
	
	"github.com/go-gl/gl/v4.3-core/gl"
)

// VirtualVoxelGPU handles GPU-accelerated virtual voxel physics
type VirtualVoxelGPU struct {
	// Shader programs
	physicsProgram  uint32
	mappingProgram  uint32
	clearProgram    uint32
	
	// SSBOs
	voxelBuffer    uint32
	bondBuffer     uint32
	plateBuffer    uint32
	gridBuffer     uint32
	weightBuffer   uint32
	lonCountBuffer uint32
	
	// Data sizes
	numVoxels     int
	numBonds      int
	numPlates     int
	gridSize      int
	
	// References
	planet *core.VoxelPlanet
	system *core.VirtualVoxelSystem
}

// GPUVirtualVoxel matches shader structure (64 bytes)
type GPUVirtualVoxel struct {
	Position     [3]float32 // r, theta, phi
	Mass         float32
	Velocity     [3]float32
	Temperature  float32
	Force        [3]float32
	PlateID      int32
	Material     int32
	BondOffset   int32
	BondCount    int32
	Padding      float32
}

// GPUVoxelBond matches shader structure (16 bytes)
type GPUVoxelBond struct {
	TargetID   int32
	RestLength float32
	Stiffness  float32
	Strength   float32
}

// GPUPlateMotion matches shader structure (16 bytes)
type GPUPlateMotion struct {
	AngularVelocity [3]float32
	Padding         float32
}

// NewVirtualVoxelGPU creates GPU-accelerated virtual voxel system
func NewVirtualVoxelGPU(planet *core.VoxelPlanet, system *core.VirtualVoxelSystem) (*VirtualVoxelGPU, error) {
	vvg := &VirtualVoxelGPU{
		planet:    planet,
		system:    system,
		numVoxels: len(system.VirtualVoxels),
		numBonds:  len(system.Bonds),
		numPlates: 20, // Max plates
	}
	
	// Compile shaders
	if err := vvg.compileShaders(); err != nil {
		return nil, fmt.Errorf("failed to compile shaders: %v", err)
	}
	
	// Create buffers
	if err := vvg.createBuffers(); err != nil {
		return nil, fmt.Errorf("failed to create buffers: %v", err)
	}
	
	// Upload initial data
	if err := vvg.uploadData(); err != nil {
		return nil, fmt.Errorf("failed to upload data: %v", err)
	}
	
	return vvg, nil
}

// compileShaders compiles the compute shaders
func (vvg *VirtualVoxelGPU) compileShaders() error {
	// Load physics shader
	physicsSource := virtualVoxelPhysicsShader
	physicsShader := gl.CreateShader(gl.COMPUTE_SHADER)
	csources, free := gl.Strs(physicsSource)
	gl.ShaderSource(physicsShader, 1, csources, nil)
	free()
	gl.CompileShader(physicsShader)
	
	// Check compilation
	var status int32
	gl.GetShaderiv(physicsShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(physicsShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(physicsShader, logLength, nil, &log[0])
		return fmt.Errorf("physics shader compilation failed: %s", log)
	}
	
	// Create physics program
	vvg.physicsProgram = gl.CreateProgram()
	gl.AttachShader(vvg.physicsProgram, physicsShader)
	gl.LinkProgram(vvg.physicsProgram)
	gl.DeleteShader(physicsShader)
	
	// Load mapping shader
	mappingSource := virtualVoxelMappingShader
	mappingShader := gl.CreateShader(gl.COMPUTE_SHADER)
	csources2, free2 := gl.Strs(mappingSource)
	gl.ShaderSource(mappingShader, 1, csources2, nil)
	free2()
	gl.CompileShader(mappingShader)
	
	// Check compilation
	gl.GetShaderiv(mappingShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(mappingShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(mappingShader, logLength, nil, &log[0])
		return fmt.Errorf("mapping shader compilation failed: %s", log)
	}
	
	// Create mapping program
	vvg.mappingProgram = gl.CreateProgram()
	gl.AttachShader(vvg.mappingProgram, mappingShader)
	gl.LinkProgram(vvg.mappingProgram)
	gl.DeleteShader(mappingShader)
	
	return nil
}

// createBuffers creates GPU buffers
func (vvg *VirtualVoxelGPU) createBuffers() error {
	// Virtual voxel buffer
	gl.GenBuffers(1, &vvg.voxelBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.voxelBuffer)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, vvg.numVoxels*64, nil, gl.DYNAMIC_DRAW)
	
	// Bond buffer
	gl.GenBuffers(1, &vvg.bondBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.bondBuffer)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, vvg.numBonds*16, nil, gl.STATIC_DRAW)
	
	// Plate motion buffer
	gl.GenBuffers(1, &vvg.plateBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.plateBuffer)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, vvg.numPlates*16, nil, gl.DYNAMIC_DRAW)
	
	// Grid buffer (for surface shell)
	surfaceShell := vvg.planet.Shells[len(vvg.planet.Shells)-2]
	vvg.gridSize = 0
	for _, count := range surfaceShell.LonCounts {
		vvg.gridSize += count
	}
	
	gl.GenBuffers(1, &vvg.gridBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.gridBuffer)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, vvg.gridSize*64, nil, gl.DYNAMIC_DRAW)
	
	// Weight buffer for accumulation
	gl.GenBuffers(1, &vvg.weightBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.weightBuffer)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, vvg.gridSize*4, nil, gl.DYNAMIC_DRAW)
	
	// Longitude count buffer
	gl.GenBuffers(1, &vvg.lonCountBuffer)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.lonCountBuffer)
	lonCounts := make([]int32, surfaceShell.LatBands)
	for i, count := range surfaceShell.LonCounts {
		lonCounts[i] = int32(count)
	}
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, len(lonCounts)*4, gl.Ptr(lonCounts), gl.STATIC_DRAW)
	
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, 0)
	
	return nil
}

// uploadData uploads virtual voxel data to GPU
func (vvg *VirtualVoxelGPU) uploadData() error {
	// Convert virtual voxels to GPU format
	gpuVoxels := make([]GPUVirtualVoxel, vvg.numVoxels)
	for i, vv := range vvg.system.VirtualVoxels {
		gpuVoxels[i] = GPUVirtualVoxel{
			Position:    [3]float32{vv.Position.R, vv.Position.Theta, vv.Position.Phi},
			Mass:        vv.Mass,
			Velocity:    [3]float32{vv.Velocity.R, vv.Velocity.Theta, vv.Velocity.Phi},
			Temperature: vv.Temperature,
			Force:       [3]float32{0, 0, 0},
			PlateID:     vv.PlateID,
			Material:    int32(vv.Material),
			BondOffset:  vv.BondOffset,
			BondCount:   vv.BondCount,
		}
	}
	
	// Upload voxels
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.voxelBuffer)
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, len(gpuVoxels)*64, gl.Ptr(gpuVoxels))
	
	// Convert bonds to GPU format
	gpuBonds := make([]GPUVoxelBond, vvg.numBonds)
	for i, bond := range vvg.system.Bonds {
		gpuBonds[i] = GPUVoxelBond{
			TargetID:   bond.TargetID,
			RestLength: bond.RestLength,
			Stiffness:  bond.Stiffness,
			Strength:   bond.Strength,
		}
	}
	
	// Upload bonds
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.bondBuffer)
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, len(gpuBonds)*16, gl.Ptr(gpuBonds))
	
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, 0)
	
	return nil
}

// UpdatePhysics runs the physics compute shader
func (vvg *VirtualVoxelGPU) UpdatePhysics(dt float32) {
	//fmt.Printf("GPU Physics Update: dt=%.6f\n", dt)
	
	gl.UseProgram(vvg.physicsProgram)
	
	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(vvg.physicsProgram, gl.Str("deltaTime\x00")), dt)
	gl.Uniform1f(gl.GetUniformLocation(vvg.physicsProgram, gl.Str("planetRadius\x00")), float32(vvg.planet.Radius))
	gl.Uniform1i(gl.GetUniformLocation(vvg.physicsProgram, gl.Str("numVoxels\x00")), int32(vvg.numVoxels))
	
	// Bind buffers
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, vvg.voxelBuffer)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, vvg.bondBuffer)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 2, vvg.plateBuffer)
	
	// Dispatch compute
	workGroups := (vvg.numVoxels + 255) / 256
	gl.DispatchCompute(uint32(workGroups), 1, 1)
	
	// Memory barrier
	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// MapToGrid runs the mapping compute shader
func (vvg *VirtualVoxelGPU) MapToGrid() {
	// First, clear the grid and weights
	vvg.clearGrid()
	
	// Then map virtual voxels to grid
	gl.UseProgram(vvg.mappingProgram)
	
	// Surface shell parameters
	surfaceShell := len(vvg.planet.Shells) - 2
	shell := &vvg.planet.Shells[surfaceShell]
	
	// Set uniforms
	gl.Uniform1i(gl.GetUniformLocation(vvg.mappingProgram, gl.Str("shellIndex\x00")), int32(surfaceShell))
	gl.Uniform1i(gl.GetUniformLocation(vvg.mappingProgram, gl.Str("latBands\x00")), int32(shell.LatBands))
	gl.Uniform1i(gl.GetUniformLocation(vvg.mappingProgram, gl.Str("numVirtualVoxels\x00")), int32(vvg.numVoxels))
	gl.Uniform1f(gl.GetUniformLocation(vvg.mappingProgram, gl.Str("innerRadius\x00")), float32(shell.InnerRadius))
	gl.Uniform1f(gl.GetUniformLocation(vvg.mappingProgram, gl.Str("outerRadius\x00")), float32(shell.OuterRadius))
	
	// Bind buffers
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, vvg.voxelBuffer)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, vvg.gridBuffer)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 2, vvg.weightBuffer)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, vvg.lonCountBuffer)
	
	// Dispatch compute - one thread per virtual voxel
	workGroups := (vvg.numVoxels + 255) / 256
	gl.DispatchCompute(uint32(workGroups), 1, 1)
	
	// Memory barrier
	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
	
	// Read back grid data to CPU voxels
	vvg.readBackGridGPU()
}

// clearGrid clears the grid buffer before mapping
func (vvg *VirtualVoxelGPU) clearGrid() {
	// For now, clear using CPU
	// TODO: Implement GPU clear shader
	clearData := make([]byte, vvg.gridSize*64)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.gridBuffer)
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, len(clearData), gl.Ptr(clearData))
	
	clearWeights := make([]float32, vvg.gridSize)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.weightBuffer)
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, len(clearWeights)*4, gl.Ptr(clearWeights))
}

// readBackGrid copies GPU grid data back to CPU voxels
func (vvg *VirtualVoxelGPU) readBackGrid() {
	// This would read the grid buffer and update planet.Shells
	// For now, we'll use the CPU mapping as fallback
	vvg.system.MapToGrid()
}

// readBackGridGPU reads the GPU grid buffer and updates CPU voxels
func (vvg *VirtualVoxelGPU) readBackGridGPU() {
	surfaceShell := len(vvg.planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}
	
	shell := &vvg.planet.Shells[surfaceShell]
	
	// Read grid data from GPU
	gridData := make([]GPUGridVoxel, vvg.gridSize)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.gridBuffer)
	gl.GetBufferSubData(gl.SHADER_STORAGE_BUFFER, 0, vvg.gridSize*64, gl.Ptr(gridData))
	
	// Read weights
	weights := make([]float32, vvg.gridSize)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.weightBuffer)
	gl.GetBufferSubData(gl.SHADER_STORAGE_BUFFER, 0, vvg.gridSize*4, gl.Ptr(weights))
	
	// Update CPU voxels
	gridIdx := 0
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			if gridIdx >= vvg.gridSize {
				return
			}
			
			voxel := &shell.Voxels[latIdx][lonIdx]
			grid := &gridData[gridIdx]
			weight := weights[gridIdx]
			
			// Update voxel if it has weight from virtual voxels
			if weight > 0.01 {
				voxel.Type = core.MaterialType(grid.Material)
				voxel.Density = grid.Density
				voxel.Temperature = grid.Temperature
				voxel.Pressure = grid.Pressure
				voxel.VelR = grid.Velocity[0]
				voxel.VelTheta = grid.Velocity[1]
				voxel.VelPhi = grid.Velocity[2]
				voxel.Age = grid.Age
				voxel.Stress = grid.Stress
				voxel.Composition = grid.Composition
				voxel.PlateID = grid.PlateID
			} else {
				// No virtual voxel mapped here - set to water
				voxel.Type = core.MatWater
				voxel.Density = core.MaterialProperties[core.MatWater].DefaultDensity
				voxel.PlateID = 0
			}
			
			gridIdx++
		}
	}
	
	// Mark as dirty for rendering
	vvg.planet.MeshDirty = true
}

// GPUGridVoxel matches the shader GridVoxel structure
type GPUGridVoxel struct {
	Material    int32
	Density     float32
	Temperature float32
	Pressure    float32
	Velocity    [3]float32
	Age         float32
	Stress      float32
	Composition float32
	PlateID     int32
}

// SetPlateVelocities updates plate motion data on GPU
func (vvg *VirtualVoxelGPU) SetPlateVelocities(velocities map[int32][3]float32) {
	// Convert to GPU format
	plateMoions := make([]GPUPlateMotion, vvg.numPlates)
	
	for plateID, vel := range velocities {
		if plateID > 0 && int(plateID) <= vvg.numPlates {
			plateMoions[plateID-1] = GPUPlateMotion{
				AngularVelocity: vel,
			}
		}
	}
	
	// Upload to GPU
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, vvg.plateBuffer)
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, len(plateMoions)*16, gl.Ptr(plateMoions))
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, 0)
}

// GetGridBuffer returns the GPU buffer containing mapped grid data
func (vvg *VirtualVoxelGPU) GetGridBuffer() uint32 {
	return vvg.gridBuffer
}

// GetVoxelBuffer returns the GPU buffer containing virtual voxels
func (vvg *VirtualVoxelGPU) GetVoxelBuffer() uint32 {
	return vvg.voxelBuffer
}

// GetNumVoxels returns the number of virtual voxels
func (vvg *VirtualVoxelGPU) GetNumVoxels() int {
	return vvg.numVoxels
}

// Release cleans up GPU resources
func (vvg *VirtualVoxelGPU) Release() {
	gl.DeleteProgram(vvg.physicsProgram)
	gl.DeleteProgram(vvg.mappingProgram)
	if vvg.clearProgram != 0 {
		gl.DeleteProgram(vvg.clearProgram)
	}
	gl.DeleteBuffers(1, &vvg.voxelBuffer)
	gl.DeleteBuffers(1, &vvg.bondBuffer)
	gl.DeleteBuffers(1, &vvg.plateBuffer)
	gl.DeleteBuffers(1, &vvg.gridBuffer)
	gl.DeleteBuffers(1, &vvg.weightBuffer)
	gl.DeleteBuffers(1, &vvg.lonCountBuffer)
}

// Embedded shader sources
const virtualVoxelPhysicsShader = `#version 430 core

layout(local_size_x = 256, local_size_y = 1, local_size_z = 1) in;

struct VirtualVoxel {
    vec3 position;
    float mass;
    vec3 velocity;
    float temperature;
    vec3 force;
    int plateID;
    int material;
    int bondOffset;
    int bondCount;
    float padding;
};

struct VoxelBond {
    int targetID;
    float restLength;
    float stiffness;
    float strength;
};

struct PlateMotion {
    vec3 angularVelocity;
    float padding;
};

layout(std430, binding = 0) buffer VoxelBuffer {
    VirtualVoxel voxels[];
};

layout(std430, binding = 1) buffer BondBuffer {
    VoxelBond bonds[];
};

layout(std430, binding = 2) buffer PlateBuffer {
    PlateMotion plates[];
};

uniform float deltaTime;
uniform float planetRadius;
uniform int numVoxels;

// Calculate angular distance between two spherical positions
float angularDistance(vec3 pos1, vec3 pos2) {
    float dTheta = pos2.y - pos1.y;
    float dPhi = pos2.z - pos1.z;
    
    float a = sin(dTheta/2) * sin(dTheta/2) +
              cos(pos1.y) * cos(pos2.y) * sin(dPhi/2) * sin(dPhi/2);
    
    return 2.0 * atan(sqrt(a), sqrt(1-a));
}

// Calculate spring force between two voxels
vec3 calculateSpringForce(VirtualVoxel v1, VirtualVoxel v2, float restLength, float stiffness) {
    float dist = angularDistance(v1.position, v2.position);
    
    if (dist < 0.0001) return vec3(0.0);
    
    // Spring force magnitude: F = -k * (x - rest_length)
    float forceMag = stiffness * (dist - restLength);
    
    // Direction from v1 to v2 in spherical coordinates
    vec3 direction = normalize(v2.position - v1.position);
    
    return forceMag * direction;
}

void main() {
    uint id = gl_GlobalInvocationID.x;
    if (id >= numVoxels) return;
    
    VirtualVoxel voxel = voxels[id];
    
    // Clear forces
    voxel.force = vec3(0.0);
    
    // Calculate spring forces from bonds
    for (int i = 0; i < voxel.bondCount; i++) {
        int bondIdx = voxel.bondOffset + i;
        VoxelBond bond = bonds[bondIdx];
        
        if (bond.strength <= 0.0) continue; // Broken bond
        
        VirtualVoxel target = voxels[bond.targetID];
        
        // Calculate spring force
        vec3 springForce = calculateSpringForce(voxel, target, bond.restLength, bond.stiffness);
        voxel.force += springForce;
        
        // Check if bond should break
        float currentDist = angularDistance(voxel.position, target.position);
        float strain = abs(currentDist - bond.restLength) / bond.restLength;
        
        if (strain > bond.strength) {
            bonds[bondIdx].strength = 0.0; // Break the bond
        }
    }
    
    // Add plate motion force if applicable
    if (voxel.plateID > 0 && voxel.plateID <= 20) {
        PlateMotion plate = plates[voxel.plateID - 1];
        // Simplified plate velocity - just use angular velocity components
        vec3 targetVel = plate.angularVelocity * planetRadius;
        
        // Moderate force to maintain plate velocity without oscillation
        vec3 plateForce = (targetVel - voxel.velocity) * 10.0;
        voxel.force += plateForce;
    }
    
    // Increase damping to reduce oscillations
    voxel.force -= voxel.velocity * 5.0;
    
    // Update velocity (F = ma, so a = F/m)
    voxel.velocity += voxel.force / voxel.mass * deltaTime;
    
    // Update position
    voxel.position += voxel.velocity * deltaTime;
    
    // Wrap longitude
    if (voxel.position.z > 3.14159265) {
        voxel.position.z -= 2.0 * 3.14159265;
    } else if (voxel.position.z < -3.14159265) {
        voxel.position.z += 2.0 * 3.14159265;
    }
    
    // Clamp latitude
    voxel.position.y = clamp(voxel.position.y, -1.5707963, 1.5707963);
    
    voxels[id] = voxel;
}
` + "\x00"

const virtualVoxelMappingShader = `#version 430 core

layout(local_size_x = 256, local_size_y = 1, local_size_z = 1) in;

struct VirtualVoxel {
    vec3 position;
    float mass;
    vec3 velocity;
    float temperature;
    vec3 force;
    int plateID;
    int material;
    int bondOffset;
    int bondCount;
    float padding;
};

struct GridVoxel {
    int material;
    float density;
    float temperature;
    float pressure;
    vec3 velocity;
    float age;
    float stress;
    float composition;
    int plateID;
};

layout(std430, binding = 0) buffer VirtualVoxelBuffer {
    VirtualVoxel virtualVoxels[];
};

layout(std430, binding = 1) buffer GridBuffer {
    GridVoxel gridVoxels[];
};

layout(std430, binding = 2) buffer WeightBuffer {
    float weights[];
};

layout(std430, binding = 3) buffer LonCountsBuffer {
    int lonCounts[];
};

uniform int shellIndex;
uniform int latBands;
uniform int numVirtualVoxels;
uniform float innerRadius;
uniform float outerRadius;

// Material type constants
const int MAT_AIR = 0;
const int MAT_WATER = 1;
const int MAT_GRANITE = 2;
const int MAT_BASALT = 3;
const int MAT_MANTLE = 4;
const int MAT_MAGMA = 5;

// Material properties
float getMaterialDensity(int material) {
    float densities[6] = float[6](1.2, 1000.0, 2700.0, 2900.0, 3300.0, 2800.0);
    return densities[material];
}

// Get grid index from lat/lon indices
int getGridIndex(int latIdx, int lonIdx) {
    if (latIdx < 0 || latIdx >= latBands) return -1;
    
    // Calculate offset for this latitude band
    int offset = 0;
    for (int i = 0; i < latIdx; i++) {
        offset += lonCounts[i];
    }
    
    int lonCount = lonCounts[latIdx];
    if (lonIdx < 0 || lonIdx >= lonCount) return -1;
    
    return offset + lonIdx;
}

void main() {
    uint id = gl_GlobalInvocationID.x;
    if (id >= numVirtualVoxels) return;
    
    VirtualVoxel voxel = virtualVoxels[id];
    
    // Skip water/air voxels
    if (voxel.material == MAT_AIR || voxel.material == MAT_WATER) return;
    
    // Convert spherical position to grid coordinates
    float lat = voxel.position.y * 180.0 / 3.14159265; // theta to degrees
    float lon = voxel.position.z * 180.0 / 3.14159265; // phi to degrees
    
    // Find affected grid cells with bilinear interpolation
    float latIdx = (lat + 90.0) / 180.0 * float(latBands);
    int lat0 = int(latIdx);
    int lat1 = min(lat0 + 1, latBands - 1);
    float latFrac = fract(latIdx);
    
    // Process both latitude bands
    for (int latBand = 0; latBand < 2; latBand++) {
        int currentLat = (latBand == 0) ? lat0 : lat1;
        if (currentLat < 0 || currentLat >= latBands) continue;
        
        float latWeight = (latBand == 0) ? (1.0 - latFrac) : latFrac;
        
        int lonCount = lonCounts[currentLat];
        float lonIdx = (lon + 180.0) / 360.0 * float(lonCount);
        int lon0 = int(lonIdx);
        int lon1 = (lon0 + 1) % lonCount;
        float lonFrac = fract(lonIdx);
        
        // Process both longitude cells
        for (int lonCell = 0; lonCell < 2; lonCell++) {
            int currentLon = (lonCell == 0) ? lon0 : lon1;
            float lonWeight = (lonCell == 0) ? (1.0 - lonFrac) : lonFrac;
            
            float totalWeight = latWeight * lonWeight;
            if (totalWeight < 0.1) continue; // Skip small contributions
            
            int gridIdx = getGridIndex(currentLat, currentLon);
            if (gridIdx < 0) continue;
            
            // Atomic operations for thread-safe accumulation
            atomicAdd(weights[gridIdx], totalWeight);
            
            // Update grid properties (simplified - in practice needs weighted averaging)
            gridVoxels[gridIdx].material = voxel.material;
            gridVoxels[gridIdx].density = getMaterialDensity(voxel.material);
            gridVoxels[gridIdx].temperature = voxel.temperature;
            gridVoxels[gridIdx].plateID = voxel.plateID;
            gridVoxels[gridIdx].velocity = voxel.velocity;
        }
    }
}
` + "\x00"