//go:build !nometal
// +build !nometal

package main

import (
	"fmt"
	"log"
	"unsafe"
)

/*
#cgo LDFLAGS: -framework OpenCL
#include <OpenCL/opencl.h>
#include <stdlib.h>

const char* kernelSource = 
"__kernel void updateTectonics(__global float* vertices,\n"
"                             __global float* plates,\n"
"                             const float deltaTime,\n"
"                             const int numVertices) {\n"
"    int gid = get_global_id(0);\n"
"    if (gid >= numVertices) return;\n"
"    \n"
"    int vertexOffset = gid * 5; // pos(3) + height(1) + plateID(1)\n"
"    int plateID = (int)vertices[vertexOffset + 4];\n"
"    \n"
"    if (plateID < 0 || plateID >= 32) return;\n"
"    \n"
"    int plateOffset = plateID * 7; // center(3) + velocity(3) + type(1)\n"
"    \n"
"    float3 velocity = (float3)(plates[plateOffset + 3],\n"
"                              plates[plateOffset + 4],\n"
"                              plates[plateOffset + 5]);\n"
"    \n"
"    float velocityMag = length(velocity);\n"
"    float uplift = velocityMag * deltaTime * 0.00001f;\n"
"    \n"
"    int plateType = (int)plates[plateOffset + 6];\n"
"    \n"
"    if (plateType == 0) { // Continental\n"
"        vertices[vertexOffset + 3] += uplift;\n"
"    } else { // Oceanic\n"
"        vertices[vertexOffset + 3] -= uplift * 0.5f;\n"
"    }\n"
"    \n"
"    // Clamp heights\n"
"    vertices[vertexOffset + 3] = clamp(vertices[vertexOffset + 3], -0.02f, 0.02f);\n"
"}\n";

typedef struct {
    cl_context context;
    cl_command_queue queue;
    cl_program program;
    cl_kernel kernel;
    cl_mem vertexBuffer;
    cl_mem plateBuffer;
    cl_device_id device;
} OpenCLCompute;

OpenCLCompute* initOpenCL() {
    OpenCLCompute* compute = malloc(sizeof(OpenCLCompute));
    cl_int err;
    
    // Get platform
    cl_platform_id platform;
    err = clGetPlatformIDs(1, &platform, NULL);
    if (err != CL_SUCCESS) {
        free(compute);
        return NULL;
    }
    
    // Get device
    err = clGetDeviceIDs(platform, CL_DEVICE_TYPE_GPU, 1, &compute->device, NULL);
    if (err != CL_SUCCESS) {
        // Try CPU if GPU fails
        err = clGetDeviceIDs(platform, CL_DEVICE_TYPE_CPU, 1, &compute->device, NULL);
        if (err != CL_SUCCESS) {
            free(compute);
            return NULL;
        }
    }
    
    // Create context
    compute->context = clCreateContext(NULL, 1, &compute->device, NULL, NULL, &err);
    if (err != CL_SUCCESS) {
        free(compute);
        return NULL;
    }
    
    // Create command queue
    compute->queue = clCreateCommandQueue(compute->context, compute->device, 0, &err);
    if (err != CL_SUCCESS) {
        clReleaseContext(compute->context);
        free(compute);
        return NULL;
    }
    
    // Create program
    compute->program = clCreateProgramWithSource(compute->context, 1, &kernelSource, NULL, &err);
    if (err != CL_SUCCESS) {
        clReleaseCommandQueue(compute->queue);
        clReleaseContext(compute->context);
        free(compute);
        return NULL;
    }
    
    // Build program
    err = clBuildProgram(compute->program, 1, &compute->device, NULL, NULL, NULL);
    if (err != CL_SUCCESS) {
        clReleaseProgram(compute->program);
        clReleaseCommandQueue(compute->queue);
        clReleaseContext(compute->context);
        free(compute);
        return NULL;
    }
    
    // Create kernel
    compute->kernel = clCreateKernel(compute->program, "updateTectonics", &err);
    if (err != CL_SUCCESS) {
        clReleaseProgram(compute->program);
        clReleaseCommandQueue(compute->queue);
        clReleaseContext(compute->context);
        free(compute);
        return NULL;
    }
    
    return compute;
}

int runOpenCLCompute(OpenCLCompute* compute, float* vertexData, float* plateData, int numVertices) {
    if (!compute) return 0;
    
    cl_int err;
    size_t vertexSize = numVertices * 5 * sizeof(float);
    size_t plateSize = 32 * 7 * sizeof(float);
    
    // Create buffers
    compute->vertexBuffer = clCreateBuffer(compute->context, CL_MEM_READ_WRITE | CL_MEM_COPY_HOST_PTR,
                                          vertexSize, vertexData, &err);
    if (err != CL_SUCCESS) return 0;
    
    compute->plateBuffer = clCreateBuffer(compute->context, CL_MEM_READ_ONLY | CL_MEM_COPY_HOST_PTR,
                                         plateSize, plateData, &err);
    if (err != CL_SUCCESS) {
        clReleaseMemObject(compute->vertexBuffer);
        return 0;
    }
    
    // Set kernel arguments
    float deltaTime = 1000.0f;
    clSetKernelArg(compute->kernel, 0, sizeof(cl_mem), &compute->vertexBuffer);
    clSetKernelArg(compute->kernel, 1, sizeof(cl_mem), &compute->plateBuffer);
    clSetKernelArg(compute->kernel, 2, sizeof(float), &deltaTime);
    clSetKernelArg(compute->kernel, 3, sizeof(int), &numVertices);
    
    // Execute kernel
    size_t globalSize = numVertices;
    size_t localSize = 256;
    if (globalSize % localSize != 0) {
        globalSize = ((globalSize / localSize) + 1) * localSize;
    }
    
    err = clEnqueueNDRangeKernel(compute->queue, compute->kernel, 1, NULL, &globalSize, &localSize, 0, NULL, NULL);
    if (err != CL_SUCCESS) {
        clReleaseMemObject(compute->vertexBuffer);
        clReleaseMemObject(compute->plateBuffer);
        return 0;
    }
    
    // Read results back
    err = clEnqueueReadBuffer(compute->queue, compute->vertexBuffer, CL_TRUE, 0, vertexSize, vertexData, 0, NULL, NULL);
    if (err != CL_SUCCESS) {
        clReleaseMemObject(compute->vertexBuffer);
        clReleaseMemObject(compute->plateBuffer);
        return 0;
    }
    
    clFinish(compute->queue);
    
    // Cleanup buffers
    clReleaseMemObject(compute->vertexBuffer);
    clReleaseMemObject(compute->plateBuffer);
    
    return 1;
}

void cleanupOpenCL(OpenCLCompute* compute) {
    if (compute) {
        clReleaseKernel(compute->kernel);
        clReleaseProgram(compute->program);
        clReleaseCommandQueue(compute->queue);
        clReleaseContext(compute->context);
        free(compute);
    }
}
*/
import "C"

