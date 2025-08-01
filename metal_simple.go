//go:build darwin && cgo && !nometal
// +build darwin,cgo,!nometal

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Metal -framework Foundation -framework CoreGraphics
#import <Metal/Metal.h>
#import <Foundation/Foundation.h>

typedef struct {
    void* device;
    void* commandQueue;
    void* library;
    void* upliftPipeline;
    void* erosionPipeline;
    void* ownershipPipeline;
} SimpleMetalContext;

typedef struct {
    float x, y, z;
    int plateId;
} MetalVertexData;

typedef struct {
    float x, y, z;
    int plateType;
} MetalPlateData;

int runMetalVertexOwnership(SimpleMetalContext* ctx, MetalVertexData* vertices, MetalPlateData* plates, 
                           int* boundaryVertices, int vertexCount, int plateCount, int boundaryCount);

SimpleMetalContext* createSimpleMetalContext() {
    @autoreleasepool {
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (!device) {
            return NULL;
        }

        id<MTLCommandQueue> queue = [device newCommandQueue];
        if (!queue) {
            return NULL;
        }

        // Simple shader for parallel height updates
        NSString *shaderSource = @"#include <metal_stdlib>\n"
            "using namespace metal;\n"
            "\n"
            "kernel void updateHeights(device float* heights [[buffer(0)]],\n"
            "                         device float* deltas [[buffer(1)]],\n"
            "                         constant uint& count [[buffer(2)]],\n"
            "                         uint index [[thread_position_in_grid]]) {\n"
            "    if (index >= count) return;\n"
            "    heights[index] += deltas[index];\n"
            "    heights[index] = clamp(heights[index], -0.04f, 0.08f);\n"
            "}\n"
            "\n"
            "kernel void applyErosion(device float* heights [[buffer(0)]],\n"
            "                        constant float& scale [[buffer(1)]],\n"
            "                        constant uint& count [[buffer(2)]],\n"
            "                        uint index [[thread_position_in_grid]]) {\n"
            "    if (index >= count) return;\n"
            "    float h = heights[index];\n"
            "    if (h > 0.01f) {\n"
            "        // Only erode higher elevations\n"
            "        float erosion = 0.000003f * scale;\n"
            "        if (h > 0.03f) erosion *= 2.0f;\n"
            "        else if (h > 0.02f) erosion *= 1.5f;\n"
            "        // Don't erode below a minimum elevation\n"
            "        heights[index] = max(h - erosion, 0.005f);\n"
            "    } else if (h > 0.001f && h < 0.01f) {\n"
            "        // Very slow erosion for low elevations\n"
            "        float erosion = 0.000001f * scale;\n"
            "        heights[index] = max(h - erosion, 0.001f);\n"
            "    } else if (h < -0.01f) {\n"
            "        // Isostatic rebound for deep ocean\n"
            "        float rebound = 0.000002f * scale;\n"
            "        heights[index] = min(h + rebound, -0.001f);\n"
            "    }\n"
            "}\n"
            "\n"
            "struct VertexData {\n"
            "    float3 position;\n"
            "    int plateId;\n"
            "};\n"
            "\n"
            "struct PlateData {\n"
            "    float3 center;\n"
            "    int plateType; // 0=oceanic, 1=continental\n"
            "};\n"
            "\n"
            "kernel void updateVertexOwnership(device VertexData* vertices [[buffer(0)]],\n"
            "                                 device const PlateData* plates [[buffer(1)]],\n"
            "                                 device const int* boundaryVertices [[buffer(2)]],\n"
            "                                 constant uint& vertexCount [[buffer(3)]],\n"
            "                                 constant uint& plateCount [[buffer(4)]],\n"
            "                                 constant uint& boundaryCount [[buffer(5)]],\n"
            "                                 uint index [[thread_position_in_grid]]) {\n"
            "    if (index >= boundaryCount) return;\n"
            "    \n"
            "    int vIdx = boundaryVertices[index];\n"
            "    if (vIdx >= vertexCount) return;\n"
            "    \n"
            "    VertexData v = vertices[vIdx];\n"
            "    float maxInfluence = 0.0f;\n"
            "    int bestPlate = v.plateId;\n"
            "    float currentPlateInfluence = 0.0f;\n"
            "    \n"
            "    // Check all plates for influence\n"
            "    for (uint pIdx = 0; pIdx < plateCount; pIdx++) {\n"
            "        PlateData p = plates[pIdx];\n"
            "        \n"
            "        // Distance-based influence\n"
            "        float3 diff = v.position - p.center;\n"
            "        float dist2 = dot(diff, diff);\n"
            "        float influence = 1.0f / (1.0f + dist2 * 10.0f);\n"
            "        \n"
            "        // Bonus for current plate (inertia)\n"
            "        if (pIdx == v.plateId) {\n"
            "            influence *= 1.5f;\n"
            "            currentPlateInfluence = influence;\n"
            "        }\n"
            "        \n"
            "        // Continental plates have stronger influence over oceanic\n"
            "        if (p.plateType == 1 && v.plateId < plateCount) {\n"
            "            if (plates[v.plateId].plateType == 0) {\n"
            "                influence *= 1.3f;\n"
            "            }\n"
            "        }\n"
            "        \n"
            "        if (influence > maxInfluence) {\n"
            "            maxInfluence = influence;\n"
            "            bestPlate = pIdx;\n"
            "        }\n"
            "    }\n"
            "    \n"
            "    // Only change if significantly better\n"
            "    if (bestPlate != v.plateId && maxInfluence > currentPlateInfluence * 1.2f) {\n"
            "        vertices[vIdx].plateId = bestPlate;\n"
            "    }\n"
            "}\n";

        NSError *error = nil;
        id<MTLLibrary> library = [device newLibraryWithSource:shaderSource options:nil error:&error];
        if (!library) {
            NSLog(@"Failed to create Metal library: %@", error);
            return NULL;
        }

        id<MTLFunction> upliftFunction = [library newFunctionWithName:@"updateHeights"];
        id<MTLFunction> erosionFunction = [library newFunctionWithName:@"applyErosion"];
        id<MTLFunction> ownershipFunction = [library newFunctionWithName:@"updateVertexOwnership"];

        if (!upliftFunction || !erosionFunction || !ownershipFunction) {
            return NULL;
        }

        id<MTLComputePipelineState> upliftPipeline = [device newComputePipelineStateWithFunction:upliftFunction error:&error];
        id<MTLComputePipelineState> erosionPipeline = [device newComputePipelineStateWithFunction:erosionFunction error:&error];
        id<MTLComputePipelineState> ownershipPipeline = [device newComputePipelineStateWithFunction:ownershipFunction error:&error];

        if (!upliftPipeline || !erosionPipeline || !ownershipPipeline) {
            NSLog(@"Failed to create compute pipeline: %@", error);
            return NULL;
        }

        SimpleMetalContext* ctx = (SimpleMetalContext*)malloc(sizeof(SimpleMetalContext));
        ctx->device = (__bridge_retained void*)device;
        ctx->commandQueue = (__bridge_retained void*)queue;
        ctx->library = (__bridge_retained void*)library;
        ctx->upliftPipeline = (__bridge_retained void*)upliftPipeline;
        ctx->erosionPipeline = (__bridge_retained void*)erosionPipeline;
        ctx->ownershipPipeline = (__bridge_retained void*)ownershipPipeline;

        return ctx;
    }
}

