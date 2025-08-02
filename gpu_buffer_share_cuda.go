// +build cuda

package main

/*
#cgo LDFLAGS: -lGL

#include <GL/gl.h>
#include <string.h>

// Since we're not using CUDA directly yet, we'll use OpenGL Persistent Mapped Buffers
// This allows CPU (Go physics) and GPU (OpenGL rendering) to share memory efficiently

typedef struct {
    GLuint buffer;
    void* mappedPtr;
    size_t size;
} PersistentBuffer;

PersistentBuffer* createPersistentBuffer(size_t size) {
    PersistentBuffer* pb = (PersistentBuffer*)malloc(sizeof(PersistentBuffer));
    if (!pb) return NULL;
    
    pb->size = size;
    
    // Generate buffer
    glGenBuffers(1, &pb->buffer);
    glBindBuffer(GL_SHADER_STORAGE_BUFFER, pb->buffer);
    
    // Allocate with persistent mapping flags
    GLbitfield flags = GL_MAP_WRITE_BIT | GL_MAP_PERSISTENT_BIT | GL_MAP_COHERENT_BIT;
    glBufferStorage(GL_SHADER_STORAGE_BUFFER, size, NULL, flags);
    
    // Map persistently
    pb->mappedPtr = glMapBufferRange(GL_SHADER_STORAGE_BUFFER, 0, size, flags);
    
    if (!pb->mappedPtr) {
        glDeleteBuffers(1, &pb->buffer);
        free(pb);
        return NULL;
    }
    
    return pb;
}

void releasePersistentBuffer(PersistentBuffer* pb) {
    if (!pb) return;
    
    if (pb->buffer) {
        glBindBuffer(GL_SHADER_STORAGE_BUFFER, pb->buffer);
        glUnmapBuffer(GL_SHADER_STORAGE_BUFFER);
        glDeleteBuffers(1, &pb->buffer);
    }
    
    free(pb);
}

void* getBufferPtr(PersistentBuffer* pb) {
    return pb ? pb->mappedPtr : NULL;
}

GLuint getBufferID(PersistentBuffer* pb) {
    return pb ? pb->buffer : 0;
}

// Memory barrier to ensure GPU sees CPU writes
void ensureGPUVisibility() {
    glMemoryBarrier(GL_CLIENT_MAPPED_BUFFER_BARRIER_BIT);
}
*/
import "C"

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
	"unsafe"
)

// PersistentBufferManager manages GPU buffers with CPU/GPU shared memory
// This is a simpler approach that works on Windows/Linux without CUDA
type PersistentBufferManager struct {
	voxelBuffer    *C.PersistentBuffer
	shellBuffer    *C.PersistentBuffer
	lonCountBuffer *C.PersistentBuffer
	
	voxelData      []GPUVoxelMaterial
	shellData      []ShellMetadataGPU
	lonCountData   []int32
	
	totalVoxels    int
	shellCount     int
}

// NewPersistentBufferManager creates CPU/GPU shared buffers using OpenGL 4.4+ persistent mapping
func NewPersistentBufferManager(planet *VoxelPlanet) (*PersistentBufferManager, error) {
	// Check OpenGL version for persistent mapping support
	var major, minor int32
	gl.GetIntegerv(gl.MAJOR_VERSION, &major)
	gl.GetIntegerv(gl.MINOR_VERSION, &minor)
	
	if major < 4 || (major == 4 && minor < 4) {
		// Fallback message - we'll use regular buffers
		fmt.Println("Warning: OpenGL 4.4+ required for zero-copy buffers, using regular buffers")
		return nil, fmt.Errorf("OpenGL 4.4+ required for persistent mapping")
	}
	
	// Count total voxels
	totalVoxels := 0
	totalLonCounts := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
		totalLonCounts += len(shell.LonCounts)
	}
	
	mgr := &PersistentBufferManager{
		totalVoxels: totalVoxels,
		shellCount:  len(planet.Shells),
	}
	
	// Create voxel buffer
	voxelSize := totalVoxels * int(unsafe.Sizeof(GPUVoxelMaterial{}))
	mgr.voxelBuffer = C.createPersistentBuffer(C.size_t(voxelSize))
	if mgr.voxelBuffer == nil {
		return nil, fmt.Errorf("failed to create persistent voxel buffer")
	}
	
	// Create shell metadata buffer
	shellSize := mgr.shellCount * int(unsafe.Sizeof(ShellMetadataGPU{}))
	mgr.shellBuffer = C.createPersistentBuffer(C.size_t(shellSize))
	if mgr.shellBuffer == nil {
		mgr.Release()
		return nil, fmt.Errorf("failed to create persistent shell buffer")
	}
	
	// Create longitude count buffer
	lonCountSize := totalLonCounts * 4 // 4 bytes per int32
	mgr.lonCountBuffer = C.createPersistentBuffer(C.size_t(lonCountSize))
	if mgr.lonCountBuffer == nil {
		mgr.Release()
		return nil, fmt.Errorf("failed to create persistent lon count buffer")
	}
	
	// Create slices that directly map to GPU memory
	mgr.voxelData = (*[1 << 30]GPUVoxelMaterial)(C.getBufferPtr(mgr.voxelBuffer))[:totalVoxels:totalVoxels]
	mgr.shellData = (*[1 << 20]ShellMetadataGPU)(C.getBufferPtr(mgr.shellBuffer))[:mgr.shellCount:mgr.shellCount]
	mgr.lonCountData = (*[1 << 20]int32)(C.getBufferPtr(mgr.lonCountBuffer))[:totalLonCounts:totalLonCounts]
	
	// Initialize shell metadata (only needs to be done once)
	mgr.UpdateShellMetadata(planet)
	
	return mgr, nil
}

