//go:build windows || linux
// +build windows linux

package gpu

import (
	"fmt"
	"unsafe"
	"worldgenerator/core"

	"github.com/go-gl/gl/v4.3-core/gl"
)

// WindowsGPUBufferManager provides efficient CPU-GPU data sharing on Windows/Linux
// Since physics runs on CPU on these platforms, we optimize the data transfer
type WindowsGPUBufferManager struct {
	// OpenGL buffer objects
	voxelSSBO    uint32
	shellSSBO    uint32
	lonCountSSBO uint32

	// CPU-side data that gets uploaded to GPU
	voxelData    []GPUVoxelMaterial
	shellData    []SphericalShellMetadata
	lonCountData []int32

	// Metadata
	totalVoxels    int
	shellCount     int
	totalLonCounts int

	// Track if data needs GPU update
	voxelsDirty bool

	// For OpenGL 4.4+ persistent mapping (if available)
	UsePersistent bool
	mappedVoxels  unsafe.Pointer
}

// NewWindowsGPUBufferManager creates an optimized buffer manager for Windows/Linux
func NewWindowsGPUBufferManager(planet *core.VoxelPlanet) (*WindowsGPUBufferManager, error) {
	// Count totals
	totalVoxels := 0
	totalLonCounts := 0
	for _, shell := range planet.Shells {
		for _, count := range shell.LonCounts {
			totalVoxels += count
		}
		totalLonCounts += len(shell.LonCounts)
	}

	mgr := &WindowsGPUBufferManager{
		totalVoxels:    totalVoxels,
		shellCount:     len(planet.Shells),
		totalLonCounts: totalLonCounts,
		voxelsDirty:    true,
	}

	// Allocate CPU-side arrays
	mgr.voxelData = make([]GPUVoxelMaterial, totalVoxels)
	mgr.shellData = make([]SphericalShellMetadata, mgr.shellCount)
	mgr.lonCountData = make([]int32, totalLonCounts)

	// Create OpenGL buffers
	gl.GenBuffers(1, &mgr.voxelSSBO)
	gl.GenBuffers(1, &mgr.shellSSBO)
	gl.GenBuffers(1, &mgr.lonCountSSBO)

	// Check for persistent mapping support (OpenGL 4.4+)
	var major, minor int32
	gl.GetIntegerv(gl.MAJOR_VERSION, &major)
	gl.GetIntegerv(gl.MINOR_VERSION, &minor)
	mgr.UsePersistent = (major > 4 || (major == 4 && minor >= 4))

	if mgr.UsePersistent {
		fmt.Println("Using OpenGL 4.4+ persistent mapped buffers for zero-copy transfer")
		mgr.createPersistentBuffers()
	} else {
		fmt.Println("Using standard OpenGL buffers with orphaning for efficient transfer")
		mgr.createStandardBuffers()
	}

	// Initialize shell metadata (doesn't change during simulation)
	mgr.updateShellMetadata(planet)

	return mgr, nil
}

// createPersistentBuffers creates buffers with persistent mapping for zero-copy
func (mgr *WindowsGPUBufferManager) createPersistentBuffers() {
	// Voxel buffer with persistent + coherent mapping
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.voxelSSBO)
	voxelSize := mgr.totalVoxels * int(unsafe.Sizeof(GPUVoxelMaterial{}))
	flags := uint32(gl.MAP_WRITE_BIT | gl.MAP_PERSISTENT_BIT | gl.MAP_COHERENT_BIT)
	gl.BufferStorage(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, flags)

	// Map the buffer persistently
	mgr.mappedVoxels = gl.MapBufferRange(gl.SHADER_STORAGE_BUFFER, 0, voxelSize, flags)

	// Shell metadata buffer (static data)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.shellSSBO)
	shellSize := mgr.shellCount * int(unsafe.Sizeof(SphericalShellMetadata{}))
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, shellSize, unsafe.Pointer(&mgr.shellData[0]), gl.STATIC_DRAW)

	// Longitude count buffer (static data)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.lonCountSSBO)
	lonSize := mgr.totalLonCounts * 4
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, lonSize, unsafe.Pointer(&mgr.lonCountData[0]), gl.STATIC_DRAW)
}

// createStandardBuffers creates regular buffers for older OpenGL
func (mgr *WindowsGPUBufferManager) createStandardBuffers() {
	// Voxel buffer
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.voxelSSBO)
	voxelSize := mgr.totalVoxels * int(unsafe.Sizeof(GPUVoxelMaterial{}))
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, gl.DYNAMIC_DRAW)

	// Shell metadata buffer
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.shellSSBO)
	shellSize := mgr.shellCount * int(unsafe.Sizeof(SphericalShellMetadata{}))
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, shellSize, unsafe.Pointer(&mgr.shellData[0]), gl.STATIC_DRAW)

	// Longitude count buffer
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.lonCountSSBO)
	lonSize := mgr.totalLonCounts * 4
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, lonSize, unsafe.Pointer(&mgr.lonCountData[0]), gl.STATIC_DRAW)
}

