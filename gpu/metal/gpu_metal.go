// +build darwin

package metal

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Metal -framework CoreGraphics -framework Foundation

#import <Metal/Metal.h>
#import <simd/simd.h>
#include <stdlib.h>

typedef struct {
    void* device;
    void* commandQueue;
    void* library;
    void* temperaturePipeline;
    void* advectionPipeline;
    void* convectionPipeline;
} MetalContext;

// Voxel data structure for GPU
typedef struct {
    uint8_t core.MaterialType;
    float density;
    float temperature;
    float pressure;
    float velR, velTheta, velPhi;
    float age;
    float stress;
    float composition;
    float yieldStrength;
    uint8_t flags; // brittle, fractured, etc
    uint8_t padding[2];
} GPUVoxel;

// Shell metadata
typedef struct {
    float innerRadius;
    float outerRadius;
    int latBands;
    int maxLonCount;
    int voxelOffset; // Offset in global voxel buffer
} GPUShell;

MetalContext* createMetalContext();
void releaseMetalContext(MetalContext* ctx);
int compileShaders(MetalContext* ctx, const char* source);
void* createBuffer(MetalContext* ctx, size_t size, const void* data);
void releaseBuffer(void* buffer);
void* getBufferContents(void* buffer);
int runTemperatureKernel(MetalContext* ctx, void* voxelBuffer, void* shellBuffer, 
                        int voxelCount, float dt, float thermalDiffusivity);
int runConvectionKernel(MetalContext* ctx, void* voxelBuffer, void* shellBuffer, 
                       int voxelCount, float dt);
int runAdvectionKernel(MetalContext* ctx, void* voxelBuffer, void* newVoxelBuffer, 
                      void* shellBuffer, int voxelCount, float dt);
int computeNeighborIndices(MetalContext* ctx, void* neighborBuffer, void* shellBuffer, int voxelCount);
int runTemperatureFastKernel(MetalContext* ctx, void* voxelBuffer, void* neighborBuffer, 
                            int voxelCount, float dt, float thermalDiffusivity);
*/
import "C"

import (
	"worldgenerator/core"
	"fmt"
	"unsafe"
)

// MetalCompute handles GPU acceleration for voxel physics
type MetalCompute struct {
	ctx             *C.MetalContext
	voxelBuffer     unsafe.Pointer // GPU buffer for all voxels
	shellBuffer     unsafe.Pointer // GPU buffer for shell metadata
	tempBuffer      unsafe.Pointer // Temporary buffer for advection
	neighborBuffer  unsafe.Pointer // Precomputed neighbor indices
	totalVoxels     int
	shellCount      int
	initialized     bool
	neighborsReady  bool
}

// NewMetalCompute creates a new Metal compute context
func NewMetalCompute(planet *core.VoxelPlanet) (*MetalCompute, error) {
	ctx := C.createMetalContext()
	if ctx == nil {
		return nil, fmt.Errorf("failed to create Metal context")
	}

	mc := &MetalCompute{
		ctx:         ctx,
		initialized: false,
		shellCount:  len(planet.Shells),
	}

	// Compile shaders
	if err := mc.compileShaders(); err != nil {
		C.releaseMetalContext(ctx)
		return nil, err
	}
	
	// Initialize with planet data
	if err := mc.InitializeForPlanet(planet); err != nil {
		C.releaseMetalContext(ctx)
		return nil, err
	}

	return mc, nil
}

// compileShaders loads and compiles Metal shaders
func (mc *MetalCompute) compileShaders() error {
	shaderSource := metalShaderSource // Defined below
	
	cSource := C.CString(shaderSource)
	defer C.free(unsafe.Pointer(cSource))
	
	result := C.compileShaders(mc.ctx, cSource)
	if result != 0 {
		return fmt.Errorf("failed to compile Metal shaders")
	}
	
	return nil
}

