// +build darwin

#import <Metal/Metal.h>
#import <Foundation/Foundation.h>
#import <simd/simd.h>
#include <stdlib.h>
#include <string.h>

// Forward declarations matching the header in gpu_metal.go
typedef struct {
    id<MTLDevice> device;
    id<MTLCommandQueue> commandQueue;
    id<MTLLibrary> library;
    id<MTLComputePipelineState> temperaturePipeline;
    id<MTLComputePipelineState> advectionPipeline;
    id<MTLComputePipelineState> convectionPipeline;
} MetalContext;

// Voxel data structure for GPU
typedef struct {
    uint8_t materialType;
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

MetalContext* createMetalContext() {
    @autoreleasepool {
        MetalContext* ctx = (MetalContext*)malloc(sizeof(MetalContext));
        if (!ctx) return NULL;
        
        // Get default Metal device
        ctx->device = MTLCreateSystemDefaultDevice();
        if (!ctx->device) {
            free(ctx);
            return NULL;
        }
        
        // Create command queue
        ctx->commandQueue = [ctx->device newCommandQueue];
        if (!ctx->commandQueue) {
            free(ctx);
            return NULL;
        }
        
        // Initialize other fields
        ctx->library = nil;
        ctx->temperaturePipeline = nil;
        ctx->advectionPipeline = nil;
        ctx->convectionPipeline = nil;
        
        return ctx;
    }
}

void releaseMetalContext(MetalContext* ctx) {
    if (!ctx) return;
    
    @autoreleasepool {
        // Release Metal objects
        ctx->temperaturePipeline = nil;
        ctx->advectionPipeline = nil;
        ctx->convectionPipeline = nil;
        ctx->library = nil;
        ctx->commandQueue = nil;
        ctx->device = nil;
    }
    
    free(ctx);
}

int compileShaders(MetalContext* ctx, const char* source) {
    @autoreleasepool {
        NSError* error = nil;
        
        // Create shader source string
        NSString* shaderSource = [NSString stringWithUTF8String:source];
        
        // Compile shaders
        MTLCompileOptions* options = [[MTLCompileOptions alloc] init];
        ctx->library = [ctx->device newLibraryWithSource:shaderSource 
                                                 options:options 
                                                   error:&error];
        
        if (!ctx->library) {
            NSLog(@"Failed to compile Metal shaders: %@", error);
            return -1;
        }
        
        // Create temperature diffusion pipeline
        id<MTLFunction> temperatureFunction = [ctx->library newFunctionWithName:@"updateTemperature"];
        if (!temperatureFunction) {
            NSLog(@"Failed to find updateTemperature function");
            return -1;
        }
        
        ctx->temperaturePipeline = [ctx->device newComputePipelineStateWithFunction:temperatureFunction 
                                                                              error:&error];
        if (!ctx->temperaturePipeline) {
            NSLog(@"Failed to create temperature pipeline: %@", error);
            return -1;
        }
        
        // Create convection pipeline
        id<MTLFunction> convectionFunction = [ctx->library newFunctionWithName:@"updateConvection"];
        if (!convectionFunction) {
            NSLog(@"Failed to find updateConvection function");
            return -1;
        }
        
        ctx->convectionPipeline = [ctx->device newComputePipelineStateWithFunction:convectionFunction 
                                                                            error:&error];
        if (!ctx->convectionPipeline) {
            NSLog(@"Failed to create convection pipeline: %@", error);
            return -1;
        }
        
        // Create advection pipeline
        id<MTLFunction> advectionFunction = [ctx->library newFunctionWithName:@"advectMaterial"];
        if (!advectionFunction) {
            NSLog(@"Failed to find advectMaterial function");
            return -1;
        }
        
        ctx->advectionPipeline = [ctx->device newComputePipelineStateWithFunction:advectionFunction 
                                                                            error:&error];
        if (!ctx->advectionPipeline) {
            NSLog(@"Failed to create advection pipeline: %@", error);
            return -1;
        }
        
        return 0;
    }
}

void* createBuffer(MetalContext* ctx, size_t size, const void* data) {
    @autoreleasepool {
        id<MTLBuffer> buffer;
        if (data) {
            buffer = [ctx->device newBufferWithBytes:data 
                                              length:size 
                                             options:MTLResourceStorageModeShared];
        } else {
            buffer = [ctx->device newBufferWithLength:size 
                                             options:MTLResourceStorageModeShared];
        }
        
        // Retain and return as void pointer
        return (__bridge_retained void*)buffer;
    }
}

void releaseBuffer(void* buffer) {
    if (!buffer) return;
    @autoreleasepool {
        // Bridge transfer to ARC and let it release
        id<MTLBuffer> metalBuffer = (__bridge_transfer id<MTLBuffer>)buffer;
        metalBuffer = nil;
    }
}

void* getBufferContents(void* buffer) {
    if (!buffer) return NULL;
    @autoreleasepool {
        id<MTLBuffer> metalBuffer = (__bridge id<MTLBuffer>)buffer;
        return [metalBuffer contents];
    }
}

int runTemperatureKernel(MetalContext* ctx, void* voxelBuffer, void* shellBuffer, 
                        int voxelCount, float dt, float thermalDiffusivity) {
    @autoreleasepool {
        // Create command buffer
        id<MTLCommandBuffer> commandBuffer = [ctx->commandQueue commandBuffer];
        if (!commandBuffer) return -1;
        
        // Create compute encoder
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
        if (!encoder) return -1;
        
        // Set pipeline
        [encoder setComputePipelineState:ctx->temperaturePipeline];
        
        // Set buffers
        id<MTLBuffer> voxelMTLBuffer = (__bridge id<MTLBuffer>)voxelBuffer;
        id<MTLBuffer> shellMTLBuffer = (__bridge id<MTLBuffer>)shellBuffer;
        [encoder setBuffer:voxelMTLBuffer offset:0 atIndex:0];
        [encoder setBuffer:shellMTLBuffer offset:0 atIndex:1];
        
        // Set constants
        [encoder setBytes:&dt length:sizeof(float) atIndex:2];
        [encoder setBytes:&thermalDiffusivity length:sizeof(float) atIndex:3];
        
        // Calculate thread groups
        NSUInteger threadsPerThreadgroup = MIN(ctx->temperaturePipeline.maxTotalThreadsPerThreadgroup, 256);
        NSUInteger threadgroupsPerGrid = (voxelCount + threadsPerThreadgroup - 1) / threadsPerThreadgroup;
        
        MTLSize gridSize = MTLSizeMake(voxelCount, 1, 1);
        MTLSize threadgroupSize = MTLSizeMake(threadsPerThreadgroup, 1, 1);
        
        // Dispatch threads
        [encoder dispatchThreadgroups:MTLSizeMake(threadgroupsPerGrid, 1, 1) 
                threadsPerThreadgroup:threadgroupSize];
        
        // End encoding
        [encoder endEncoding];
        
        // Commit and wait
        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];
        
        return 0;
    }
}

