package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
	"strings"
)

// ComputePhysics implements GPU physics using OpenGL compute shaders
type ComputePhysics struct {
	// Compute shader programs
	temperatureDiffusionProgram uint32
	convectionProgram          uint32
	advectionProgram           uint32
	phaseTransitionProgram     uint32
	
	// Plate tectonics
	plateTectonics *ComputePlateTectonics
	
	// Work group sizes
	workGroupSizeX int32
	workGroupSizeY int32
	workGroupSizeZ int32
	
	// Total work groups needed
	numWorkGroupsX int
	numWorkGroupsY int
	numWorkGroupsZ int
	
	// Planet parameters
	totalVoxels int
	shellCount  int
	planetRef   *VoxelPlanet
}

// Temperature diffusion compute shader
const temperatureDiffusionShader = `
#version 430 core

layout(local_size_x = 32, local_size_y = 1, local_size_z = 1) in;

// Voxel data SSBO
layout(std430, binding = 0) buffer VoxelData {
    // Material properties
    int type;
    float density;
    float temperature;
    float pressure;
    
    // Velocity
    float velTheta;
    float velPhi;
    float velR;
    
    // Additional properties
    float age;
    float viscosity;
    float heatCapacity;
    float thermalConductivity;
    float radioactiveHeat;
    
    // Plate tectonics
    int plateID;
    int isBoundary;
    
    // Padding
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

// Uniforms
uniform float deltaTime;
uniform int shellCount;
uniform float planetRadius;

// Find voxel index from shell/lat/lon coordinates
int getVoxelIndex(int shell, int lat, int lon) {
    if (shell < 0 || shell >= shellCount) return -1;
    
    int offset = shells[shell].voxelOffset;
    int latBands = shells[shell].latBands;
    
    if (lat < 0 || lat >= latBands) return -1;
    
    // Get longitude count for this latitude
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    if (lon < 0 || lon >= lonCount) return -1;
    
    // Calculate offset within shell
    for (int i = 0; i < lat; i++) {
        offset += lonCounts.counts[lonCountOffset + i];
    }
    
    return offset + lon;
}

// Get neighbor indices with spherical wrapping
void getNeighbors(int idx, int shell, int lat, int lon, out int neighbors[6]) {
    int latBands = shells[shell].latBands;
    int lonCountOffset = shells[shell].lonCountOffset;
    int lonCount = lonCounts.counts[lonCountOffset + lat];
    
    // Initialize all neighbors to -1
    for (int i = 0; i < 6; i++) {
        neighbors[i] = -1;
    }
    
    // Radial neighbors
    neighbors[0] = getVoxelIndex(shell - 1, lat, lon); // Inner
    neighbors[1] = getVoxelIndex(shell + 1, lat, lon); // Outer
    
    // Latitudinal neighbors
    if (lat > 0) {
        neighbors[2] = getVoxelIndex(shell, lat - 1, lon);
    }
    if (lat < latBands - 1) {
        neighbors[3] = getVoxelIndex(shell, lat + 1, lon);
    }
    
    // Longitudinal neighbors (with wrapping)
    int lonPrev = (lon - 1 + lonCount) % lonCount;
    int lonNext = (lon + 1) % lonCount;
    neighbors[4] = getVoxelIndex(shell, lat, lonPrev);
    neighbors[5] = getVoxelIndex(shell, lat, lonNext);
}

void main() {
    uint idx = gl_GlobalInvocationID.x;
    
    if (idx >= voxels.length()) return;
    
    // Find shell, lat, lon from linear index
    int shell = -1;
    int lat = -1;
    int lon = -1;
    
    // Find which shell this voxel belongs to
    for (int s = 0; s < shellCount; s++) {
        int shellStart = shells[s].voxelOffset;
        int shellEnd = (s < shellCount - 1) ? shells[s + 1].voxelOffset : int(voxels.length());
        
        if (idx >= shellStart && idx < shellEnd) {
            shell = s;
            int offsetInShell = int(idx) - shellStart;
            
            // Find latitude
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
    
    // Get material properties
    float temperature = voxels[idx].temperature;
    float thermalConductivity = voxels[idx].thermalConductivity;
    float density = voxels[idx].density;
    float heatCapacity = voxels[idx].heatCapacity;
    float radioactiveHeat = voxels[idx].radioactiveHeat;
    
    // Skip air voxels
    if (voxels[idx].type == 0) return;
    
    // Get neighbors
    int neighbors[6];
    getNeighbors(int(idx), shell, lat, lon, neighbors);
    
    // Calculate heat diffusion
    float heatFlow = 0.0;
    float avgRadius = (shells[shell].innerRadius + shells[shell].outerRadius) * 0.5;
    
    // Radial diffusion
    for (int i = 0; i < 2; i++) {
        if (neighbors[i] >= 0 && voxels[neighbors[i]].type != 0) {
            float neighborTemp = voxels[neighbors[i]].temperature;
            float neighborConductivity = voxels[neighbors[i]].thermalConductivity;
            float avgConductivity = (thermalConductivity + neighborConductivity) * 0.5;
            
            float dr = (shells[shell].outerRadius - shells[shell].innerRadius);
            float tempGradient = (neighborTemp - temperature) / dr;
            
            heatFlow += avgConductivity * tempGradient;
        }
    }
    
    // Lateral diffusion (simplified for spherical geometry)
    float lateralScale = 1.0 / (avgRadius * avgRadius);
    for (int i = 2; i < 6; i++) {
        if (neighbors[i] >= 0 && voxels[neighbors[i]].type != 0) {
            float neighborTemp = voxels[neighbors[i]].temperature;
            float neighborConductivity = voxels[neighbors[i]].thermalConductivity;
            float avgConductivity = (thermalConductivity + neighborConductivity) * 0.5;
            
            float tempGradient = (neighborTemp - temperature);
            heatFlow += avgConductivity * tempGradient * lateralScale;
        }
    }
    
    // Add radioactive heating
    heatFlow += radioactiveHeat;
    
    // Update temperature
    float tempChange = (heatFlow * deltaTime) / (density * heatCapacity);
    voxels[idx].temperature = temperature + tempChange;
    
    // Clamp temperature to reasonable bounds
    voxels[idx].temperature = clamp(voxels[idx].temperature, 0.0, 6000.0);
}
`