void destroySimpleMetalContext(SimpleMetalContext* ctx) {
    if (!ctx) return;
    @autoreleasepool {
        if (ctx->device) CFRelease(ctx->device);
        if (ctx->commandQueue) CFRelease(ctx->commandQueue);
        if (ctx->library) CFRelease(ctx->library);
        if (ctx->upliftPipeline) CFRelease(ctx->upliftPipeline);
        if (ctx->erosionPipeline) CFRelease(ctx->erosionPipeline);
        if (ctx->ownershipPipeline) CFRelease(ctx->ownershipPipeline);
        free(ctx);
    }
}

int runMetalHeightUpdate(SimpleMetalContext* ctx, float* heights, float* deltas, int count) {
    @autoreleasepool {
        id<MTLDevice> device = (__bridge id<MTLDevice>)ctx->device;
        id<MTLCommandQueue> queue = (__bridge id<MTLCommandQueue>)ctx->commandQueue;
        id<MTLComputePipelineState> pipeline = (__bridge id<MTLComputePipelineState>)ctx->upliftPipeline;

        // Create buffers
        id<MTLBuffer> heightBuffer = [device newBufferWithBytes:heights
                                                          length:count * sizeof(float)
                                                         options:MTLResourceStorageModeShared];
        id<MTLBuffer> deltaBuffer = [device newBufferWithBytes:deltas
                                                         length:count * sizeof(float)
                                                        options:MTLResourceStorageModeShared];
        uint countVal = count;

        // Create command buffer and encoder
        id<MTLCommandBuffer> commandBuffer = [queue commandBuffer];
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];

        [encoder setComputePipelineState:pipeline];
        [encoder setBuffer:heightBuffer offset:0 atIndex:0];
        [encoder setBuffer:deltaBuffer offset:0 atIndex:1];
        [encoder setBytes:&countVal length:sizeof(uint) atIndex:2];

        // Calculate thread groups
        NSUInteger threadGroupSize = pipeline.maxTotalThreadsPerThreadgroup;
        if (threadGroupSize > count) threadGroupSize = count;

        MTLSize threadsPerThreadgroup = MTLSizeMake(threadGroupSize, 1, 1);
        MTLSize numThreadgroups = MTLSizeMake((count + threadGroupSize - 1) / threadGroupSize, 1, 1);

        [encoder dispatchThreadgroups:numThreadgroups threadsPerThreadgroup:threadsPerThreadgroup];
        [encoder endEncoding];

        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];

        // Copy results back
        float* result = (float*)[heightBuffer contents];
        memcpy(heights, result, count * sizeof(float));

        return 0;
    }
}

