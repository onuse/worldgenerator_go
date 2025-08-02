// +build windows linux

package gpu

import (
	"github.com/go-gl/gl/v4.3-core/gl"
	
	"worldgenerator/core"
	"worldgenerator/simulation"
)

// UpdateBuffersWithPlateData updates GPU buffers including plate information
func (mgr *WindowsGPUBufferManager) UpdateFromPlanetWithPlates(planet *core.VoxelPlanet, plateManager *simulation.PlateManager) {
	if mgr.UsePersistent && mgr.mappedVoxels != nil {
		// Direct write to mapped memory
		mapped := (*[1 << 30]GPUVoxelMaterial)(mgr.mappedVoxels)[:mgr.totalVoxels:mgr.totalVoxels]
		idx := 0
		for shellIdx, shell := range planet.Shells {
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					if idx < mgr.totalVoxels {
						// Basic voxel data
						mapped[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
						
						// Add plate information
						coord := core.VoxelCoord{Shell: shellIdx, Lat: latIdx, Lon: lonIdx}
						if plateID, exists := plateManager.VoxelPlateMap[coord]; exists {
							mapped[idx].PlateID = int32(plateID)
							
							// Check if boundary using the efficient map
							if plateManager.BoundaryMap != nil {
								if plateManager.BoundaryMap[coord] {
									mapped[idx].IsBoundary = 1
								} else {
									mapped[idx].IsBoundary = 0
								}
							}
						} else {
							mapped[idx].PlateID = 0  // No plate
							mapped[idx].IsBoundary = 0
						}
						
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
		for shellIdx, shell := range planet.Shells {
			for latIdx := range shell.Voxels {
				for lonIdx := range shell.Voxels[latIdx] {
					if idx < mgr.totalVoxels {
						mgr.voxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
						
						// Add plate information
						coord := core.VoxelCoord{Shell: shellIdx, Lat: latIdx, Lon: lonIdx}
						if plateID, exists := plateManager.VoxelPlateMap[coord]; exists {
							mgr.voxelData[idx].PlateID = int32(plateID)
							
							// Check if boundary using the efficient map
							if plateManager.BoundaryMap != nil {
								if plateManager.BoundaryMap[coord] {
									mgr.voxelData[idx].IsBoundary = 1
								} else {
									mgr.voxelData[idx].IsBoundary = 0
								}
							}
						} else {
							mgr.voxelData[idx].PlateID = 0
							mgr.voxelData[idx].IsBoundary = 0
						}
						
						idx++
					}
				}
			}
		}
		mgr.voxelsDirty = true
	}
}

// UpdateSharedBuffersWithPlates updates the simple shared buffers with plate data
func UpdateSharedBuffersWithPlates(buffers *SharedGPUBuffers, planet *core.VoxelPlanet, plateManager *simulation.PlateManager) {
	idx := 0
	for shellIdx, shell := range planet.Shells {
		for latIdx := range shell.Voxels {
			for lonIdx := range shell.Voxels[latIdx] {
				if idx < len(buffers.VoxelData) {
					buffers.VoxelData[idx] = ConvertToGPUVoxel(&shell.Voxels[latIdx][lonIdx])
					
					// Add plate information
					coord := core.VoxelCoord{Shell: shellIdx, Lat: latIdx, Lon: lonIdx}
					if plateID, exists := plateManager.VoxelPlateMap[coord]; exists {
						buffers.VoxelData[idx].PlateID = int32(plateID)
						
						// Check if boundary using the efficient map
						if plateManager.BoundaryMap != nil {
							if plateManager.BoundaryMap[coord] {
								buffers.VoxelData[idx].IsBoundary = 1
							} else {
								buffers.VoxelData[idx].IsBoundary = 0
							}
						}
					} else {
						buffers.VoxelData[idx].PlateID = 0
						buffers.VoxelData[idx].IsBoundary = 0
					}
					
					idx++
				}
			}
		}
	}
}