// UpdateFromPlanet updates GPU buffers from planet data
func (mgr *WindowsGPUBufferManager) UpdateFromPlanet(planet *core.VoxelPlanet) {
	if mgr.UsePersistent && mgr.mappedVoxels != nil {
		// Direct write to mapped memory
		mapped := (*[1 << 30]GPUVoxelMaterial)(mgr.mappedVoxels)[:mgr.totalVoxels:mgr.totalVoxels]
		idx := 0
		for _, shell := range planet.Shells {
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					if idx < mgr.totalVoxels {
						mapped[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
						idx++
					}
				}
			}
		}
		// Memory barrier for coherent mapping
		gl.MemoryBarrier(gl.CLIENT_MAPPED_BUFFER_BARRIER_BIT)
	} else {
		// Copy to CPU array first
		idx := 0
		for _, shell := range planet.Shells {
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					if idx < mgr.totalVoxels {
						mgr.voxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
						idx++
					}
				}
			}
		}
		mgr.voxelsDirty = true
	}
}

// UpdateToPlanet reads data back from GPU buffers to planet
func (mgr *WindowsGPUBufferManager) UpdateToPlanet(planet *core.VoxelPlanet) {
	var data []GPUVoxelMaterial

	if mgr.UsePersistent && mgr.mappedVoxels != nil {
		// Read directly from mapped memory
		data = (*[1 << 30]GPUVoxelMaterial)(mgr.mappedVoxels)[:mgr.totalVoxels:mgr.totalVoxels]
	} else {
		// Read back from GPU
		gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.voxelSSBO)
		ptr := gl.MapBuffer(gl.SHADER_STORAGE_BUFFER, gl.READ_ONLY)
		if ptr != nil {
			data = (*[1 << 30]GPUVoxelMaterial)(ptr)[:mgr.totalVoxels:mgr.totalVoxels]
			defer gl.UnmapBuffer(gl.SHADER_STORAGE_BUFFER)
		}
	}

	if data != nil {
		idx := 0
		for _, shell := range planet.Shells {
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					if idx < mgr.totalVoxels {
						gpu := &data[idx]
						voxel := &shell.Voxels[latIdx][lonIdx]

						voxel.Type = core.MaterialType(gpu.Type)
						voxel.Density = gpu.Density
						voxel.Temperature = gpu.Temperature
						voxel.Pressure = gpu.Pressure
						voxel.VelNorth = gpu.VelNorth
						voxel.VelEast = gpu.VelEast
						voxel.VelR = gpu.VelR
						voxel.Age = gpu.Age

						idx++
					}
				}
			}
		}
	}
}

// updateShellMetadata fills shell metadata arrays
func (mgr *WindowsGPUBufferManager) updateShellMetadata(planet *core.VoxelPlanet) {
	voxelOffset := 0
	lonCountOffset := 0

	for i, shell := range planet.Shells {
		mgr.shellData[i] = SphericalShellMetadata{
			InnerRadius: float32(shell.InnerRadius),
			OuterRadius: float32(shell.OuterRadius),
			LatBands:    int32(shell.LatBands),
			VoxelOffset: int32(voxelOffset),
		}

		// Copy longitude counts
		for j, count := range shell.LonCounts {
			mgr.lonCountData[lonCountOffset+j] = int32(count)
			voxelOffset += count
		}
		lonCountOffset += len(shell.LonCounts)
	}
}

// SyncToGPU uploads any dirty data to GPU
func (mgr *WindowsGPUBufferManager) SyncToGPU() {
	if !mgr.UsePersistent && mgr.voxelsDirty {
		// GPU sync completed
		// Upload voxel data using buffer orphaning for efficiency
		gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.voxelSSBO)
		voxelSize := mgr.totalVoxels * int(unsafe.Sizeof(GPUVoxelMaterial{}))

		// Orphan the old buffer to avoid synchronization
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, gl.DYNAMIC_DRAW)

		// Upload new data
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, voxelSize, unsafe.Pointer(&mgr.voxelData[0]))

		mgr.voxelsDirty = false
	}
}

// BindBuffers binds all SSBOs to their binding points
func (mgr *WindowsGPUBufferManager) BindBuffers() {
	// Make sure data is uploaded
	mgr.SyncToGPU()

	// Bind to SSBO binding points
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, mgr.voxelSSBO)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, mgr.shellSSBO)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 2, mgr.lonCountSSBO)
}

// GetBufferIDs returns the OpenGL buffer IDs
func (mgr *WindowsGPUBufferManager) GetBufferIDs() (voxel, shell, lonCount uint32) {
	return mgr.voxelSSBO, mgr.shellSSBO, mgr.lonCountSSBO
}

// Release cleans up all resources
func (mgr *WindowsGPUBufferManager) Release() {
	if mgr.UsePersistent && mgr.mappedVoxels != nil {
		gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, mgr.voxelSSBO)
		gl.UnmapBuffer(gl.SHADER_STORAGE_BUFFER)
	}

	if mgr.voxelSSBO != 0 {
		gl.DeleteBuffers(1, &mgr.voxelSSBO)
	}
	if mgr.shellSSBO != 0 {
		gl.DeleteBuffers(1, &mgr.shellSSBO)
	}
	if mgr.lonCountSSBO != 0 {
		gl.DeleteBuffers(1, &mgr.lonCountSSBO)
	}
}
