package main

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// VoxelRenderer handles native OpenGL rendering of voxel data
type VoxelRenderer struct {
	window *glfw.Window
	
	// Shader programs
	rayMarchProgram uint32
	
	// Test mode (can be removed after debugging)
	currentTest int
	
	// Vertex array for fullscreen quad
	quadVAO uint32
	
	// Sphere geometry for test rendering
	sphereVAO        uint32
	sphereVBO        uint32
	sphereEBO        uint32
	sphereIndexCount int32
	
	// Shared GPU buffers
	voxelSSBO      uint32  // Shared with Metal compute
	shellSSBO      uint32  // Shell metadata
	
	// Voxel texture data
	voxelTextures  *VoxelTextureData
	
	// Planet reference for shell count
	planetShellCount int32
	
	// Uniforms
	viewMatrix       mgl32.Mat4
	projMatrix       mgl32.Mat4
	cameraPos        mgl32.Vec3
	planetRadius     float32
	
	// Render settings
	width, height    int
	renderMode       int32  // 0=material, 1=temperature, 2=velocity, 3=age
	crossSection     bool
	crossSectionAxis int32  // 0=X, 1=Y, 2=Z
	crossSectionPos  float32
	
	// Mouse state for camera control
	mouseDown        bool
	lastMouseX       float64
	lastMouseY       float64
	cameraRotationX  float32
	cameraRotationY  float32
}

// NewVoxelRenderer creates a native OpenGL voxel renderer
func NewVoxelRenderer(width, height int) (*VoxelRenderer, error) {
	runtime.LockOSThread()
	
	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize GLFW: %v", err)
	}
	
	// Configure OpenGL context
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	
	// Create window
	window, err := glfw.CreateWindow(width, height, "Voxel Planet Evolution", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %v", err)
	}
	
	window.MakeContextCurrent()
	
	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL: %v", err)
	}
	
	// Print OpenGL info
	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version:", version)
	
	r := &VoxelRenderer{
		window:       window,
		width:        width,
		height:       height,
		planetRadius: 6371000, // Earth radius in meters
		renderMode:   0,
		cameraPos:    mgl32.Vec3{0, 0, float32(6371000 * 3)}, // 3x planet radius
		cameraRotationX: 0,
		cameraRotationY: 0,
	}
	
	// Setup OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.CULL_FACE)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0)
	
	// Create shader program - ray marching works perfectly now!
	program, err := CompileVoxelRayMarchShaders()
	if err != nil {
		return nil, fmt.Errorf("failed to compile ray march shaders: %v", err)
	}
	r.rayMarchProgram = program
	
	// Create fullscreen quad for ray marching
	r.createQuad()
	
	// Create sphere for test rendering
	r.createSphere()
	
	// Create voxel texture storage
	r.voxelTextures = NewVoxelTextureData(30) // Support up to 30 shells
	
	// Test system can be enabled if needed for debugging
	// r.CreateTestRenderers()
	
	// Setup matrices
	r.updateMatrices()
	
	// Setup callbacks
	window.SetSizeCallback(func(w *glfw.Window, width, height int) {
		r.onResize(width, height)
	})
	
	window.SetKeyCallback(func(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
		r.onKey(key, scancode, action, mods)
	})
	
	window.SetScrollCallback(func(w *glfw.Window, xoff, yoff float64) {
		r.onScroll(xoff, yoff)
	})
	
	window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		r.onMouseButton(button, action, mods)
	})
	
	window.SetCursorPosCallback(func(w *glfw.Window, xpos, ypos float64) {
		r.onMouseMove(xpos, ypos)
	})
	
	return r, nil
}

