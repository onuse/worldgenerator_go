package overlay

import (
	"fmt"
	"strings"

	"github.com/go-gl/gl/v4.3-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Simple stats overlay using OpenGL immediate mode style rendering

const statsVertexShader = `
#version 430 core

layout (location = 0) in vec2 position;
layout (location = 1) in vec4 color;

out vec4 fragColor;

uniform mat4 projection;

void main() {
    gl_Position = projection * vec4(position, 0.0, 1.0);
    fragColor = color;
}
`

const statsFragmentShader = `
#version 430 core

in vec4 fragColor;
out vec4 outColor;

void main() {
    outColor = fragColor;
}
`

// StatsOverlay renders performance stats
type StatsOverlay struct {
	program   uint32
	vao       uint32
	vbo       uint32
	
	width     float32
	height    float32
	
	// Stats data
	fps       float64
	zoom      float64
	distance  float32
	
	// Debug
	renderCount int
}

// NewStatsOverlay creates a stats overlay renderer
func NewStatsOverlay(width, height int) (*StatsOverlay, error) {
	so := &StatsOverlay{
		width:  float32(width),
		height: float32(height),
	}
	
	// Compile shaders
	vertShader, err := compileShader(statsVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("failed to compile stats vertex shader: %v", err)
	}
	defer gl.DeleteShader(vertShader)
	
	fragShader, err := compileShader(statsFragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return nil, fmt.Errorf("failed to compile stats fragment shader: %v", err)
	}
	defer gl.DeleteShader(fragShader)
	
	// Link program
	so.program = gl.CreateProgram()
	gl.AttachShader(so.program, vertShader)
	gl.AttachShader(so.program, fragShader)
	gl.LinkProgram(so.program)
	
	var status int32
	gl.GetProgramiv(so.program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(so.program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(so.program, logLength, nil, gl.Str(log))
		return nil, fmt.Errorf("stats shader link failed: %s", log)
	}
	
	// Create VAO and VBO
	gl.GenVertexArrays(1, &so.vao)
	gl.GenBuffers(1, &so.vbo)
	
	gl.BindVertexArray(so.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, so.vbo)
	
	// Each vertex has 6 floats: 2 for position, 4 for color
	stride := int32(6 * 4) // 6 floats * 4 bytes per float
	
	// Position attribute (2 floats)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, stride, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	
	// Color attribute (4 floats)
	gl.VertexAttribPointer(1, 4, gl.FLOAT, false, stride, gl.PtrOffset(2*4))
	gl.EnableVertexAttribArray(1)
	
	gl.BindVertexArray(0)
	
	return so, nil
}

// UpdateStats updates the stats to display
func (so *StatsOverlay) UpdateStats(fps float64, zoom float64, distance float32) {
	so.fps = fps
	so.zoom = zoom
	so.distance = distance
}

// Render draws the stats overlay
func (so *StatsOverlay) Render() {
	// Debug: print once to confirm render is being called
	if so.renderCount == 0 {
		fmt.Println("DEBUG: StatsOverlay.Render() is being called!")
		var viewport [4]int32
		gl.GetIntegerv(gl.VIEWPORT, &viewport[0])
		fmt.Printf("Viewport: %d,%d %dx%d\n", viewport[0], viewport[1], viewport[2], viewport[3])
		fmt.Printf("Overlay size: %.0fx%.0f\n", so.width, so.height)
	}
	so.renderCount++
	
	// Save OpenGL state
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	
	// Clear any errors
	for gl.GetError() != gl.NO_ERROR {
	}
	
	gl.UseProgram(so.program)
	
	// Check if program is valid
	if so.program == 0 {
		fmt.Println("ERROR: Overlay shader program is 0!")
		return
	}
	
	// Set orthographic projection
	projection := mgl32.Ortho2D(0, so.width, so.height, 0)
	projLoc := gl.GetUniformLocation(so.program, gl.Str("projection\x00"))
	if projLoc == -1 {
		fmt.Println("ERROR: Could not find 'projection' uniform!")
	}
	gl.UniformMatrix4fv(projLoc, 1, false, &projection[0])
	
	// Draw background box in top-left corner
	boxX := float32(10)
	boxY := float32(10) // Top left
	boxW := float32(300)
	boxH := float32(100)
	
	vertices := []float32{
		// Position     Color (RGBA)
		// Background box (bright red for debugging)
		boxX,         boxY,         1.0, 0.0, 0.0, 1.0,
		boxX + boxW,  boxY,         1.0, 0.0, 0.0, 1.0,
		boxX,         boxY + boxH,  1.0, 0.0, 0.0, 1.0,
		boxX + boxW,  boxY,         1.0, 0.0, 0.0, 1.0,
		boxX + boxW,  boxY + boxH,  1.0, 0.0, 0.0, 1.0,
		boxX,         boxY + boxH,  1.0, 0.0, 0.0, 1.0,
	}
	
	gl.BindVertexArray(so.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, so.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.DYNAMIC_DRAW)
	
	// Check for GL errors before draw
	if err := gl.GetError(); err != gl.NO_ERROR {
		fmt.Printf("GL Error before draw: 0x%x\n", err)
	}
	
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	
	// Check for GL errors after draw
	if err := gl.GetError(); err != gl.NO_ERROR {
		fmt.Printf("GL Error after draw: 0x%x\n", err)
	}
	
	// Draw text lines as colored bars (simple visualization)
	// In a real implementation, you'd render actual text
	textY := boxY + 10
	
	// Add a bright test rectangle first
	testVertices := []float32{
		// Big white rectangle for testing
		100, 100, 1.0, 1.0, 1.0, 1.0,
		200, 100, 1.0, 1.0, 1.0, 1.0,
		100, 150, 1.0, 1.0, 1.0, 1.0,
		200, 100, 1.0, 1.0, 1.0, 1.0,
		200, 150, 1.0, 1.0, 1.0, 1.0,
		100, 150, 1.0, 1.0, 1.0, 1.0,
	}
	gl.BufferData(gl.ARRAY_BUFFER, len(testVertices)*4, gl.Ptr(testVertices), gl.DYNAMIC_DRAW)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	
	// FPS bar (green)
	so.drawTextBar(boxX + 10, textY, fmt.Sprintf("FPS: %.1f", so.fps), mgl32.Vec4{0.0, 1.0, 0.0, 1.0})
	
	// Zoom bar (blue) 
	so.drawTextBar(boxX + 10, textY + 25, fmt.Sprintf("Zoom: %.3f", so.zoom), mgl32.Vec4{0.5, 0.5, 1.0, 1.0})
	
	// Distance bar (yellow)
	so.drawTextBar(boxX + 10, textY + 50, fmt.Sprintf("Dist: %.0f km", so.distance/1000.0), mgl32.Vec4{1.0, 1.0, 0.0, 1.0})
	
	// Restore OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.BLEND)
	gl.BindVertexArray(0)
}

// drawTextBar draws a simple colored bar to represent text
func (so *StatsOverlay) drawTextBar(x, y float32, text string, color mgl32.Vec4) {
	// For now, just draw a colored line to show where text would be
	// Width based on text length
	width := float32(len(text) * 8)
	height := float32(15)
	
	vertices := []float32{
		// Position     Color
		x,         y,          color[0], color[1], color[2], color[3],
		x + width, y,          color[0], color[1], color[2], color[3],
		x,         y + height, color[0], color[1], color[2], color[3],
		x + width, y,          color[0], color[1], color[2], color[3],
		x + width, y + height, color[0], color[1], color[2], color[3],
		x,         y + height, color[0], color[1], color[2], color[3],
	}
	
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.DYNAMIC_DRAW)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
}

// UpdateSize updates viewport size
func (so *StatsOverlay) UpdateSize(width, height int) {
	so.width = float32(width)
	so.height = float32(height)
}

// Release cleans up resources
func (so *StatsOverlay) Release() {
	if so.program != 0 {
		gl.DeleteProgram(so.program)
	}
	if so.vao != 0 {
		gl.DeleteVertexArrays(1, &so.vao)
	}
	if so.vbo != 0 {
		gl.DeleteBuffers(1, &so.vbo)
	}
}