// InitializeForPlanet sets up GPU buffers for a voxel planet
func (mc *MetalCompute) InitializeForPlanet(planet *core.VoxelPlanet) error {
	// Count total voxels
	totalVoxels := 0
	for _, shell := range planet.Shells {
		for _, latVoxels := range shell.Voxels {
			totalVoxels += len(latVoxels)
		}
	}
	
	mc.totalVoxels = totalVoxels
	mc.shellCount = len(planet.Shells)
	
	// Allocate GPU buffers
	voxelSize := C.sizeof_GPUVoxel
	shellSize := C.sizeof_GPUShell
	
	// Allocate voxel buffer
	mc.voxelBuffer = C.createBuffer(mc.ctx, C.size_t(totalVoxels)*C.size_t(voxelSize), nil)
	if mc.voxelBuffer == nil {
		return fmt.Errorf("failed to allocate voxel buffer")
	}
	
	// Allocate temporary buffer for advection
	mc.tempBuffer = C.createBuffer(mc.ctx, C.size_t(totalVoxels)*C.size_t(voxelSize), nil)
	if mc.tempBuffer == nil {
		return fmt.Errorf("failed to allocate temp buffer")
	}
	
	// Allocate shell metadata buffer
	mc.shellBuffer = C.createBuffer(mc.ctx, C.size_t(mc.shellCount)*C.size_t(shellSize), nil)
	if mc.shellBuffer == nil {
		return fmt.Errorf("failed to allocate shell buffer")
	}
	
	// Allocate neighbor indices buffer (6 neighbors per voxel)
	mc.neighborBuffer = C.createBuffer(mc.ctx, C.size_t(totalVoxels*6)*C.size_t(4), nil) // 4 bytes per int
	if mc.neighborBuffer == nil {
		return fmt.Errorf("failed to allocate neighbor buffer")
	}
	
	// Copy initial data to GPU
	if err := mc.uploadPlanetData(planet); err != nil {
		return err
	}
	
	// Compute neighbor indices
	if err := mc.computeNeighborIndices(); err != nil {
		return err
	}
	
	mc.initialized = true
	return nil
}

// UpdateTemperature runs temperature diffusion on GPU
func (mc *MetalCompute) UpdateTemperature(dt float64) error {
	if !mc.initialized {
		return fmt.Errorf("Metal compute not initialized")
	}
	
	// Use fast kernel if neighbors are precomputed
	if mc.neighborsReady {
		result := C.runTemperatureFastKernel(
			mc.ctx,
			mc.voxelBuffer,
			mc.neighborBuffer,
			C.int(mc.totalVoxels),
			C.float(dt),
			C.float(1e-6), // thermal diffusivity
		)
		
		if result != 0 {
			return fmt.Errorf("fast temperature kernel failed")
		}
	} else {
		// Fallback to slower kernel
		result := C.runTemperatureKernel(
			mc.ctx,
			mc.voxelBuffer,
			mc.shellBuffer,
			C.int(mc.totalVoxels),
			C.float(dt),
			C.float(1e-6), // thermal diffusivity
		)
		
		if result != 0 {
			return fmt.Errorf("temperature kernel failed")
		}
	}
	
	return nil
}

// UpdateConvection calculates convection velocities on GPU
func (mc *MetalCompute) UpdateConvection(dt float64) error {
	if !mc.initialized {
		return fmt.Errorf("Metal compute not initialized")
	}
	
	// Run convection kernel
	result := C.runConvectionKernel(
		mc.ctx,
		mc.voxelBuffer,
		mc.shellBuffer,
		C.int(mc.totalVoxels),
		C.float(dt),
	)
	
	if result != 0 {
		return fmt.Errorf("convection kernel failed")
	}
	
	return nil
}

// Release cleans up Metal resources
func (mc *MetalCompute) Release() {
	if mc.voxelBuffer != nil {
		C.releaseBuffer(mc.voxelBuffer)
		mc.voxelBuffer = nil
	}
	if mc.tempBuffer != nil {
		C.releaseBuffer(mc.tempBuffer)
		mc.tempBuffer = nil
	}
	if mc.shellBuffer != nil {
		C.releaseBuffer(mc.shellBuffer)
		mc.shellBuffer = nil
	}
	if mc.neighborBuffer != nil {
		C.releaseBuffer(mc.neighborBuffer)
		mc.neighborBuffer = nil
	}
	if mc.ctx != nil {
		C.releaseMetalContext(mc.ctx)
		mc.ctx = nil
	}
}