int runConvectionKernel(MetalContext* ctx, void* voxelBuffer, void* shellBuffer, 
                       int voxelCount, float dt) {
    @autoreleasepool {
        // Create command buffer
        id<MTLCommandBuffer> commandBuffer = [ctx->commandQueue commandBuffer];
        if (!commandBuffer) return -1;
        
        // Create compute encoder
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
        if (!encoder) return -1;
        
        // Set pipeline
        [encoder setComputePipelineState:ctx->convectionPipeline];
        
        // Set buffers
        id<MTLBuffer> voxelMTLBuffer = (__bridge id<MTLBuffer>)voxelBuffer;
        id<MTLBuffer> shellMTLBuffer = (__bridge id<MTLBuffer>)shellBuffer;
        [encoder setBuffer:voxelMTLBuffer offset:0 atIndex:0];
        [encoder setBuffer:shellMTLBuffer offset:0 atIndex:1];
        
        // Set constants
        [encoder setBytes:&dt length:sizeof(float) atIndex:2];
        
        // Calculate thread groups
        NSUInteger threadsPerThreadgroup = MIN(ctx->convectionPipeline.maxTotalThreadsPerThreadgroup, 256);
        NSUInteger threadgroupsPerGrid = (voxelCount + threadsPerThreadgroup - 1) / threadsPerThreadgroup;
        
        MTLSize gridSize = MTLSizeMake(voxelCount, 1, 1);
        MTLSize threadgroupSize = MTLSizeMake(threadsPerThreadgroup, 1, 1);
        
        // Dispatch threads
        [encoder dispatchThreadgroups:MTLSizeMake(threadgroupsPerGrid, 1, 1) 
                threadsPerThreadgroup:threadgroupSize];
        
        // End encoding
        [encoder endEncoding];
        
        // Commit and wait
        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];
        
        return 0;
    }
}

