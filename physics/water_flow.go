package physics

import (
	"math"
	"worldgenerator/core"
)

// WaterFlow handles realistic water movement physics
type WaterFlow struct {
	planet *core.VoxelPlanet
	
	// Flow parameters
	gravity        float32 // m/s^2
	viscosity      float32 // Water viscosity
	minFlowVolume  float32 // Minimum water volume to flow
	maxFlowRate    float32 // Maximum flow rate per timestep
}

// NewWaterFlow creates a new water flow physics system
func NewWaterFlow(planet *core.VoxelPlanet) *WaterFlow {
	return &WaterFlow{
		planet:        planet,
		gravity:       9.81,
		viscosity:     0.001,     // Water viscosity at 20°C
		minFlowVolume: 0.01,      // 1% minimum to flow
		maxFlowRate:   0.25,      // Max 25% of water flows per timestep
	}
}

// UpdateFlow calculates and applies water flow for one timestep
func (wf *WaterFlow) UpdateFlow(dt float32) {
	// Process each surface shell
	if len(wf.planet.Shells) < 2 {
		return
	}
	
	surfaceShell := len(wf.planet.Shells) - 2
	shell := &wf.planet.Shells[surfaceShell]
	
	// Calculate flow rates between cells
	flows := wf.calculateFlows(shell, dt)
	
	// Apply flows to update water volumes
	wf.applyFlows(shell, flows)
	
	// Update material types based on water volume
	wf.updateMaterialTypes(shell)
}

// flowData represents water flow between cells
type flowData struct {
	fromLat, fromLon int
	toLat, toLon     int
	volume           float32
}

// calculateFlows determines water flow between adjacent cells
func (wf *WaterFlow) calculateFlows(shell *core.SphericalShell, dt float32) []flowData {
	flows := make([]flowData, 0)
	
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Only flow if we have enough water
			if voxel.WaterVolume < wf.minFlowVolume {
				continue
			}
			
			// Check all neighbors for downhill flow
			neighbors := wf.getNeighbors(shell, latIdx, lonIdx)
			
			for _, n := range neighbors {
				neighborVoxel := &shell.Voxels[n.lat][n.lon]
				
				// Calculate height difference (including water surface)
				myHeight := voxel.Elevation + voxel.WaterVolume*100 // 100m per full cell
				neighborHeight := neighborVoxel.Elevation + neighborVoxel.WaterVolume*100
				
				// Water flows downhill
				if myHeight > neighborHeight {
					// Calculate flow rate based on height difference
					heightDiff := myHeight - neighborHeight
					flowRate := wf.calculateFlowRate(heightDiff, n.distance, dt)
					
					// Limit flow to available water and max rate
					maxFlow := voxel.WaterVolume * wf.maxFlowRate
					flowVolume := math.Min(float64(flowRate), float64(maxFlow))
					
					if flowVolume > 0 {
						flows = append(flows, flowData{
							fromLat: latIdx,
							fromLon: lonIdx,
							toLat:   n.lat,
							toLon:   n.lon,
							volume:  float32(flowVolume),
						})
					}
				}
			}
		}
	}
	
	return flows
}

// neighbor represents an adjacent cell
type neighbor struct {
	lat, lon int
	distance float32 // Distance between cell centers
}