// Convection compute shader
const convectionShader = `
#version 430 core

layout(local_size_x = 32, local_size_y = 1, local_size_z = 1) in;

// Same buffer layouts as temperature diffusion
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

uniform float deltaTime;
uniform int shellCount;
uniform float planetRadius;
uniform float gravity;

// Reuse getVoxelIndex and getNeighbors functions from temperature shader
int getVoxelIndex(int shell, int lat, int lon) {
    // Same implementation as temperature shader
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

void main() {
    uint idx = gl_GlobalInvocationID.x;
    
    if (idx >= voxels.length()) return;
    
    // Skip non-mantle materials
    int matType = voxels[idx].type;
    if (matType != 4 && matType != 5) return; // Only peridotite and magma
    
    // Find shell location
    int shell = -1;
    for (int s = 0; s < shellCount; s++) {
        int shellStart = shells[s].voxelOffset;
        int shellEnd = (s < shellCount - 1) ? shells[s + 1].voxelOffset : int(voxels.length());
        if (idx >= shellStart && idx < shellEnd) {
            shell = s;
            break;
        }
    }
    
    if (shell < 0) return;
    
    // Get temperature and calculate buoyancy
    float temperature = voxels[idx].temperature;
    float density = voxels[idx].density;
    float viscosity = voxels[idx].viscosity;
    
    // Reference temperature at this depth (linear profile)
    float depth = planetRadius - shells[shell].outerRadius;
    float refTemp = 273.0 + (3000.0 * depth / planetRadius);
    
    // Thermal expansion coefficient
    float alpha = 3e-5; // 1/K
    
    // Buoyancy force (Rayleigh-Bénard convection)
    float tempDiff = temperature - refTemp;
    float buoyancy = gravity * alpha * tempDiff * density;
    
    // Update radial velocity based on buoyancy
    float velR = voxels[idx].velR;
    velR += buoyancy * deltaTime / density;
    
    // Apply viscous damping
    float damping = exp(-deltaTime / viscosity);
    velR *= damping;
    
    // Limit velocity
    float maxVel = 0.1; // m/s
    velR = clamp(velR, -maxVel, maxVel);
    
    voxels[idx].velR = velR;
    
    // Update lateral velocities based on continuity
    // This is a simplified approach - full convection would require solving Navier-Stokes
    float avgRadius = (shells[shell].innerRadius + shells[shell].outerRadius) * 0.5;
    float lateralScale = velR / avgRadius * 0.1; // Simplified lateral flow
    
    voxels[idx].velTheta += lateralScale * sin(idx * 0.1) * deltaTime;
    voxels[idx].velPhi += lateralScale * cos(idx * 0.1) * deltaTime;
    
    // Limit lateral velocities
    voxels[idx].velTheta = clamp(voxels[idx].velTheta, -maxVel, maxVel);
    voxels[idx].velPhi = clamp(voxels[idx].velPhi, -maxVel, maxVel);
}
`