type OpenCLGPU struct {
	compute *C.OpenCLCompute
	enabled bool
}

var openclGPU *OpenCLGPU

func initOpenCLGPU() *OpenCLGPU {
	compute := C.initOpenCL()
	if compute == nil {
		fmt.Println("OpenCL initialization failed, using CPU fallback")
		return &OpenCLGPU{enabled: false}
	}
	
	fmt.Println("OpenCL GPU compute initialized successfully")
	return &OpenCLGPU{
		compute: compute,
		enabled: true,
	}
}

func updateTectonicsOpenCL(planet Planet, deltaYears float64) Planet {
	if !openclGPU.enabled || len(planet.Vertices) == 0 || len(planet.Plates) == 0 {
		return updateTectonics(planet, deltaYears) // CPU fallback
	}
	
	// Prepare vertex data: [x, y, z, height, plateID] per vertex
	vertexData := make([]float32, len(planet.Vertices)*5)
	for i, v := range planet.Vertices {
		vertexData[i*5+0] = float32(v.Position.X)
		vertexData[i*5+1] = float32(v.Position.Y)
		vertexData[i*5+2] = float32(v.Position.Z)
		vertexData[i*5+3] = float32(v.Height)
		vertexData[i*5+4] = float32(v.PlateID)
	}
	
	// Prepare plate data: [centerX, centerY, centerZ, velX, velY, velZ, type] per plate
	plateData := make([]float32, 32*7) // Max 32 plates
	for i, p := range planet.Plates {
		if i >= 32 { break }
		plateData[i*7+0] = float32(p.Center.X)
		plateData[i*7+1] = float32(p.Center.Y)
		plateData[i*7+2] = float32(p.Center.Z)
		plateData[i*7+3] = float32(p.Velocity.X)
		plateData[i*7+4] = float32(p.Velocity.Y)
		plateData[i*7+5] = float32(p.Velocity.Z)
		if p.Type == Continental {
			plateData[i*7+6] = 0
		} else {
			plateData[i*7+6] = 1
		}
	}
	
	// Run OpenCL kernel
	success := C.runOpenCLCompute(
		openclGPU.compute,
		(*C.float)(unsafe.Pointer(&vertexData[0])),
		(*C.float)(unsafe.Pointer(&plateData[0])),
		C.int(len(planet.Vertices)),
	)
	
	if success == 0 {
		log.Println("OpenCL compute failed, falling back to CPU")
		return updateTectonics(planet, deltaYears)
	}
	
	// Copy results back to planet
	for i := range planet.Vertices {
		planet.Vertices[i].Height = float64(vertexData[i*5+3])
		// Keep vertex position normalized - height affects rendering only
		planet.Vertices[i].Position = planet.Vertices[i].Position.Normalize()
	}
	
	planet.GeologicalTime += deltaYears
	return planet
}

func init() {
	openclGPU = initOpenCLGPU()
}