// getNeighbors returns all adjacent cells
func (wf *WaterFlow) getNeighbors(shell *core.SphericalShell, latIdx, lonIdx int) []neighbor {
	neighbors := make([]neighbor, 0, 8)
	
	// Calculate cell size for distance
	radius := float32((shell.InnerRadius + shell.OuterRadius) / 2)
	latSize := float32(math.Pi) * radius / float32(shell.LatBands)
	
	// North/South neighbors
	if latIdx > 0 {
		neighbors = append(neighbors, neighbor{
			lat:      latIdx - 1,
			lon:      lonIdx,
			distance: latSize,
		})
	}
	if latIdx < shell.LatBands-1 {
		neighbors = append(neighbors, neighbor{
			lat:      latIdx + 1,
			lon:      lonIdx,
			distance: latSize,
		})
	}
	
	// East/West neighbors
	lonCount := shell.LonCounts[latIdx]
	lat := core.GetLatitudeForBand(latIdx, shell.LatBands)
	lonSize := float32(2*math.Pi*float64(radius)*math.Cos(lat*math.Pi/180)) / float32(lonCount)
	
	// West
	westLon := (lonIdx - 1 + lonCount) % lonCount
	neighbors = append(neighbors, neighbor{
		lat:      latIdx,
		lon:      westLon,
		distance: lonSize,
	})
	
	// East
	eastLon := (lonIdx + 1) % lonCount
	neighbors = append(neighbors, neighbor{
		lat:      latIdx,
		lon:      eastLon,
		distance: lonSize,
	})
	
	// Diagonal neighbors (simplified - approximate distance)
	diagDist := float32(math.Sqrt(float64(latSize*latSize + lonSize*lonSize)))
	
	// NW, NE
	if latIdx > 0 {
		northLonCount := shell.LonCounts[latIdx-1]
		// Map longitude to north latitude band
		northLon := lonIdx * northLonCount / lonCount
		
		neighbors = append(neighbors, neighbor{
			lat:      latIdx - 1,
			lon:      (northLon - 1 + northLonCount) % northLonCount,
			distance: diagDist,
		})
		neighbors = append(neighbors, neighbor{
			lat:      latIdx - 1,
			lon:      (northLon + 1) % northLonCount,
			distance: diagDist,
		})
	}
	
	// SW, SE
	if latIdx < shell.LatBands-1 {
		southLonCount := shell.LonCounts[latIdx+1]
		// Map longitude to south latitude band
		southLon := lonIdx * southLonCount / lonCount
		
		neighbors = append(neighbors, neighbor{
			lat:      latIdx + 1,
			lon:      (southLon - 1 + southLonCount) % southLonCount,
			distance: diagDist,
		})
		neighbors = append(neighbors, neighbor{
			lat:      latIdx + 1,
			lon:      (southLon + 1) % southLonCount,
			distance: diagDist,
		})
	}
	
	return neighbors
}

// calculateFlowRate uses simplified hydraulics to determine flow speed
func (wf *WaterFlow) calculateFlowRate(heightDiff, distance, dt float32) float32 {
	// Simplified: flow rate proportional to sqrt(height difference)
	// This approximates Torricelli's law for flow through an orifice
	velocity := float32(math.Sqrt(float64(2 * wf.gravity * heightDiff)))
	
	// Reduce by distance (longer distance = slower flow)
	velocity = velocity * (1.0 / (1.0 + distance/1000.0))
	
	// Convert to volume fraction
	flowRate := velocity * dt / distance
	
	return flowRate
}

// applyFlows updates water volumes based on calculated flows
func (wf *WaterFlow) applyFlows(shell *core.SphericalShell, flows []flowData) {
	// Apply all flows
	for _, flow := range flows {
		fromVoxel := &shell.Voxels[flow.fromLat][flow.fromLon]
		toVoxel := &shell.Voxels[flow.toLat][flow.toLon]
		
		// Transfer water
		fromVoxel.WaterVolume -= flow.volume
		toVoxel.WaterVolume += flow.volume
		
		// Clamp values
		if fromVoxel.WaterVolume < 0 {
			fromVoxel.WaterVolume = 0
		}
		if toVoxel.WaterVolume > 1 {
			toVoxel.WaterVolume = 1
		}
	}
}

// updateMaterialTypes changes voxel materials based on water presence
func (wf *WaterFlow) updateMaterialTypes(shell *core.SphericalShell) {
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Update material type based on water volume
			switch voxel.Type {
			case core.MatWater:
				// If water drained away, expose sediment
				if voxel.WaterVolume < 0.1 {
					voxel.Type = core.MatSediment
				}
				
			case core.MatGranite, core.MatBasalt, core.MatSediment:
				// If land is flooded, it becomes water
				if voxel.WaterVolume > 0.5 {
					voxel.Type = core.MatWater
				}
			}
		}
	}
}