// createShadersOld was the old shader creation method - replaced by CompileVoxelShaders
func (r *VoxelRenderer) createShadersOld() error {
	vertexShader := `
#version 410 core

// Fullscreen quad vertices
const vec2 positions[4] = vec2[](
    vec2(-1.0, -1.0),
    vec2( 1.0, -1.0),
    vec2(-1.0,  1.0),
    vec2( 1.0,  1.0)
);

out vec2 fragCoord;

void main() {
    vec2 pos = positions[gl_VertexID];
    fragCoord = pos * 0.5 + 0.5;
    gl_Position = vec4(pos, 0.0, 1.0);
}
`

	fragmentShader := `
#version 410 core

in vec2 fragCoord;
out vec4 outColor;

// Uniforms
uniform mat4 invViewProj;
uniform vec3 cameraPos;
uniform float planetRadius;
uniform int renderMode;
uniform int crossSection;
uniform int crossSectionAxis;
uniform float crossSectionPos;

// Simplified voxel data for initial testing
uniform sampler2D voxelTexture;

// Simple planet rendering for initial test
vec3 renderPlanet(vec3 ro, vec3 rd) {
    // Simple sphere intersection
    vec3 oc = ro;
    float b = dot(oc, rd);
    float c = dot(oc, oc) - planetRadius * planetRadius;
    float discriminant = b * b - c;
    
    if (discriminant < 0.0) {
        return vec3(0.05, 0.05, 0.1); // Space background
    }
    
    float t = -b - sqrt(discriminant);
    if (t < 0.0) {
        return vec3(0.05, 0.05, 0.1);
    }
    
    vec3 pos = ro + rd * t;
    vec3 normal = normalize(pos);
    
    // Simple shading
    vec3 lightDir = normalize(vec3(0.5, 1.0, 0.3));
    float NdotL = max(dot(normal, lightDir), 0.0);
    
    // Planet color
    vec3 baseColor = vec3(0.2, 0.5, 0.8);
    return baseColor * (0.3 + 0.7 * NdotL);
}

void main() {
    // Generate ray from screen coordinates
    vec4 nearPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, -1.0, 1.0);
    vec4 farPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, 1.0, 1.0);
    
    vec3 ro = cameraPos;
    vec3 rd = normalize(farPoint.xyz / farPoint.w - nearPoint.xyz / nearPoint.w);
    
    // Simple planet rendering
    vec3 color = renderPlanet(ro, rd);
    outColor = vec4(color, 1.0);
}
`

	// Compile shaders
	vertShader, err := compileShader(vertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("vertex shader error: %v", err)
	}
	defer gl.DeleteShader(vertShader)
	
	fragShader, err := compileShader(fragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return fmt.Errorf("fragment shader error: %v", err)
	}
	defer gl.DeleteShader(fragShader)
	
	// Link program
	r.rayMarchProgram = gl.CreateProgram()
	gl.AttachShader(r.rayMarchProgram, vertShader)
	gl.AttachShader(r.rayMarchProgram, fragShader)
	gl.LinkProgram(r.rayMarchProgram)
	
	var status int32
	gl.GetProgramiv(r.rayMarchProgram, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(r.rayMarchProgram, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(r.rayMarchProgram, logLength, nil, &log[0])
		return fmt.Errorf("program link error: %s", log)
	}
	
	return nil
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

// createQuad creates a VAO for fullscreen quad
func (r *VoxelRenderer) createQuad() {
	gl.GenVertexArrays(1, &r.quadVAO)
	// No VBO needed - we generate vertices in shader
}

// CreateBuffers creates OpenGL SSBOs for voxel data
func (r *VoxelRenderer) CreateBuffers(buffers *SharedGPUBuffers) {
	// Create voxel SSBO
	gl.GenBuffers(1, &r.voxelSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
	voxelSize := len(buffers.voxelData) * int(unsafe.Sizeof(GPUVoxelMaterial{}))
	if len(buffers.voxelData) > 0 {
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, unsafe.Pointer(&buffers.voxelData[0]), gl.DYNAMIC_DRAW)
	} else {
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, gl.DYNAMIC_DRAW)
	}
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, r.voxelSSBO)
	
	// Create shell metadata SSBO with header
	type ShellHeader struct {
		ShellCount int32
		_padding   [3]int32
	}
	
	header := ShellHeader{ShellCount: int32(len(buffers.shellData))}
	shellSize := int(unsafe.Sizeof(header)) + len(buffers.shellData)*int(unsafe.Sizeof(SphericalShellMetadata{}))
	
	gl.GenBuffers(1, &r.shellSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.shellSSBO)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, shellSize, nil, gl.DYNAMIC_DRAW)
	
	// Upload header
	headerBytes := (*[16]byte)(unsafe.Pointer(&header))
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, int(unsafe.Sizeof(header)), gl.Ptr(&headerBytes[0]))
	
	// Upload shell data
	if len(buffers.shellData) > 0 {
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, int(unsafe.Sizeof(header)), 
			len(buffers.shellData)*int(unsafe.Sizeof(SphericalShellMetadata{})), 
			unsafe.Pointer(&buffers.shellData[0]))
	}
	
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, r.shellSSBO)
}