int runAdvectionKernel(MetalContext* ctx, void* voxelBuffer, void* newVoxelBuffer, 
                      void* shellBuffer, int voxelCount, float dt) {
    @autoreleasepool {
        // Create command buffer
        id<MTLCommandBuffer> commandBuffer = [ctx->commandQueue commandBuffer];
        if (!commandBuffer) return -1;
        
        // Create compute encoder
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
        if (!encoder) return -1;
        
        // Set pipeline
        [encoder setComputePipelineState:ctx->advectionPipeline];
        
        // Set buffers
        id<MTLBuffer> voxelMTLBuffer = (__bridge id<MTLBuffer>)voxelBuffer;
        id<MTLBuffer> newVoxelMTLBuffer = (__bridge id<MTLBuffer>)newVoxelBuffer;
        id<MTLBuffer> shellMTLBuffer = (__bridge id<MTLBuffer>)shellBuffer;
        [encoder setBuffer:voxelMTLBuffer offset:0 atIndex:0];
        [encoder setBuffer:newVoxelMTLBuffer offset:0 atIndex:1];
        [encoder setBuffer:shellMTLBuffer offset:0 atIndex:2];
        
        // Set constants
        [encoder setBytes:&dt length:sizeof(float) atIndex:3];
        
        // Calculate thread groups
        NSUInteger threadsPerThreadgroup = MIN(ctx->advectionPipeline.maxTotalThreadsPerThreadgroup, 256);
        NSUInteger threadgroupsPerGrid = (voxelCount + threadsPerThreadgroup - 1) / threadsPerThreadgroup;
        
        MTLSize gridSize = MTLSizeMake(voxelCount, 1, 1);
        MTLSize threadgroupSize = MTLSizeMake(threadsPerThreadgroup, 1, 1);
        
        // Dispatch threads
        [encoder dispatchThreadgroups:MTLSizeMake(threadgroupsPerGrid, 1, 1) 
                threadsPerThreadgroup:threadgroupSize];
        
        // End encoding
        [encoder endEncoding];
        
        // Commit and wait
        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];
        
        return 0;
    }
}
int computeNeighborIndices(MetalContext* ctx, void* neighborBuffer, void* shellBuffer, int voxelCount) {
    @autoreleasepool {
        // Get the compute neighbor indices function
        id<MTLFunction> function = [ctx->library newFunctionWithName:@"computeNeighborIndices"];
        if (!function) {
            NSLog(@"Failed to find computeNeighborIndices function");
            return -1;
        }
        
        // Create pipeline state
        NSError* error = nil;
        id<MTLComputePipelineState> pipeline = [ctx->device newComputePipelineStateWithFunction:function 
                                                                                          error:&error];
        if (!pipeline) {
            NSLog(@"Failed to create neighbor indices pipeline: %@", error);
            return -1;
        }
        
        // Create command buffer
        id<MTLCommandBuffer> commandBuffer = [ctx->commandQueue commandBuffer];
        if (!commandBuffer) return -1;
        
        // Create compute encoder
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
        if (!encoder) return -1;
        
        // Set pipeline
        [encoder setComputePipelineState:pipeline];
        
        // Set buffers
        id<MTLBuffer> neighborMTLBuffer = (__bridge id<MTLBuffer>)neighborBuffer;
        id<MTLBuffer> shellMTLBuffer = (__bridge id<MTLBuffer>)shellBuffer;
        [encoder setBuffer:neighborMTLBuffer offset:0 atIndex:0];
        [encoder setBuffer:shellMTLBuffer offset:0 atIndex:1];
        
        // Calculate thread groups
        NSUInteger threadsPerThreadgroup = MIN(pipeline.maxTotalThreadsPerThreadgroup, 256);
        NSUInteger threadgroupsPerGrid = (voxelCount + threadsPerThreadgroup - 1) / threadsPerThreadgroup;
        
        // Dispatch threads
        [encoder dispatchThreadgroups:MTLSizeMake(threadgroupsPerGrid, 1, 1) 
                threadsPerThreadgroup:MTLSizeMake(threadsPerThreadgroup, 1, 1)];
        
        // End encoding
        [encoder endEncoding];
        
        // Commit and wait
        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];
        
        return 0;
    }
}