int runMetalErosion(SimpleMetalContext* ctx, float* heights, float scale, int count) {
    @autoreleasepool {
        id<MTLDevice> device = (__bridge id<MTLDevice>)ctx->device;
        id<MTLCommandQueue> queue = (__bridge id<MTLCommandQueue>)ctx->commandQueue;
        id<MTLComputePipelineState> pipeline = (__bridge id<MTLComputePipelineState>)ctx->erosionPipeline;

        // Create buffer
        id<MTLBuffer> heightBuffer = [device newBufferWithBytes:heights
                                                          length:count * sizeof(float)
                                                         options:MTLResourceStorageModeShared];
        uint countVal = count;

        // Create command buffer and encoder
        id<MTLCommandBuffer> commandBuffer = [queue commandBuffer];
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];

        [encoder setComputePipelineState:pipeline];
        [encoder setBuffer:heightBuffer offset:0 atIndex:0];
        [encoder setBytes:&scale length:sizeof(float) atIndex:1];
        [encoder setBytes:&countVal length:sizeof(uint) atIndex:2];

        // Calculate thread groups
        NSUInteger threadGroupSize = pipeline.maxTotalThreadsPerThreadgroup;
        if (threadGroupSize > count) threadGroupSize = count;

        MTLSize threadsPerThreadgroup = MTLSizeMake(threadGroupSize, 1, 1);
        MTLSize numThreadgroups = MTLSizeMake((count + threadGroupSize - 1) / threadGroupSize, 1, 1);

        [encoder dispatchThreadgroups:numThreadgroups threadsPerThreadgroup:threadsPerThreadgroup];
        [encoder endEncoding];

        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];

        // Copy results back
        float* result = (float*)[heightBuffer contents];
        memcpy(heights, result, count * sizeof(float));

        return 0;
    }
}

int runMetalVertexOwnership(SimpleMetalContext* ctx, MetalVertexData* vertices, MetalPlateData* plates, 
                           int* boundaryVertices, int vertexCount, int plateCount, int boundaryCount) {
    @autoreleasepool {
        id<MTLDevice> device = (__bridge id<MTLDevice>)ctx->device;
        id<MTLCommandQueue> queue = (__bridge id<MTLCommandQueue>)ctx->commandQueue;
        id<MTLComputePipelineState> pipeline = (__bridge id<MTLComputePipelineState>)ctx->ownershipPipeline;

        // Create buffers
        id<MTLBuffer> vertexBuffer = [device newBufferWithBytes:vertices
                                                          length:vertexCount * sizeof(MetalVertexData)
                                                         options:MTLResourceStorageModeShared];
        id<MTLBuffer> plateBuffer = [device newBufferWithBytes:plates
                                                         length:plateCount * sizeof(MetalPlateData)
                                                        options:MTLResourceStorageModeShared];
        id<MTLBuffer> boundaryBuffer = [device newBufferWithBytes:boundaryVertices
                                                           length:boundaryCount * sizeof(int)
                                                          options:MTLResourceStorageModeShared];

        uint vCount = vertexCount;
        uint pCount = plateCount;
        uint bCount = boundaryCount;

        // Create command buffer and encoder
        id<MTLCommandBuffer> commandBuffer = [queue commandBuffer];
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];

        [encoder setComputePipelineState:pipeline];
        [encoder setBuffer:vertexBuffer offset:0 atIndex:0];
        [encoder setBuffer:plateBuffer offset:0 atIndex:1];
        [encoder setBuffer:boundaryBuffer offset:0 atIndex:2];
        [encoder setBytes:&vCount length:sizeof(uint) atIndex:3];
        [encoder setBytes:&pCount length:sizeof(uint) atIndex:4];
        [encoder setBytes:&bCount length:sizeof(uint) atIndex:5];

        // Calculate thread groups
        NSUInteger threadGroupSize = pipeline.maxTotalThreadsPerThreadgroup;
        if (threadGroupSize > boundaryCount) threadGroupSize = boundaryCount;

        MTLSize threadsPerThreadgroup = MTLSizeMake(threadGroupSize, 1, 1);
        MTLSize numThreadgroups = MTLSizeMake((boundaryCount + threadGroupSize - 1) / threadGroupSize, 1, 1);

        [encoder dispatchThreadgroups:numThreadgroups threadsPerThreadgroup:threadsPerThreadgroup];
        [encoder endEncoding];

        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];

        // Copy results back
        MetalVertexData* result = (MetalVertexData*)[vertexBuffer contents];
        memcpy(vertices, result, vertexCount * sizeof(MetalVertexData));

        return 0;
    }
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type SimpleMetalGPU struct {
	context *C.SimpleMetalContext
	enabled bool
}