// UpdateBuffers updates the GPU buffers with new voxel data
func (r *VoxelRenderer) UpdateBuffers(buffers *SharedGPUBuffers) {
	// Update voxel data
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
	if len(buffers.voxelData) > 0 {
		voxelSize := len(buffers.voxelData) * int(unsafe.Sizeof(GPUVoxelMaterial{}))
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, voxelSize, unsafe.Pointer(&buffers.voxelData[0]))
	}
}

// UpdateVoxelTextures updates the voxel textures from planet data
func (r *VoxelRenderer) UpdateVoxelTextures(planet *VoxelPlanet) {
	if r.voxelTextures != nil {
		r.voxelTextures.UpdateFromPlanet(planet)
		r.planetShellCount = int32(len(planet.Shells))
	}
}

// Render performs one frame of voxel rendering
func (r *VoxelRenderer) Render() {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	
	// Use ray marching program
	gl.UseProgram(r.rayMarchProgram)
	
	// Set uniforms
	invViewProj := r.projMatrix.Mul4(r.viewMatrix).Inv()
	gl.UniformMatrix4fv(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("invViewProj\x00")), 1, false, &invViewProj[0])
	gl.Uniform3fv(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("cameraPos\x00")), 1, &r.cameraPos[0])
	gl.Uniform1f(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("planetRadius\x00")), r.planetRadius)
	gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("renderMode\x00")), r.renderMode)
	
	// Cross-section uniforms
	crossSectionInt := int32(0)
	if r.crossSection {
		crossSectionInt = 1
	}
	gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("crossSection\x00")), crossSectionInt)
	gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("crossSectionAxis\x00")), r.crossSectionAxis)
	gl.Uniform1f(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("crossSectionPos\x00")), r.crossSectionPos)
	
	// Add shell count uniform - use the actual planet shell count
	shellCount := r.planetShellCount
	if shellCount == 0 {
		shellCount = 20 // Default fallback
	}
	gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("shellCount\x00")), shellCount)
	
	// Bind voxel textures
	if r.voxelTextures != nil {
		r.voxelTextures.Bind()
		gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("materialTexture\x00")), 0)
		gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("temperatureTexture\x00")), 1)
		gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("velocityTexture\x00")), 2)
		gl.Uniform1i(gl.GetUniformLocation(r.rayMarchProgram, gl.Str("shellInfoTexture\x00")), 3)
	}
	
	// Draw fullscreen quad
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	
	r.window.SwapBuffers()
}

// updateMatrices updates view and projection matrices
func (r *VoxelRenderer) updateMatrices() {
	// Calculate camera position from rotation angles
	dist := r.cameraPos.Len()
	
	// Convert spherical coordinates to cartesian
	x := dist * float32(math.Cos(float64(r.cameraRotationY))) * float32(math.Cos(float64(r.cameraRotationX)))
	y := dist * float32(math.Sin(float64(r.cameraRotationY)))
	z := dist * float32(math.Cos(float64(r.cameraRotationY))) * float32(math.Sin(float64(r.cameraRotationX)))
	
	r.cameraPos = mgl32.Vec3{x, y, z}
	
	// View matrix - looking at origin
	r.viewMatrix = mgl32.LookAtV(
		r.cameraPos,
		mgl32.Vec3{0, 0, 0},
		mgl32.Vec3{0, 1, 0},
	)
	
	// Projection matrix - adjust near/far planes for planet scale
	aspect := float32(r.width) / float32(r.height)
	r.projMatrix = mgl32.Perspective(mgl32.DegToRad(45.0), aspect, 1000.0, 100000000.0)
}

