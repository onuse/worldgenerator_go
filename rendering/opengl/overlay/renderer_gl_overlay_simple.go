package overlay

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// SimpleOverlayShader contains the shader for simple colored rectangle overlays
type SimpleOverlayShader struct {
	Program uint32
	VAO     uint32
}

// Simple overlay shader for colored rectangles
const overlayVertexShader = `
#version 430 core

const vec2 positions[4] = vec2[](
    vec2(0.0, 0.0),
    vec2(1.0, 0.0),
    vec2(0.0, 1.0),
    vec2(1.0, 1.0)
);

uniform vec2 offset;
uniform vec2 size;
uniform vec2 screenSize;

void main() {
    vec2 pos = positions[gl_VertexID];
    vec2 pixelPos = offset + pos * size;
    vec2 ndcPos = (pixelPos / screenSize) * 2.0 - 1.0;
    ndcPos.y = -ndcPos.y; // Flip Y for top-left origin
    gl_Position = vec4(ndcPos, 0.0, 1.0);
}
`

const overlayFragmentShader = `
#version 430 core

uniform vec4 color;
out vec4 outColor;

void main() {
    outColor = color;
}
`

// NewSimpleOverlayShader creates and initializes a simple overlay shader
func NewSimpleOverlayShader() (*SimpleOverlayShader, error) {
	shader := &SimpleOverlayShader{}
	
	var err error
	shader.Program, err = compileOverlayShaders()
	if err != nil {
		return nil, err
	}

	// Create empty VAO for drawing
	gl.GenVertexArrays(1, &shader.VAO)

	return shader, nil
}

// compileOverlayShaders compiles the overlay shaders
func compileOverlayShaders() (uint32, error) {
	vertShader, err := compileShader(overlayVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex shader: %v", err)
	}
	defer gl.DeleteShader(vertShader)

	fragShader, err := compileShader(overlayFragmentShader, gl.FRAGMENT_SHADER)
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

	fmt.Println("Overlay shader program linked successfully")
	return program, nil
}