var simpleMetalGPU *SimpleMetalGPU

func initSimpleMetalGPU() *SimpleMetalGPU {
	ctx := C.createSimpleMetalContext()
	if ctx == nil {
		fmt.Println("Metal initialization failed - using CPU fallback")
		return &SimpleMetalGPU{enabled: false}
	}

	fmt.Println("Metal acceleration initialized successfully!")
	return &SimpleMetalGPU{
		context: ctx,
		enabled: true,
	}
}

func (gpu *SimpleMetalGPU) UpdateTectonics(planet Planet, deltaYears float64) Planet {
	if !gpu.enabled {
		return updateTectonics(planet, deltaYears)
	}

	// Use the version without erosion to avoid double erosion
	planet = updateTectonicsWithoutErosion(planet, deltaYears)

	// Now apply GPU-accelerated erosion
	if deltaYears >= 1000 {
		//fmt.Printf("GPU: Running erosion for deltaYears=%.0f\n", deltaYears)
		gpu.accelerateErosion(&planet, deltaYears)

		// For very large time steps, also do bulk height updates on GPU
		if deltaYears > 100000 {
			fmt.Printf("GPU: Running tectonic acceleration for deltaYears=%.0f\n", deltaYears)
			planet = gpu.accelerateTectonics(planet, deltaYears)
		}

		// Apply sedimentation and isostatic adjustment for smaller time steps
		if deltaYears < 1000000 {
			planet = applySedimentation(planet, deltaYears)
			planet = applyIsostasticAdjustment(planet, deltaYears)
		}
	}

	return planet
}

func (gpu *SimpleMetalGPU) accelerateTectonics(planet Planet, deltaYears float64) Planet {
	// Simple GPU-accelerated tectonic uplift/subsidence
	// This is a simplified version - just applies general height changes

	count := len(planet.Vertices)
	if count == 0 {
		return planet
	}

	// Calculate height deltas based on plate boundaries
	heights := make([]float32, count)
	deltas := make([]float32, count)

	for i, v := range planet.Vertices {
		heights[i] = float32(v.Height)

		// Simple tectonic model for GPU
		// In reality, this would be more complex
		if v.PlateID >= 0 && v.PlateID < len(planet.Plates) {
			plate := planet.Plates[v.PlateID]
			if plate.Type == Continental {
				// Very slight continental uplift
				deltas[i] = float32(0.0000005 * deltaYears / 1000000.0)
			} else {
				// Oceanic crust slight subsidence
				deltas[i] = float32(-0.0000003 * deltaYears / 1000000.0)
			}
		}
	}

	// Apply height changes on GPU
	C.runMetalHeightUpdate(
		gpu.context,
		(*C.float)(unsafe.Pointer(&heights[0])),
		(*C.float)(unsafe.Pointer(&deltas[0])),
		C.int(count),
	)

	// Copy back
	for i := range planet.Vertices {
		planet.Vertices[i].Height = float64(heights[i])
		// Position stays on unit sphere - height is separate
	}

	return planet
}