// Metal shader source code
const metalShaderSource = `
#include <metal_stdlib>
using namespace metal;

struct Voxel {
    uint8_t core.MaterialType;
    float density;
    float temperature;
    float pressure;
    float velR;
    float velTheta;
    float velPhi;
    float age;
    float stress;
    float composition;
    float yieldStrength;
    uint8_t flags;
    uint8_t padding[2];
};

struct Shell {
    float innerRadius;
    float outerRadius;
    int latBands;
    int maxLonCount;
    int voxelOffset;
};

// Temperature diffusion kernel
kernel void updateTemperature(
    device Voxel* voxels [[buffer(0)]],
    device const Shell* shells [[buffer(1)]],
    constant float& dt [[buffer(2)]],
    constant float& thermalDiffusivity [[buffer(3)]],
    uint3 gid [[thread_position_in_grid]],
    uint3 gridSize [[threads_per_grid]]
) {
    uint voxelIndex = gid.x;
    if (voxelIndex >= gridSize.x) return;
    
    device Voxel& voxel = voxels[voxelIndex];
    
    // Skip air voxels
    if (voxel.core.MaterialType == 0) { // MatAir
        return;
    }
    
    // Find which shell this voxel belongs to
    int shellIdx = -1;
    int localIdx = voxelIndex;
    for (int i = 0; i < 32; i++) { // Max 32 shells
        if (shells[i].voxelOffset <= (int)voxelIndex && 
            (i == 31 || shells[i+1].voxelOffset > (int)voxelIndex)) {
            shellIdx = i;
            localIdx = voxelIndex - shells[i].voxelOffset;
            break;
        }
    }
    
    if (shellIdx < 0) return;
    
    // Simple diffusion: average with neighbors
    float avgTemp = voxel.temperature;
    int neighborCount = 1;
    
    // Check radial neighbors
    if (shellIdx > 0 && localIdx < shells[shellIdx-1].latBands * shells[shellIdx-1].maxLonCount) {
        // Inner neighbor
        int innerIdx = shells[shellIdx-1].voxelOffset + localIdx;
        if (innerIdx >= 0 && innerIdx < (int)gridSize.x) {
            avgTemp += voxels[innerIdx].temperature;
            neighborCount++;
        }
    }
    
    if (shellIdx < 31 && localIdx < shells[shellIdx+1].latBands * shells[shellIdx+1].maxLonCount) {
        // Outer neighbor
        int outerIdx = shells[shellIdx+1].voxelOffset + localIdx;
        if (outerIdx >= 0 && outerIdx < (int)gridSize.x) {
            avgTemp += voxels[outerIdx].temperature;
            neighborCount++;
        }
    }
    
    avgTemp /= float(neighborCount);
    
    // Apply diffusion equation
    float dr = (shells[shellIdx].outerRadius - shells[shellIdx].innerRadius) / 1000.0;
    float dTemp = thermalDiffusivity * (avgTemp - voxel.temperature) * dt / (dr * dr);
    
    // Add internal heating for deep shells
    if (shellIdx < 5) {
        dTemp += 1e-9 * dt; // Radioactive heating
    }
    
    voxel.temperature += dTemp;
    
    // Clamp temperature
    if (voxel.temperature < 0) voxel.temperature = 0;
    if (voxel.temperature > 6000) voxel.temperature = 6000;
}

// Convection velocity calculation
kernel void updateConvection(
    device Voxel* voxels [[buffer(0)]],
    device const Shell* shells [[buffer(1)]],
    constant float& dt [[buffer(2)]],
    uint3 gid [[thread_position_in_grid]],
    uint3 gridSize [[threads_per_grid]]
) {
    uint voxelIndex = gid.x;
    if (voxelIndex >= gridSize.x) return;
    
    device Voxel& voxel = voxels[voxelIndex];
    
    // Skip air and water
    if (voxel.core.MaterialType == 0 || voxel.core.MaterialType == 2) { // MatAir or MatWater
        return;
    }
    
    // Find which shell this voxel belongs to
    int shellIdx = -1;
    for (int i = 0; i < 32; i++) {
        if (shells[i].voxelOffset <= (int)voxelIndex && 
            (i == 31 || shells[i+1].voxelOffset > (int)voxelIndex)) {
            shellIdx = i;
            break;
        }
    }
    
    if (shellIdx < 0 || shellIdx >= 31) return;
    
    // Get temperature gradient with outer neighbor
    int localIdx = voxelIndex - shells[shellIdx].voxelOffset;
    int outerIdx = shells[shellIdx+1].voxelOffset + localIdx;
    
    if (outerIdx >= (int)gridSize.x) return;
    
    float outerTemp = voxels[outerIdx].temperature;
    float deltaT = voxel.temperature - outerTemp;
    
    // Buoyancy calculation
    float alpha = 3e-5; // Thermal expansion coefficient
    float g = 9.81;
    float deltaDensity = voxel.density * alpha * deltaT;
    float buoyancyForce = -deltaDensity * g;
    
    // Continental crust extra buoyancy
    if (voxel.core.MaterialType == 6) { // MatGranite
        float avgDensity = 2900.0;
        if (voxels[outerIdx].core.MaterialType == 5 || voxels[outerIdx].core.MaterialType == 7) { // Basalt or Peridotite
            avgDensity = 2900.0;
        }
        float compositionalBuoyancy = (avgDensity - voxel.density) * g / 100.0;
        buoyancyForce += compositionalBuoyancy;
    }
    
    // Simple velocity update (simplified viscosity)
    float viscosity = 1e21; // Pa·s
    float lengthScale = (shells[shellIdx].outerRadius - shells[shellIdx].innerRadius) / 10.0;
    float velocity = buoyancyForce * lengthScale * lengthScale / (6.0 * 3.14159 * viscosity);
    
    // Apply Rayleigh number criterion
    float thermalDiff = 1e-6;
    float rayleigh = abs(deltaDensity * g * lengthScale * lengthScale * lengthScale / 
        (thermalDiff * viscosity));
    
    if (rayleigh > 1000) {
        voxel.velR = velocity * dt;
        
        // Add some lateral circulation
        voxel.velTheta = velocity * 0.1 * sin(float(localIdx) * 0.1) * dt;
        voxel.velPhi = velocity * 0.1 * cos(float(localIdx) * 0.15) * dt;
    } else {
        // Decay velocities
        voxel.velR *= 0.95;
        voxel.velTheta *= 0.95;
        voxel.velPhi *= 0.95;
    }
}

// Material advection kernel
kernel void advectMaterial(
    device Voxel* voxels [[buffer(0)]],
    device Voxel* newVoxels [[buffer(1)]],
    device const Shell* shells [[buffer(2)]],
    constant float& dt [[buffer(3)]],
    uint3 gid [[thread_position_in_grid]],
    uint3 gridSize [[threads_per_grid]]
) {
    uint voxelIndex = gid.x;
    if (voxelIndex >= gridSize.x) return;
    
    device Voxel& voxel = voxels[voxelIndex];
    device Voxel& newVoxel = newVoxels[voxelIndex];
    
    // Copy current state
    newVoxel = voxel;
    
    // Skip air voxels
    if (voxel.core.MaterialType == 0) { // MatAir
        return;
    }
    
    // Find which shell this voxel belongs to
    int shellIdx = -1;
    int localIdx = voxelIndex;
    for (int i = 0; i < 32; i++) {
        if (shells[i].voxelOffset <= (int)voxelIndex && 
            (i == 31 || shells[i+1].voxelOffset > (int)voxelIndex)) {
            shellIdx = i;
            localIdx = voxelIndex - shells[i].voxelOffset;
            break;
        }
    }
    
    if (shellIdx < 0) return;
    
    // Simple advection for surface materials
    if (shellIdx >= 9) { // Near surface
        // Calculate position in shell
        int latIdx = localIdx / shells[shellIdx].maxLonCount;
        int lonIdx = localIdx % shells[shellIdx].maxLonCount;
        
        // Simple eastward drift for demo
        float yearsPerSecond = dt / (365.25 * 24 * 3600);
        // Make movement visible: 1 cell per 100,000 years at equator
        float cellsToShiftFloat = yearsPerSecond / 100000.0;
        
        // Calculate latitude-based speed (faster at equator)
        float latFraction = float(latIdx) / float(shells[shellIdx].latBands);
        float latitude = (latFraction - 0.5) * 180.0; // -90 to +90 degrees
        float speedFactor = cos(latitude * 3.14159 / 180.0);
        
        cellsToShiftFloat *= speedFactor;
        
        // For now, only shift if we've accumulated enough movement
        int cellsToShift = int(cellsToShiftFloat);
        
        if (cellsToShift > 0 && voxel.core.MaterialType > 2) { // Skip air and water
            // Calculate source position
            int sourceLon = (lonIdx - cellsToShift + 1000 * shells[shellIdx].maxLonCount) % 
                           shells[shellIdx].maxLonCount;
            int sourceIdx = shells[shellIdx].voxelOffset + latIdx * shells[shellIdx].maxLonCount + sourceLon;
            
            if (sourceIdx >= 0 && sourceIdx < (int)gridSize.x) {
                // Copy material properties from source
                device Voxel& sourceVoxel = voxels[sourceIdx];
                if (sourceVoxel.core.MaterialType > 2) { // Don't copy air/water
                    newVoxel.core.MaterialType = sourceVoxel.core.MaterialType;
                    newVoxel.density = sourceVoxel.density;
                    newVoxel.composition = sourceVoxel.composition;
                    newVoxel.age = sourceVoxel.age + float(yearsPerSecond);
                }
            }
        } else if (voxel.core.MaterialType > 2) {
            // Still increment age even if not shifting
            newVoxel.age += float(yearsPerSecond);
        }
        
        // Update velocity to reflect actual movement
        if (voxel.core.MaterialType > 2) {
            // Velocity in m/s (10 cm/year at equator)
            newVoxel.velPhi = 3e-9 * speedFactor;
        }
    }
    
    // Temperature-based material advection for deeper shells
    if (shellIdx < 9 && voxel.velR > 0.0001) {
        // Upwelling material
        if (shellIdx < 31) {
            int outerIdx = shells[shellIdx+1].voxelOffset + localIdx;
            if (outerIdx < (int)gridSize.x) {
                device Voxel& outerVoxel = newVoxels[outerIdx];
                
                // Mix properties
                float mixFactor = 0.001;
                float tempDiff = voxel.temperature - outerVoxel.temperature;
                outerVoxel.temperature += tempDiff * mixFactor;
                
                // Transfer magma composition
                if (voxel.core.MaterialType == 4 && outerVoxel.core.MaterialType != 0) { // MatMagma
                    outerVoxel.composition = (outerVoxel.composition + voxel.composition * mixFactor) / 
                                           (1.0 + mixFactor);
                }
            }
        }
    }
}

// Neighbor lookup kernel - precomputes neighbor indices for faster physics
kernel void computeNeighborIndices(
    device int* neighborIndices [[buffer(0)]], // 6 neighbors per voxel: -r,+r,-lat,+lat,-lon,+lon
    device const Shell* shells [[buffer(1)]],
    uint3 gid [[thread_position_in_grid]],
    uint3 gridSize [[threads_per_grid]]
) {
    uint voxelIndex = gid.x;
    if (voxelIndex >= gridSize.x) return;
    
    // Initialize all neighbors to -1 (no neighbor)
    for (int i = 0; i < 6; i++) {
        neighborIndices[voxelIndex * 6 + i] = -1;
    }
    
    // Find which shell this voxel belongs to
    int shellIdx = -1;
    int localIdx = voxelIndex;
    for (int i = 0; i < 32; i++) {
        if (shells[i].voxelOffset <= (int)voxelIndex && 
            (i == 31 || shells[i+1].voxelOffset > (int)voxelIndex)) {
            shellIdx = i;
            localIdx = voxelIndex - shells[i].voxelOffset;
            break;
        }
    }
    
    if (shellIdx < 0) return;
    
    int latBands = shells[shellIdx].latBands;
    int latIdx = localIdx / shells[shellIdx].maxLonCount;
    int lonIdx = localIdx % shells[shellIdx].maxLonCount;
    int lonCount = shells[shellIdx].maxLonCount; // Approximate
    
    // Radial neighbors
    if (shellIdx > 0) {
        // Inner neighbor (-r)
        neighborIndices[voxelIndex * 6 + 0] = shells[shellIdx-1].voxelOffset + 
            min(localIdx, shells[shellIdx-1].latBands * shells[shellIdx-1].maxLonCount - 1);
    }
    if (shellIdx < 31) {
        // Outer neighbor (+r)
        neighborIndices[voxelIndex * 6 + 1] = shells[shellIdx+1].voxelOffset + 
            min(localIdx, shells[shellIdx+1].latBands * shells[shellIdx+1].maxLonCount - 1);
    }
    
    // Latitude neighbors
    if (latIdx > 0) {
        // North neighbor (-lat)
        neighborIndices[voxelIndex * 6 + 2] = shells[shellIdx].voxelOffset + 
            (latIdx - 1) * lonCount + lonIdx;
    }
    if (latIdx < latBands - 1) {
        // South neighbor (+lat)
        neighborIndices[voxelIndex * 6 + 3] = shells[shellIdx].voxelOffset + 
            (latIdx + 1) * lonCount + lonIdx;
    }
    
    // Longitude neighbors (with wrapping)
    // West neighbor (-lon)
    neighborIndices[voxelIndex * 6 + 4] = shells[shellIdx].voxelOffset + 
        latIdx * lonCount + ((lonIdx - 1 + lonCount) % lonCount);
    
    // East neighbor (+lon)
    neighborIndices[voxelIndex * 6 + 5] = shells[shellIdx].voxelOffset + 
        latIdx * lonCount + ((lonIdx + 1) % lonCount);
}

// Optimized temperature diffusion using precomputed neighbors
kernel void updateTemperatureFast(
    device Voxel* voxels [[buffer(0)]],
    device const int* neighborIndices [[buffer(1)]],
    constant float& dt [[buffer(2)]],
    constant float& thermalDiffusivity [[buffer(3)]],
    uint3 gid [[thread_position_in_grid]],
    uint3 gridSize [[threads_per_grid]]
) {
    uint voxelIndex = gid.x;
    if (voxelIndex >= gridSize.x) return;
    
    device Voxel& voxel = voxels[voxelIndex];
    
    // Skip air voxels
    if (voxel.core.MaterialType == 0) return;
    
    // Get neighbor indices
    device const int* neighbors = &neighborIndices[voxelIndex * 6];
    
    // Calculate average neighbor temperature
    float avgTemp = voxel.temperature;
    int neighborCount = 1;
    
    for (int i = 0; i < 6; i++) {
        int neighborIdx = neighbors[i];
        if (neighborIdx >= 0 && neighborIdx < (int)gridSize.x) {
            device Voxel& neighbor = voxels[neighborIdx];
            if (neighbor.core.MaterialType != 0) { // Not air
                avgTemp += neighbor.temperature;
                neighborCount++;
            }
        }
    }
    
    avgTemp /= float(neighborCount);
    
    // Apply diffusion
    float dTemp = thermalDiffusivity * (avgTemp - voxel.temperature) * dt / (1000.0 * 1000.0);
    
    // Add internal heating for deep voxels
    if (voxel.temperature > 4000) { // Deep mantle/core
        dTemp += 1e-9 * dt;
    }
    
    voxel.temperature += dTemp;
    
    // Clamp
    voxel.temperature = clamp(voxel.temperature, 0.0f, 6000.0f);
}
`

