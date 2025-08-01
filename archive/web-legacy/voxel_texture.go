package main

import (
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"bytes"
)

// TextureData represents voxel data as textures for GPU rendering
type TextureData struct {
	Type           string  `json:"type"`
	Width          int     `json:"width"`  // Longitude resolution
	Height         int     `json:"height"` // Latitude resolution
	MaterialData   string  `json:"materialData"`   // Base64 encoded material types
	HeightData     string  `json:"heightData"`     // Base64 encoded heights
	AgeData        string  `json:"ageData"`        // Base64 encoded ages
	VelocityData   string  `json:"velocityData"`   // Base64 encoded velocities
	Time           float64 `json:"time"`
	TimeSpeed      float64 `json:"timeSpeed"`
}

// CreateTextureData converts planet surface to texture format
func (p *VoxelPlanet) CreateTextureData() TextureData {
	// Use higher resolution for better detail
	// Match the surface shell resolution
	surfaceShell := &p.Shells[len(p.Shells)-2] // Second to last is surface
	width := 720   // 0.5 degree longitude resolution
	height := surfaceShell.LatBands * 2 // Double the voxel resolution for smoother visuals
	
	// Create data arrays
	materials := make([]byte, width*height*4)  // RGBA
	heights := make([]byte, width*height*4)    // RGBA for float packing
	ages := make([]byte, width*height*4)       // RGBA for float packing
	velocities := make([]byte, width*height*4) // RGBA for velocity components
	
	// Sample the planet surface with smoothing
	for y := 0; y < height; y++ {
		lat := (float64(y)/float64(height) - 0.5) * 180.0 // -90 to +90
		
		for x := 0; x < width; x++ {
			lon := (float64(x)/float64(width) - 0.5) * 360.0 // -180 to +180
			
			// Find surface voxel at this lat/lon
			// Use the actual surface shell (not atmosphere)
			surfaceShellIdx := len(p.Shells) - 2
			if surfaceShellIdx < 0 {
				continue
			}
			surfaceShell := &p.Shells[surfaceShellIdx]
			
			// Use smooth sampling for less angular appearance
			materialType, heightValue, ageValue, velPhiValue := sampleVoxelSmooth(surfaceShell, lat, lon)
			idx := (y*width + x) * 4
			
			// Determine actual surface material
			// If it's basalt (ocean floor), show water instead
			if materialType == MatBasalt {
				// Ocean floor should show as water
				materialType = MatWater
			}
			
			// Material type in R channel
			materials[idx] = byte(materialType)
			materials[idx+1] = 0
			materials[idx+2] = 0
			materials[idx+3] = 255
			
			// Height already computed by smooth sampling
			packFloat32(heights[idx:idx+4], heightValue)
			
			// Age encoded in RGBA (normalized to 0-1 over 500My)
			ageNorm := ageValue / 500000000.0
			if ageNorm > 1.0 { ageNorm = 1.0 }
			packFloat32(ages[idx:idx+4], ageNorm)
			
			// Velocity in R,G channels (normalized)
			velLon := velPhiValue * 1e9  // Convert to cm/year
			velocities[idx] = byte((velLon + 50.0) * 255.0 / 100.0)   // -50 to +50 cm/yr
			velocities[idx+1] = 0 // No latitude velocity for now
			velocities[idx+2] = 0
			velocities[idx+3] = 255
		}
	}
	
	// Get current simulation speed
	simSpeedMutex.RLock()
	currentSpeed := simSpeed
	simSpeedMutex.RUnlock()
	
	return TextureData{
		Type:         "texture_update",
		Width:        width,
		Height:       height,
		MaterialData: encodeImage(materials, width, height),
		HeightData:   encodeImage(heights, width, height),
		AgeData:      encodeImage(ages, width, height),
		VelocityData: encodeImage(velocities, width, height),
		Time:         p.Time,
		TimeSpeed:    currentSpeed,
	}
}

// packFloat32 packs a float32 into 4 bytes (RGBA)
func packFloat32(bytes []byte, value float32) {
	// Simple packing - could be improved with better precision
	normalized := (value + 1.0) * 0.5 // Map to 0-1
	if normalized < 0 { normalized = 0 }
	if normalized > 1 { normalized = 1 }
	
	// Split across RGBA channels for precision
	v := uint32(normalized * 16777215) // 24-bit precision
	bytes[0] = byte(v & 0xFF)
	bytes[1] = byte((v >> 8) & 0xFF)
	bytes[2] = byte((v >> 16) & 0xFF)
	bytes[3] = 255
}

// encodeImage converts raw bytes to base64 PNG
func encodeImage(data []byte, width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			img.Set(x, y, color.RGBA{
				R: data[idx],
				G: data[idx+1],
				B: data[idx+2],
				A: data[idx+3],
			})
		}
	}
	
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}