// UpdateFromPlanet writes voxel data directly to GPU-mapped memory
func (mgr *PersistentBufferManager) UpdateFromPlanet(planet *VoxelPlanet) {
	// Write directly to GPU-visible memory
	idx := 0
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if idx < len(mgr.voxelData) {
					mgr.voxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
					idx++
				}
			}
		}
	}
	
	// Ensure GPU sees the writes
	C.ensureGPUVisibility()
}

// UpdateToPlanet reads voxel data from GPU-mapped memory back to planet
func (mgr *PersistentBufferManager) UpdateToPlanet(planet *VoxelPlanet) {
	idx := 0
	for _, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if idx < len(mgr.voxelData) {
					// Convert back from GPU format
					gpu := &mgr.voxelData[idx]
					voxel := &shell.Voxels[latIdx][lonIdx]
					
					voxel.Type = MaterialType(gpu.Type)
					voxel.Density = gpu.Density
					voxel.Temperature = gpu.Temperature
					voxel.Pressure = gpu.Pressure
					voxel.VelTheta = gpu.VelTheta
					voxel.VelPhi = gpu.VelPhi
					voxel.VelR = gpu.VelR
					voxel.Age = gpu.Age
					
					idx++
				}
			}
		}
	}
}

// UpdateShellMetadata updates shell metadata in GPU memory
func (mgr *PersistentBufferManager) UpdateShellMetadata(planet *VoxelPlanet) {
	voxelOffset := 0
	lonCountOffset := 0
	
	for i, shell := range planet.Shells {
		mgr.shellData[i] = ShellMetadataGPU{
			InnerRadius:    float32(shell.InnerRadius),
			OuterRadius:    float32(shell.OuterRadius),
			LatBands:       int32(shell.LatBands),
			VoxelOffset:    int32(voxelOffset),
			LonCountOffset: int32(lonCountOffset),
		}
		
		// Copy longitude counts
		for j, count := range shell.LonCounts {
			mgr.lonCountData[lonCountOffset+j] = int32(count)
			voxelOffset += count
		}
		lonCountOffset += len(shell.LonCounts)
	}
	
	C.ensureGPUVisibility()
}

// GetBufferIDs returns OpenGL buffer IDs for binding
func (mgr *PersistentBufferManager) GetBufferIDs() (voxel, shell, lonCount uint32) {
	return uint32(C.getBufferID(mgr.voxelBuffer)),
		   uint32(C.getBufferID(mgr.shellBuffer)),
		   uint32(C.getBufferID(mgr.lonCountBuffer))
}

// BindBuffers binds the buffers to SSBO binding points
func (mgr *PersistentBufferManager) BindBuffers() {
	voxelID, shellID, lonCountID := mgr.GetBufferIDs()
	
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, voxelID)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, shellID)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 2, lonCountID)
}

// GetVoxelData returns direct access to voxel data
func (mgr *PersistentBufferManager) GetVoxelData() []GPUVoxelMaterial {
	return mgr.voxelData
}

// SyncToGPU ensures GPU sees latest CPU writes
func (mgr *PersistentBufferManager) SyncToGPU() {
	C.ensureGPUVisibility()
}

// Release cleans up resources
func (mgr *PersistentBufferManager) Release() {
	if mgr.voxelBuffer != nil {
		C.releasePersistentBuffer(mgr.voxelBuffer)
		mgr.voxelBuffer = nil
	}
	
	if mgr.shellBuffer != nil {
		C.releasePersistentBuffer(mgr.shellBuffer)
		mgr.shellBuffer = nil
	}
	
	if mgr.lonCountBuffer != nil {
		C.releasePersistentBuffer(mgr.lonCountBuffer)
		mgr.lonCountBuffer = nil
	}
}