// uploadPlanetData transfers planet voxel data to GPU
func (mc *MetalCompute) uploadPlanetData(planet *core.VoxelPlanet) error {
	// Get GPU buffer pointers
	voxelData := (*[1 << 30]C.GPUVoxel)(C.getBufferContents(mc.voxelBuffer))[:mc.totalVoxels:mc.totalVoxels]
	shellData := (*[1 << 20]C.GPUShell)(C.getBufferContents(mc.shellBuffer))[:mc.shellCount:mc.shellCount]
	
	// Copy voxel data
	voxelIndex := 0
	for shellIdx, shell := range planet.Shells {
		// Set shell metadata
		shellData[shellIdx].innerRadius = C.float(shell.InnerRadius)
		shellData[shellIdx].outerRadius = C.float(shell.OuterRadius)
		shellData[shellIdx].latBands = C.int(shell.LatBands)
		shellData[shellIdx].maxLonCount = C.int(len(shell.Voxels[0])) // Approximate
		shellData[shellIdx].voxelOffset = C.int(voxelIndex)
		
		// Copy voxels
		for _, latVoxels := range shell.Voxels {
			for _, voxel := range latVoxels {
				if voxelIndex >= mc.totalVoxels {
					return fmt.Errorf("voxel index overflow")
				}
				
				voxelData[voxelIndex].core.MaterialType = C.uint8_t(voxel.Type)
				voxelData[voxelIndex].density = C.float(voxel.Density)
				voxelData[voxelIndex].temperature = C.float(voxel.Temperature)
				voxelData[voxelIndex].pressure = C.float(voxel.Pressure)
				voxelData[voxelIndex].velR = C.float(voxel.VelR)
				voxelData[voxelIndex].velTheta = C.float(voxel.VelTheta)
				voxelData[voxelIndex].velPhi = C.float(voxel.VelPhi)
				voxelData[voxelIndex].age = C.float(voxel.Age)
				voxelData[voxelIndex].stress = C.float(voxel.Stress)
				voxelData[voxelIndex].composition = C.float(voxel.Composition)
				voxelData[voxelIndex].yieldStrength = C.float(voxel.YieldStrength)
				
				// Pack flags
				flags := C.uint8_t(0)
				if voxel.IsBrittle {
					flags |= 1
				}
				if voxel.IsFractured {
					flags |= 2
				}
				voxelData[voxelIndex].flags = flags
				
				voxelIndex++
			}
		}
	}
	
	return nil
}