int runTemperatureFastKernel(MetalContext* ctx, void* voxelBuffer, void* neighborBuffer, 
                            int voxelCount, float dt, float thermalDiffusivity) {
    @autoreleasepool {
        // Get the fast temperature function
        id<MTLFunction> function = [ctx->library newFunctionWithName:@"updateTemperatureFast"];
        if (!function) {
            NSLog(@"Failed to find updateTemperatureFast function");
            return -1;
        }
        
        // Create pipeline state
        NSError* error = nil;
        id<MTLComputePipelineState> pipeline = [ctx->device newComputePipelineStateWithFunction:function 
                                                                                          error:&error];
        if (!pipeline) {
            NSLog(@"Failed to create fast temperature pipeline: %@", error);
            return -1;
        }
        
        // Create command buffer
        id<MTLCommandBuffer> commandBuffer = [ctx->commandQueue commandBuffer];
        if (!commandBuffer) return -1;
        
        // Create compute encoder
        id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
        if (!encoder) return -1;
        
        // Set pipeline
        [encoder setComputePipelineState:pipeline];
        
        // Set buffers
        id<MTLBuffer> voxelMTLBuffer = (__bridge id<MTLBuffer>)voxelBuffer;
        id<MTLBuffer> neighborMTLBuffer = (__bridge id<MTLBuffer>)neighborBuffer;
        [encoder setBuffer:voxelMTLBuffer offset:0 atIndex:0];
        [encoder setBuffer:neighborMTLBuffer offset:0 atIndex:1];
        
        // Set constants
        [encoder setBytes:&dt length:sizeof(float) atIndex:2];
        [encoder setBytes:&thermalDiffusivity length:sizeof(float) atIndex:3];
        
        // Calculate thread groups
        NSUInteger threadsPerThreadgroup = MIN(pipeline.maxTotalThreadsPerThreadgroup, 256);
        NSUInteger threadgroupsPerGrid = (voxelCount + threadsPerThreadgroup - 1) / threadsPerThreadgroup;
        
        // Dispatch threads
        [encoder dispatchThreadgroups:MTLSizeMake(threadgroupsPerGrid, 1, 1) 
                threadsPerThreadgroup:MTLSizeMake(threadsPerThreadgroup, 1, 1)];
        
        // End encoding
        [encoder endEncoding];
        
        // Commit and wait
        [commandBuffer commit];
        [commandBuffer waitUntilCompleted];
        
        return 0;
    }
}