func (gpu *SimpleMetalGPU) accelerateErosion(planet *Planet, deltaYears float64) {
	count := len(planet.Vertices)
	if count == 0 {
		return
	}

	// Extract heights
	heights := make([]float32, count)
	for i, v := range planet.Vertices {
		heights[i] = float32(v.Height)
	}

	// Run erosion on GPU
	scale := float32(deltaYears / 1000000.0)
	C.runMetalErosion(
		gpu.context,
		(*C.float)(unsafe.Pointer(&heights[0])),
		C.float(scale),
		C.int(count),
	)

	// Copy back
	for i := range planet.Vertices {
		planet.Vertices[i].Height = float64(heights[i])
		// Position stays on unit sphere - height is separate
	}
}

func (gpu *SimpleMetalGPU) accelerateVertexOwnership(planet *Planet, boundaryVertices []int) {
	if !gpu.enabled || len(boundaryVertices) == 0 {
		return
	}

	// Prepare vertex data
	vertices := make([]C.MetalVertexData, len(planet.Vertices))
	for i, v := range planet.Vertices {
		vertices[i] = C.MetalVertexData{
			x:       C.float(v.Position.X),
			y:       C.float(v.Position.Y),
			z:       C.float(v.Position.Z),
			plateId: C.int(v.PlateID),
		}
	}

	// Prepare plate data
	plates := make([]C.MetalPlateData, len(planet.Plates))
	for i, p := range planet.Plates {
		plateType := 0
		if p.Type == Continental {
			plateType = 1
		}
		plates[i] = C.MetalPlateData{
			x:         C.float(p.Center.X),
			y:         C.float(p.Center.Y),
			z:         C.float(p.Center.Z),
			plateType: C.int(plateType),
		}
	}

	// Convert boundary vertices to C array
	boundaryIndices := make([]C.int, len(boundaryVertices))
	for i, v := range boundaryVertices {
		boundaryIndices[i] = C.int(v)
	}

	// Run on GPU
	C.runMetalVertexOwnership(
		gpu.context,
		(*C.MetalVertexData)(unsafe.Pointer(&vertices[0])),
		(*C.MetalPlateData)(unsafe.Pointer(&plates[0])),
		(*C.int)(unsafe.Pointer(&boundaryIndices[0])),
		C.int(len(vertices)),
		C.int(len(plates)),
		C.int(len(boundaryIndices)),
	)

	// Copy back plate IDs
	for i := range planet.Vertices {
		planet.Vertices[i].PlateID = int(vertices[i].plateId)
	}
}

func updateTectonicsSimpleMetal(planet Planet, deltaYears float64) Planet {
	if simpleMetalGPU == nil {
		simpleMetalGPU = initSimpleMetalGPU()
	}

	if simpleMetalGPU.enabled {
		return simpleMetalGPU.UpdateTectonics(planet, deltaYears)
	}

	return updateTectonics(planet, deltaYears)
}

// UpdateTectonicsWithoutErosion is used by Metal backend to avoid double erosion
func updateTectonicsWithoutErosion(planet Planet, deltaYears float64) Planet {
	// Store old heights for spike prevention
	oldPlanet := Planet{
		Vertices: make([]Vertex, len(planet.Vertices)),
	}
	for i, v := range planet.Vertices {
		oldPlanet.Vertices[i].Height = v.Height
	}
	
	planet.GeologicalTime += deltaYears

	// Use realistic plate movement
	planet = updateRealisticPlatesSimple(planet, deltaYears)

	// Only update boundaries periodically or when plates have moved significant
	if len(planet.Boundaries) == 0 || deltaYears > 100000 || int(planet.GeologicalTime)%1000000 == 0 {
		planet.Boundaries = findPlateBoundaries(planet)
	}

	// Apply volcanic activity (less frequent for large time steps)
	if deltaYears < 1000000 {
		planet = applyVolcanism(planet, deltaYears)
	} else {
		// For very large time steps, apply volcanism less frequently
		if int(planet.GeologicalTime)%5000000 == 0 {
			planet = applyVolcanism(planet, 5000000)
		}
	}
	
	// Prevent spikes - clamp height changes based on time step
	maxChange := 0.001 * (deltaYears / 1000.0) // Scale with time
	if maxChange > 0.01 {
		maxChange = 0.01 // Cap maximum change per frame
	}
	planet = clampHeightChanges(planet, oldPlanet, maxChange)
	
	// Apply smoothing for realistic terrain at all speeds
	iterations := 2 // Base smoothing for realistic features
	if deltaYears >= 1000000 {
		iterations = 3 // Extra smoothing at very high speeds
	}
	planet = smoothHeights(planet, iterations)
	
	// Preserve minimum landmass
	planet = preserveLandmass(planet, 0.3)

	// Erosion is handled separately by Metal backend

	return planet
}

func init() {
	simpleMetalGPU = initSimpleMetalGPU()
}