// downloadPlanetData transfers GPU data back to planet
func (mc *MetalCompute) downloadPlanetData(planet *core.VoxelPlanet) error {
	// Get GPU buffer pointers
	voxelData := (*[1 << 30]C.GPUVoxel)(C.getBufferContents(mc.voxelBuffer))[:mc.totalVoxels:mc.totalVoxels]
	
	// Copy voxel data back
	voxelIndex := 0
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if voxelIndex >= mc.totalVoxels {
					return fmt.Errorf("voxel index overflow")
				}
				
				voxel := &shell.Voxels[latIdx][lonIdx]
				gpuVoxel := &voxelData[voxelIndex]
				
				voxel.Type = core.MaterialType(gpuVoxel.core.MaterialType)
				voxel.Density = float32(gpuVoxel.density)
				voxel.Temperature = float32(gpuVoxel.temperature)
				voxel.Pressure = float32(gpuVoxel.pressure)
				voxel.VelR = float32(gpuVoxel.velR)
				voxel.VelTheta = float32(gpuVoxel.velTheta)
				voxel.VelPhi = float32(gpuVoxel.velPhi)
				voxel.Age = float32(gpuVoxel.age)
				voxel.Stress = float32(gpuVoxel.stress)
				voxel.Composition = float32(gpuVoxel.composition)
				voxel.YieldStrength = float32(gpuVoxel.yieldStrength)
				
				// Unpack flags
				voxel.IsBrittle = (gpuVoxel.flags & 1) != 0
				voxel.IsFractured = (gpuVoxel.flags & 2) != 0
				
				voxelIndex++
			}
		}
	}
	
	return nil
}

// computeNeighborIndices precomputes neighbor relationships for fast lookup
func (mc *MetalCompute) computeNeighborIndices() error {
	result := C.computeNeighborIndices(
		mc.ctx,
		mc.neighborBuffer,
		mc.shellBuffer,
		C.int(mc.totalVoxels),
	)
	
	if result != 0 {
		return fmt.Errorf("failed to compute neighbor indices")
	}
	
	mc.neighborsReady = true
	return nil
}

// UpdateAdvection runs material advection on GPU
func (mc *MetalCompute) UpdateAdvection(dt float64) error {
	if !mc.initialized {
		return fmt.Errorf("Metal compute not initialized")
	}
	
	// Run advection kernel
	result := C.runAdvectionKernel(
		mc.ctx,
		mc.voxelBuffer,
		mc.tempBuffer,
		mc.shellBuffer,
		C.int(mc.totalVoxels),
		C.float(dt),
	)
	
	if result != 0 {
		return fmt.Errorf("advection kernel failed")
	}
	
	// Swap buffers
	mc.voxelBuffer, mc.tempBuffer = mc.tempBuffer, mc.voxelBuffer
	
	return nil
}