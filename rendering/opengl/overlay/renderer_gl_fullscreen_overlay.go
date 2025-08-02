package overlay

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// FullscreenOverlayShader contains the shader constants and functions for fullscreen overlay
type FullscreenOverlayShader struct {
	Program uint32
	VAO     uint32
}

// Fullscreen quad overlay approach
const fullscreenOverlayVertexShader = `
#version 430 core

// Generate fullscreen triangle
vec2 positions[3] = vec2[](
    vec2(-1.0, -1.0),
    vec2( 3.0, -1.0),
    vec2(-1.0,  3.0)
);

out vec2 fragCoord;

void main() {
    vec2 pos = positions[gl_VertexID];
    fragCoord = pos * 0.5 + 0.5;
    gl_Position = vec4(pos, 0.0, 1.0);
}
`

const fullscreenOverlayFragmentShader = `
#version 430 core

in vec2 fragCoord;
out vec4 outColor;

uniform vec2 screenSize;
uniform float fps;
uniform float zoom;
uniform float distance;

void main() {
    vec2 pixelPos = fragCoord * screenSize;
    
    // Bottom-left overlay
    if (pixelPos.x < 260.0 && pixelPos.y > screenSize.y - 90.0) {
        float y = pixelPos.y - (screenSize.y - 90.0); // Y relative to bottom
        
        if (pixelPos.x > 10.0 && pixelPos.x < 260.0 && y > 0.0 && y < 80.0) {
            // Dark blue background
            outColor = vec4(0.1, 0.1, 0.3, 0.8);
            
            // FPS row (y: 10-20)
            if (y > 10.0 && y < 20.0) {
                // White text placeholder "FPS: XXX"
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Green bar
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(fps, 150.0)) {
                    outColor = vec4(0.0, 1.0, 0.0, 1.0);
                }
            }
            
            // Zoom row (y: 30-40)
            if (y > 30.0 && y < 40.0) {
                // White text placeholder "Z: X.XXX"
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Blue bar
                float zoomBar = zoom * 100.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(zoomBar, 200.0)) {
                    outColor = vec4(0.5, 0.5, 1.0, 1.0);
                }
            }
            
            // Distance row (y: 50-60)
            if (y > 50.0 && y < 60.0) {
                // White text placeholder "km: XXXXX"
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Yellow bar
                float distBar = distance / 100000.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(distBar, 200.0)) {
                    outColor = vec4(1.0, 1.0, 0.0, 1.0);
                }
            }
        } else {
            discard;
        }
    } else {
        discard;
    }
}
`

// NewFullscreenOverlayShader creates and initializes a fullscreen overlay shader
func NewFullscreenOverlayShader() (*FullscreenOverlayShader, error) {
	shader := &FullscreenOverlayShader{}
	
	var err error
	shader.Program, err = compileFullscreenOverlayShaders()
	if err != nil {
		return nil, err
	}

	// Create empty VAO for fullscreen triangle
	gl.GenVertexArrays(1, &shader.VAO)

	return shader, nil
}

// compileShader compiles a single shader
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(shader, logLength, nil, &log[0])
		return 0, fmt.Errorf("%s", log)
	}

	return shader, nil
}

// compileFullscreenOverlayShaders compiles the overlay shaders
func compileFullscreenOverlayShaders() (uint32, error) {
	vertShader, err := compileShader(fullscreenOverlayVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex shader: %v", err)
	}
	defer gl.DeleteShader(vertShader)

	fragShader, err := compileShader(fullscreenOverlayFragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("fragment shader: %v", err)
	}
	defer gl.DeleteShader(fragShader)

	program := gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(program, logLength, nil, &log[0])
		return 0, fmt.Errorf("link failed: %s", log)
	}

	fmt.Println("Fullscreen overlay shader linked successfully")
	return program, nil
}

// Global frame counter for occasional logging
var frameCounter int

func init() {
	frameCounter = 0
}

// IncrementFrameCounter increments the frame counter
func IncrementFrameCounter() {
	frameCounter++
}

// GetFrameCounter returns the current frame count
func GetFrameCounter() int {
	return frameCounter
}