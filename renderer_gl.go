package main

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"github.com/go-gl/gl/v4.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// VoxelRenderer handles native OpenGL rendering of voxel data
type VoxelRenderer struct {
	window *glfw.Window
	
	// Shader programs
	rayMarchProgram uint32
	ssboProgram     uint32  // SSBO-based ray marching
	volumeProgram   uint32  // True volume ray marching
	
	// Test mode (can be removed after debugging)
	currentTest int
	useSSBO     bool    // Use SSBO instead of textures
	useVolume   bool    // Use volume rendering instead of surface
	
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
	lonCountSSBO   uint32  // Longitude counts per latitude band
	
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
	renderMode       int32  // 0=material, 1=temperature, 2=velocity, 3=age, 4=plates
	crossSection     bool
	crossSectionAxis int32  // 0=X, 1=Y, 2=Z
	crossSectionPos  float32
	
	// Plate visualization
	showPlates       bool
	selectedPlateID  int
	highlightBoundaries bool
	
	// Volume rendering parameters
	opacityScale     float32
	stepSize         float32
	maxStepsVolume   int32
	densityThreshold float32
	
	// Mouse state for camera control
	mouseDown        bool
	lastMouseX       float64
	lastMouseY       float64
	cameraRotationX  float32
	cameraRotationY  float32
	
	// Planet reference for picking
	planetRef        *VoxelPlanet
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
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	
	// Create window
	window, err := glfw.CreateWindow(width, height, "Voxel Planet Evolution", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %v", err)
	}
	
	window.MakeContextCurrent()
	
	// Disable vsync for accurate performance measurement
	glfw.SwapInterval(0)
	
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
		// Volume rendering defaults
		opacityScale:     1.0,
		stepSize:         1.0,
		maxStepsVolume:   400,
		densityThreshold: 0.0,
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
	
	// Try to create SSBO-based shader (optional)
	ssboProgram, err := CreateSSBOProgram()
	if err != nil {
		fmt.Printf("Warning: Failed to create SSBO shader (OpenGL 4.3+ required): %v\n", err)
		r.useSSBO = false
	} else {
		r.ssboProgram = ssboProgram
		r.useSSBO = false // Disable SSBO until fixed
		fmt.Println("✅ SSBO shaders compiled successfully")
	}
	
	// Create volume ray marching shader
	volumeProgram, err := CompileVolumeRayMarchShaders()
	if err != nil {
		fmt.Printf("Warning: Failed to create volume shader: %v\n", err)
		r.useVolume = false
	} else {
		r.volumeProgram = volumeProgram
		r.useVolume = false // Default to surface rendering
		fmt.Println("✅ Volume ray marching shaders compiled successfully")
	}
	
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

// HasComputeShaderSupport checks if compute shaders are available
func (r *VoxelRenderer) HasComputeShaderSupport() bool {
	var major, minor int32
	gl.GetIntegerv(gl.MAJOR_VERSION, &major)
	gl.GetIntegerv(gl.MINOR_VERSION, &minor)
	return major > 4 || (major == 4 && minor >= 3)
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

// SetOptimizedBuffers uses optimized GPU buffer manager instead of copying data
func (r *VoxelRenderer) SetOptimizedBuffers(mgr *WindowsGPUBufferManager) {
	// Replace our SSBOs with the optimized ones
	if r.voxelSSBO != 0 {
		gl.DeleteBuffers(1, &r.voxelSSBO)
	}
	if r.shellSSBO != 0 {
		gl.DeleteBuffers(1, &r.shellSSBO)
	}
	if r.lonCountSSBO != 0 {
		gl.DeleteBuffers(1, &r.lonCountSSBO)
	}
	
	// Use the optimized buffer IDs
	r.voxelSSBO, r.shellSSBO, r.lonCountSSBO = mgr.GetBufferIDs()
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
	
	// Choose shader program based on mode
	var program uint32
	if r.useVolume && r.volumeProgram != 0 {
		program = r.volumeProgram
	} else if r.useSSBO && r.ssboProgram != 0 {
		program = r.ssboProgram
	} else {
		program = r.rayMarchProgram
	}
	
	// Use selected program
	gl.UseProgram(program)
	
	// Set uniforms
	invViewProj := r.projMatrix.Mul4(r.viewMatrix).Inv()
	gl.UniformMatrix4fv(gl.GetUniformLocation(program, gl.Str("invViewProj\x00")), 1, false, &invViewProj[0])
	gl.Uniform3fv(gl.GetUniformLocation(program, gl.Str("cameraPos\x00")), 1, &r.cameraPos[0])
	gl.Uniform1f(gl.GetUniformLocation(program, gl.Str("planetRadius\x00")), r.planetRadius)
	gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("renderMode\x00")), r.renderMode)
	
	// Cross-section uniforms
	crossSectionInt := int32(0)
	if r.crossSection {
		crossSectionInt = 1
	}
	gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("crossSection\x00")), crossSectionInt)
	gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("crossSectionAxis\x00")), r.crossSectionAxis)
	gl.Uniform1f(gl.GetUniformLocation(program, gl.Str("crossSectionPos\x00")), r.crossSectionPos)
	
	// Add shell count uniform - use the actual planet shell count
	shellCount := r.planetShellCount
	if shellCount == 0 {
		shellCount = 20 // Default fallback
	}
	gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("shellCount\x00")), shellCount)
	
	// Set volume rendering uniforms if using volume shader
	if r.useVolume && r.volumeProgram != 0 {
		gl.Uniform1f(gl.GetUniformLocation(program, gl.Str("opacityScale\x00")), r.opacityScale)
		gl.Uniform1f(gl.GetUniformLocation(program, gl.Str("stepSize\x00")), r.stepSize)
		gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("maxStepsVolume\x00")), r.maxStepsVolume)
		gl.Uniform1f(gl.GetUniformLocation(program, gl.Str("densityThreshold\x00")), r.densityThreshold)
	}
	
	// Bind data based on mode
	if r.useSSBO && r.ssboProgram != 0 {
		// SSBOs are already bound to binding points 0 and 1
		// No additional binding needed here
	} else {
		// Bind voxel textures for texture-based rendering
		if r.voxelTextures != nil {
			r.voxelTextures.Bind()
			gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("materialTexture\x00")), 0)
			gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("temperatureTexture\x00")), 1)
			gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("velocityTexture\x00")), 2)
			gl.Uniform1i(gl.GetUniformLocation(program, gl.Str("shellInfoTexture\x00")), 3)
		}
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
		fmt.Println("Switched to material view")
	case glfw.Key2:
		r.renderMode = 1 // Temperature
		fmt.Println("Switched to temperature view")
	case glfw.Key3:
		r.renderMode = 2 // Velocity
		fmt.Println("Switched to velocity view")
	case glfw.Key4:
		r.renderMode = 3 // Age
		fmt.Println("Switched to age view")
	case glfw.Key5:
		r.renderMode = 4 // Plates
		r.showPlates = true
		fmt.Println("Switched to plate tectonics view")
		fmt.Println("Click on plates to see their information")
	case glfw.KeyB:
		// Toggle boundary highlighting
		r.highlightBoundaries = !r.highlightBoundaries
		if r.highlightBoundaries {
			fmt.Println("Plate boundaries highlighted")
		} else {
			fmt.Println("Plate boundaries normal")
		}
	case glfw.KeyX:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 0
	case glfw.KeyY:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 1
	case glfw.KeyZ:
		r.crossSection = !r.crossSection
		r.crossSectionAxis = 2
	case glfw.KeyS:
		// Toggle SSBO mode
		if r.ssboProgram != 0 {
			r.useSSBO = !r.useSSBO
			if r.useSSBO {
				fmt.Println("Switched to SSBO-based rendering")
			} else {
				fmt.Println("Switched to texture-based rendering")
			}
		}
	case glfw.KeyV:
		// Toggle volume rendering
		if r.volumeProgram != 0 {
			r.useVolume = !r.useVolume
			if r.useVolume {
				fmt.Println("Switched to volume rendering - see inside the planet!")
				fmt.Println("Use +/- to adjust opacity, [/] to adjust step size")
			} else {
				fmt.Println("Switched to surface rendering")
			}
		}
	case glfw.KeyEqual, glfw.KeyKPAdd:
		// Increase opacity
		if r.useVolume {
			r.opacityScale = r.opacityScale * 1.2
			if r.opacityScale > 10.0 {
				r.opacityScale = 10.0
			}
			fmt.Printf("Volume opacity scale: %.2f\n", r.opacityScale)
		}
	case glfw.KeyMinus, glfw.KeyKPSubtract:
		// Decrease opacity
		if r.useVolume {
			r.opacityScale = r.opacityScale / 1.2
			if r.opacityScale < 0.1 {
				r.opacityScale = 0.1
			}
			fmt.Printf("Volume opacity scale: %.2f\n", r.opacityScale)
		}
	case glfw.KeyLeftBracket:
		// Decrease step size (better quality)
		if r.useVolume {
			r.stepSize = r.stepSize / 1.2
			if r.stepSize < 0.1 {
				r.stepSize = 0.1
			}
			fmt.Printf("Volume step size: %.2f (smaller = better quality)\n", r.stepSize)
		}
	case glfw.KeyRightBracket:
		// Increase step size (faster)
		if r.useVolume {
			r.stepSize = r.stepSize * 1.2
			if r.stepSize > 5.0 {
				r.stepSize = 5.0
			}
			fmt.Printf("Volume step size: %.2f (larger = faster)\n", r.stepSize)
		}
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
			
			// Check for plate selection in plate mode
			if r.renderMode == 4 && r.planetRef != nil {
				r.HandleMouseClick(r.lastMouseX, r.lastMouseY, r.planetRef)
			}
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
	if r.ssboProgram != 0 {
		gl.DeleteProgram(r.ssboProgram)
	}
	if r.volumeProgram != 0 {
		gl.DeleteProgram(r.volumeProgram)
	}
	gl.DeleteVertexArrays(1, &r.quadVAO)
	gl.DeleteBuffers(1, &r.voxelSSBO)
	gl.DeleteBuffers(1, &r.shellSSBO)
	if r.lonCountSSBO != 0 {
		gl.DeleteBuffers(1, &r.lonCountSSBO)
	}
	r.window.Destroy()
	glfw.Terminate()
}