// NewComputePhysics creates a new GPU compute physics engine
func NewComputePhysics(planet *VoxelPlanet) (*ComputePhysics, error) {
	// Check compute shader support
	var maxWorkGroupSize [3]int32
	gl.GetIntegeri_v(gl.MAX_COMPUTE_WORK_GROUP_SIZE, 0, &maxWorkGroupSize[0])
	gl.GetIntegeri_v(gl.MAX_COMPUTE_WORK_GROUP_SIZE, 1, &maxWorkGroupSize[1])
	gl.GetIntegeri_v(gl.MAX_COMPUTE_WORK_GROUP_SIZE, 2, &maxWorkGroupSize[2])
	
	fmt.Printf("Max compute work group size: %d x %d x %d\n", 
		maxWorkGroupSize[0], maxWorkGroupSize[1], maxWorkGroupSize[2])
	
	// Count total voxels
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
	}
	
	cp := &ComputePhysics{
		totalVoxels: totalVoxels,
		shellCount:  len(planet.Shells),
		planetRef:   planet,
		workGroupSizeX: 32, // Match shader local_size_x
		workGroupSizeY: 1,
		workGroupSizeZ: 1,
	}
	
	// Calculate number of work groups needed
	cp.numWorkGroupsX = (totalVoxels + int(cp.workGroupSizeX) - 1) / int(cp.workGroupSizeX)
	cp.numWorkGroupsY = 1
	cp.numWorkGroupsZ = 1
	
	// Compile compute shaders
	var err error
	cp.temperatureDiffusionProgram, err = compileComputeShader(temperatureDiffusionShader)
	if err != nil {
		return nil, fmt.Errorf("failed to compile temperature diffusion shader: %v", err)
	}
	
	cp.convectionProgram, err = compileComputeShader(convectionShader)
	if err != nil {
		return nil, fmt.Errorf("failed to compile convection shader: %v", err)
	}
	
	fmt.Println("✅ Compute shaders compiled successfully")
	fmt.Printf("Total voxels: %d, Work groups: %d\n", totalVoxels, cp.numWorkGroupsX)
	
	return cp, nil
}

// compileComputeShader compiles a compute shader
func compileComputeShader(source string) (uint32, error) {
	shader := gl.CreateShader(gl.COMPUTE_SHADER)
	
	csource, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csource, nil)
	free()
	gl.CompileShader(shader)
	
	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("compute shader compilation failed: %s", log)
	}
	
	program := gl.CreateProgram()
	gl.AttachShader(program, shader)
	gl.LinkProgram(program)
	
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("compute program link failed: %s", log)
	}
	
	gl.DeleteShader(shader)
	
	return program, nil
}

