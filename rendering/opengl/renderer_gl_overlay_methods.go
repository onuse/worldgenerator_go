package opengl

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// RenderFullscreenStats renders statistics using a fullscreen shader approach
func (r *VoxelRenderer) RenderFullscreenStats() {
	if r.statsOverlay == nil {
		fmt.Println("WARNING: statsOverlay is nil in RenderFullscreenStats")
		return
	}

	// Disable depth testing for overlay
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Render the stats overlay
	r.statsOverlay.Render()

	// Restore state
	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.BLEND)
}

// RenderStatsText renders stats using texture-based text (placeholder)
func (r *VoxelRenderer) RenderStatsText() {
	// This would use a proper text rendering system
	// For now, it's a placeholder
	fmt.Printf("\rFPS: %.1f | Zoom: %.3f | Distance: %.0f km", 
		60.0, r.GetCameraDistance()/r.planetRadius, r.GetCameraDistance()/1000)
}

// RenderSimpleOverlay renders a simple colored overlay
func (r *VoxelRenderer) RenderSimpleOverlay() {
	// Simple overlay rendering logic
	// This is a placeholder implementation
}