// Event handlers
func (r *VoxelRenderer) onResize(width, height int) {
	r.width = width
	r.height = height
	gl.Viewport(0, 0, int32(width), int32(height))
	r.updateMatrices()
}

func (r *VoxelRenderer) onKey(key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if action != glfw.Press {
		return
	}
	
	switch key {
	case glfw.KeyEscape:
		r.window.SetShouldClose(true)
	case glfw.Key1:
		r.renderMode = 0 // Material
	case glfw.Key2:
		r.renderMode = 1 // Temperature
	case glfw.Key3:
		r.renderMode = 2 // Velocity
	case glfw.Key4:
		r.renderMode = 3 // Age
	case glfw.KeyX:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 0
	case glfw.KeyY:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 1
	case glfw.KeyZ:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 2
	}
}

func (r *VoxelRenderer) onScroll(xoff, yoff float64) {
	// Zoom camera
	zoom := float32(1.0 - yoff*0.1) // Inverted for natural scrolling
	dist := r.cameraPos.Len() * zoom
	
	// Update camera position while maintaining rotation
	x := dist * float32(math.Cos(float64(r.cameraRotationY))) * float32(math.Cos(float64(r.cameraRotationX)))
	y := dist * float32(math.Sin(float64(r.cameraRotationY)))
	z := dist * float32(math.Cos(float64(r.cameraRotationY))) * float32(math.Sin(float64(r.cameraRotationX)))
	
	r.cameraPos = mgl32.Vec3{x, y, z}
	r.updateMatrices()
}

// onMouseButton handles mouse button events
func (r *VoxelRenderer) onMouseButton(button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	if button == glfw.MouseButtonLeft {
		if action == glfw.Press {
			r.mouseDown = true
			r.lastMouseX, r.lastMouseY = r.window.GetCursorPos()
		} else if action == glfw.Release {
			r.mouseDown = false
		}
	}
}

// onMouseMove handles mouse movement
func (r *VoxelRenderer) onMouseMove(xpos, ypos float64) {
	if r.mouseDown {
		dx := float32(xpos - r.lastMouseX)
		dy := float32(ypos - r.lastMouseY)
		
		// Update camera rotation (inverted for natural feel)
		r.cameraRotationX += dx * 0.01
		r.cameraRotationY += dy * 0.01
		
		// Clamp vertical rotation
		if r.cameraRotationY > 1.5 {
			r.cameraRotationY = 1.5
		}
		if r.cameraRotationY < -1.5 {
			r.cameraRotationY = -1.5
		}
		
		r.lastMouseX = xpos
		r.lastMouseY = ypos
		
		r.updateMatrices()
	}
}

// ShouldClose returns true if the window should close
func (r *VoxelRenderer) ShouldClose() bool {
	return r.window.ShouldClose()
}

// PollEvents processes window events
func (r *VoxelRenderer) PollEvents() {
	glfw.PollEvents()
}

// Terminate cleans up OpenGL resources
func (r *VoxelRenderer) Terminate() {
	if r.voxelTextures != nil {
		r.voxelTextures.Cleanup()
	}
	gl.DeleteProgram(r.rayMarchProgram)
	gl.DeleteVertexArrays(1, &r.quadVAO)
	gl.DeleteBuffers(1, &r.voxelSSBO)
	gl.DeleteBuffers(1, &r.shellSSBO)
	r.window.Destroy()
	glfw.Terminate()
}