// RunTemperatureDiffusion runs temperature diffusion on GPU
func (cp *ComputePhysics) RunTemperatureDiffusion(deltaTime float32, planetRadius float32) {
	gl.UseProgram(cp.temperatureDiffusionProgram)
	
	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(cp.temperatureDiffusionProgram, gl.Str("deltaTime\x00")), deltaTime)
	gl.Uniform1i(gl.GetUniformLocation(cp.temperatureDiffusionProgram, gl.Str("shellCount\x00")), int32(cp.shellCount))
	gl.Uniform1f(gl.GetUniformLocation(cp.temperatureDiffusionProgram, gl.Str("planetRadius\x00")), planetRadius)
	
	// Dispatch compute shader
	gl.DispatchCompute(uint32(cp.numWorkGroupsX), uint32(cp.numWorkGroupsY), uint32(cp.numWorkGroupsZ))
	
	// Memory barrier to ensure completion
	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// RunConvection runs mantle convection on GPU
func (cp *ComputePhysics) RunConvection(deltaTime float32, planetRadius float32, gravity float32) {
	gl.UseProgram(cp.convectionProgram)
	
	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(cp.convectionProgram, gl.Str("deltaTime\x00")), deltaTime)
	gl.Uniform1i(gl.GetUniformLocation(cp.convectionProgram, gl.Str("shellCount\x00")), int32(cp.shellCount))
	gl.Uniform1f(gl.GetUniformLocation(cp.convectionProgram, gl.Str("planetRadius\x00")), planetRadius)
	gl.Uniform1f(gl.GetUniformLocation(cp.convectionProgram, gl.Str("gravity\x00")), gravity)
	
	// Dispatch compute shader
	gl.DispatchCompute(uint32(cp.numWorkGroupsX), uint32(cp.numWorkGroupsY), uint32(cp.numWorkGroupsZ))
	
	// Memory barrier
	gl.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT)
}

// InitializePlateTectonics sets up plate tectonics if available
func (cp *ComputePhysics) InitializePlateTectonics(plateManager *PlateManager) error {
	if plateManager == nil || len(plateManager.Plates) == 0 {
		return fmt.Errorf("no plates available for tectonics")
	}
	
	pt, err := NewComputePlateTectonics(cp.planetRef, plateManager)
	if err != nil {
		return err
	}
	
	cp.plateTectonics = pt
	return nil
}

// RunPhysicsStep runs a complete physics step on GPU
func (cp *ComputePhysics) RunPhysicsStep(deltaTime float32, planetRadius float32, gravity float32) {
	// Run temperature diffusion
	cp.RunTemperatureDiffusion(deltaTime, planetRadius)
	
	// Run convection
	cp.RunConvection(deltaTime, planetRadius, gravity)
	
	// Run plate tectonics if initialized
	if cp.plateTectonics != nil {
		surfaceShell := int32(cp.shellCount - 2) // Second from top is lithosphere
		cp.plateTectonics.RunFullPlateStep(deltaTime, planetRadius, surfaceShell)
	}
	
	// Additional physics steps can be added here:
	// - Advection
	// - Phase transitions
}

// Release cleans up GPU resources
func (cp *ComputePhysics) Release() {
	if cp.temperatureDiffusionProgram != 0 {
		gl.DeleteProgram(cp.temperatureDiffusionProgram)
	}
	if cp.convectionProgram != 0 {
		gl.DeleteProgram(cp.convectionProgram)
	}
	if cp.advectionProgram != 0 {
		gl.DeleteProgram(cp.advectionProgram)
	}
	if cp.phaseTransitionProgram != 0 {
		gl.DeleteProgram(cp.phaseTransitionProgram)
	}
	if cp.plateTectonics != nil {
		cp.plateTectonics.Release()
	}
}