package opengl

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"github.com/go-gl/gl/v4.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"worldgenerator/core"
	"worldgenerator/gpu"
	"worldgenerator/rendering/opengl/overlay"
	"worldgenerator/rendering/opengl/shaders"
	"worldgenerator/rendering/textures"
)

// VoxelRenderer handles native OpenGL rendering of voxel data
type VoxelRenderer struct {
	window *glfw.Window

	// Shader program - ONLY ONE RENDERING PATH
	shaderProgram uint32

	// Vertex array for fullscreen quad
	quadVAO uint32

	// Sphere geometry for test rendering
	sphereVAO        uint32
	sphereVBO        uint32
	sphereEBO        uint32
	sphereIndexCount int32

	// Shared GPU buffers
	voxelSSBO    uint32 // Shared with Metal compute
	shellSSBO    uint32 // Shell metadata
	lonCountSSBO uint32 // Longitude counts per latitude band

	// Voxel texture data
	voxelTextures *textures.VoxelTextureData

	// Planet reference for shell count
	planetShellCount int32

	// Uniforms
	viewMatrix   mgl32.Mat4
	projMatrix   mgl32.Mat4
	cameraPos    mgl32.Vec3
	planetRadius float32

	// Render settings
	width, height    int
	RenderMode       int32 // 0=material, 1=temperature, 2=velocity, 3=age, 4=plates
	crossSection     bool
	crossSectionAxis int32 // 0=X, 1=Y, 2=Z
	elevationScale   float32 // Exaggeration factor for elevation (0=flat, 100=visible)
	crossSectionPos  float32

	// Plate visualization
	ShowPlates          bool
	selectedPlateID     int
	highlightBoundaries bool


	// Mouse state for camera control
	MouseDown       bool
	lastMouseX      float64
	lastMouseY      float64
	cameraRotationX float32
	cameraRotationY float32

	// Planet reference for picking
	PlanetRef interface{} // *core.VoxelPlanet but avoid import cycle

	// Stats overlay
	statsOverlay *overlay.StatsOverlay
	showStats    bool
	
	// Simulation control (public for main.go access)
	SpeedMultiplier float32
	Paused          bool
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
		window:          window,
		width:           width,
		height:          height,
		planetRadius:    6371000, // Earth radius in meters
		RenderMode:      0,
		cameraPos:       mgl32.Vec3{0, 0, float32(6371000 * 3)}, // 3x planet radius
		cameraRotationX: 0,
		cameraRotationY: 0,
		showStats:        true, // Show stats overlay by default
		SpeedMultiplier:  1.0,
		Paused:           false,
	}

	// Setup OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.CULL_FACE)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0)

	// Create THE ONLY shader program
	program, err := shaders.CompileVoxelRayMarchShaders()
	if err != nil {
		return nil, fmt.Errorf("failed to compile shaders: %v", err)
	}
	r.shaderProgram = program
	fmt.Println("✅ Shader compiled successfully")

	// Create fullscreen quad for ray marching
	r.createQuad()

	// Create sphere for test rendering
	// r.createSphere() // TODO: Implement if needed

	// Create voxel texture storage
	r.voxelTextures = textures.NewVoxelTextureData(30) // Support up to 30 shells

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

	// Create stats overlay
	fmt.Println("Creating stats overlay...")
	statsOverlay, err := overlay.NewStatsOverlay(width, height)
	if err != nil {
		fmt.Printf("ERROR: Failed to create stats overlay: %v\n", err)
		// Continue without stats
	} else {
		r.statsOverlay = statsOverlay
		fmt.Println("✅ Stats overlay created successfully")
		fmt.Printf("   Stats overlay object: %v\n", r.statsOverlay != nil)
	}

	return r, nil
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
func (r *VoxelRenderer) CreateBuffers(buffers *gpu.SharedGPUBuffers) {
	// Create voxel SSBO
	gl.GenBuffers(1, &r.voxelSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
	voxelSize := len(buffers.VoxelData) * int(unsafe.Sizeof(gpu.GPUVoxelMaterial{}))
	if len(buffers.VoxelData) > 0 {
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, unsafe.Pointer(&buffers.VoxelData[0]), gl.DYNAMIC_DRAW)
	} else {
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, voxelSize, nil, gl.DYNAMIC_DRAW)
	}
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, r.voxelSSBO)

	// Create shell metadata SSBO with header
	type ShellHeader struct {
		ShellCount int32
		_padding   [3]int32
	}

	header := ShellHeader{ShellCount: int32(len(buffers.ShellData))}
	shellSize := int(unsafe.Sizeof(header)) + len(buffers.ShellData)*int(unsafe.Sizeof(gpu.SphericalShellMetadata{}))

	gl.GenBuffers(1, &r.shellSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.shellSSBO)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, shellSize, nil, gl.DYNAMIC_DRAW)

	// Upload header
	headerBytes := (*[16]byte)(unsafe.Pointer(&header))
	gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, int(unsafe.Sizeof(header)), gl.Ptr(&headerBytes[0]))

	// Upload shell data
	if len(buffers.ShellData) > 0 {
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, int(unsafe.Sizeof(header)),
			len(buffers.ShellData)*int(unsafe.Sizeof(gpu.SphericalShellMetadata{})),
			unsafe.Pointer(&buffers.ShellData[0]))
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

// GetCameraDistance returns the current camera distance from the planet center
func (r *VoxelRenderer) GetCameraDistance() float32 {
	return r.cameraPos.Len()
}

// UpdateStats updates the stats overlay with current values
func (r *VoxelRenderer) UpdateStats(fps float64) {
	if r.statsOverlay != nil {
		distance := r.GetCameraDistance()
		zoom := (r.planetRadius * 3.0) / distance
		r.statsOverlay.UpdateStats(fps, float64(zoom), distance)
	}
}

// UpdateBuffers updates the GPU buffers with new voxel data
func (r *VoxelRenderer) UpdateBuffers(buffers *gpu.SharedGPUBuffers) {
	// Update voxel data
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, r.voxelSSBO)
	if len(buffers.VoxelData) > 0 {
		voxelSize := len(buffers.VoxelData) * int(unsafe.Sizeof(gpu.GPUVoxelMaterial{}))
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, voxelSize, unsafe.Pointer(&buffers.VoxelData[0]))
	}
}

// SetOptimizedBuffers uses optimized GPU buffer manager instead of copying data
func (r *VoxelRenderer) SetOptimizedBuffers(mgr *gpu.WindowsGPUBufferManager) {
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
func (r *VoxelRenderer) UpdateVoxelTextures(planet *core.VoxelPlanet) {
	if r.voxelTextures != nil {
		r.voxelTextures.UpdateFromPlanet(planet)
		r.planetShellCount = int32(len(planet.Shells))
	}
}

// Render performs one frame of voxel rendering
func (r *VoxelRenderer) Render() {
	// Check for OpenGL errors
	if err := gl.GetError(); err != gl.NO_ERROR {
		fmt.Printf("OpenGL error before render: 0x%x\n", err)
	}
	
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)


	// Use THE ONLY shader program
	gl.UseProgram(r.shaderProgram)

	// Set uniforms
	invViewProj := r.projMatrix.Mul4(r.viewMatrix).Inv()
	gl.UniformMatrix4fv(gl.GetUniformLocation(r.shaderProgram, gl.Str("invViewProj\x00")), 1, false, &invViewProj[0])
	gl.Uniform3fv(gl.GetUniformLocation(r.shaderProgram, gl.Str("cameraPos\x00")), 1, &r.cameraPos[0])
	gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("planetRadius\x00")), r.planetRadius)
	
	// Debug render mode
	renderModeLoc := gl.GetUniformLocation(r.shaderProgram, gl.Str("renderMode\x00"))
	if renderModeLoc < 0 {
		fmt.Printf("WARNING: renderMode uniform not found in shader!\n")
	}
	gl.Uniform1i(renderModeLoc, int32(r.RenderMode))

	// Cross-section uniforms
	crossSectionInt := int32(0)
	if r.crossSection {
		crossSectionInt = 1
	}
	gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("crossSection\x00")), crossSectionInt)
	gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("crossSectionAxis\x00")), r.crossSectionAxis)
	gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("crossSectionPos\x00")), r.crossSectionPos)

	// Add shell count uniform - use the actual planet shell count
	shellCount := r.planetShellCount
	if shellCount == 0 {
		shellCount = 20 // Default fallback
	}
	gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("shellCount\x00")), shellCount)
	
	// Add time uniform
	gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("time\x00")), float32(glfw.GetTime()))


	// Bind voxel textures for texture-based rendering
	if r.voxelTextures != nil {
		r.voxelTextures.Bind()
		gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("materialTexture\x00")), 0)
		gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("temperatureTexture\x00")), 1)
		gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("velocityTexture\x00")), 2)
		gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("shellInfoTexture\x00")), 3)
		
		// Debug: Add a debug value uniform
		gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("debugValue\x00")), float32(glfw.GetTime()))
	} else {
		fmt.Printf("WARNING: voxelTextures is nil!\n")
	}

	// Draw fullscreen quad
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	
	// Check for errors after draw
	if err := gl.GetError(); err != gl.NO_ERROR {
		fmt.Printf("OpenGL error after draw: 0x%x\n", err)
	}
	

	// Render stats overlay if enabled
	if r.showStats {
		r.RenderFullscreenStats()
	}

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
	case glfw.KeyF1:
		// Toggle stats overlay
		r.showStats = !r.showStats
		if r.showStats {
			fmt.Println("Stats overlay: ON")
		} else {
			fmt.Println("Stats overlay: OFF")
		}
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
	case glfw.Key0:
		r.SpeedMultiplier = 1.0
		fmt.Println("Time speed reset to 1x")
	case glfw.Key1:
		if mods&glfw.ModShift != 0 {
			// Shift+1 = 10x speed
			r.SpeedMultiplier = 10.0
			fmt.Printf("Time speed: %.0fx\n", r.SpeedMultiplier)
		} else {
			// Normal 1 = material view
			r.RenderMode = 0
			fmt.Println("Switched to material view")
		}
	case glfw.Key2:
		if mods&glfw.ModShift != 0 {
			// Shift+2 = 100x speed
			r.SpeedMultiplier = 100.0
			fmt.Printf("Time speed: %.0fx\n", r.SpeedMultiplier)
		} else {
			// Normal 2 = temperature view
			r.RenderMode = 1
			fmt.Println("Switched to temperature view")
		}
	case glfw.Key3:
		if mods&glfw.ModShift != 0 {
			// Shift+3 = 1000x speed
			r.SpeedMultiplier = 1000.0
			fmt.Printf("Time speed: %.0fx\n", r.SpeedMultiplier)
		} else {
			// Normal 3 = velocity view
			r.RenderMode = 2
			fmt.Println("Switched to velocity view")
		}
	case glfw.Key4:
		if mods&glfw.ModShift != 0 {
			// Shift+4 = 10000x speed
			r.SpeedMultiplier = 10000.0
			fmt.Printf("Time speed: %.0fx\n", r.SpeedMultiplier)
		} else {
			// Normal 4 = age view
			r.RenderMode = 3
			fmt.Println("Switched to age view")
		}
	case glfw.Key5:
		if mods&glfw.ModShift != 0 {
			// Shift+5 = 100000x speed
			r.SpeedMultiplier = 100000.0
			fmt.Printf("Time speed: %.0fx (continents should move visibly!)\n", r.SpeedMultiplier)
		} else {
			// Normal 5 = plate view
			r.RenderMode = 4
			r.ShowPlates = true
			fmt.Println("Switched to plate tectonics view")
			fmt.Println("Click on plates to see their information")
		}
	case glfw.Key6:
		// 6 = stress view
		r.RenderMode = 5
		fmt.Println("Switched to stress visualization")
		fmt.Println("Red = high stress/velocity, Blue = low stress")
	case glfw.Key7:
		// 7 = sub-position view
		r.RenderMode = 6
		fmt.Println("Switched to sub-position visualization")
		fmt.Println("Shows sub-cell positions: Red=lon, Green=lat, Blue=magnitude")
	case glfw.Key8:
		// 8 = elevation/altitude view
		r.RenderMode = 7
		fmt.Println("Switched to elevation visualization")
		fmt.Println("Blue=ocean trenches, Green=lowlands, Yellow=highlands, Red=mountains, White=peaks")
	case glfw.KeyP:
		r.Paused = !r.Paused
		if r.Paused {
			fmt.Println("Simulation PAUSED")
		} else {
			fmt.Println("Simulation RESUMED")
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
			r.MouseDown = true
			r.lastMouseX, r.lastMouseY = r.window.GetCursorPos()

			// Check for plate selection in plate mode
			if r.RenderMode == 4 && r.PlanetRef != nil {
				r.HandleMouseClick(r.lastMouseX, r.lastMouseY, r.PlanetRef.(*core.VoxelPlanet))
			}
		} else if action == glfw.Release {
			r.MouseDown = false
		}
	}
}

// onMouseMove handles mouse movement
func (r *VoxelRenderer) onMouseMove(xpos, ypos float64) {
	if r.MouseDown {
		dx := float32(xpos - r.lastMouseX)
		dy := float32(ypos - r.lastMouseY)

		// Base sensitivity calibrated so zoom 0.590 feels perfect
		// At zoom 0.590, the camera distance is planetRadius * 3.0 / 0.590
		perfectZoom := float32(0.590)
		defaultDistance := r.planetRadius * 3.0
		currentZoom := defaultDistance / r.cameraPos.Len()

		// Scale sensitivity based on zoom difference from perfect zoom
		// When at perfectZoom (0.590), scale = 1.0
		// Use an extremely gentle logarithmic scaling
		zoomRatio := currentZoom / perfectZoom
		// This formula gives very minimal scaling:
		// At 2x zoom: scale = ~1.05 (only 5% faster)
		// At 0.5x zoom: scale = ~0.95 (only 5% slower)
		zoomScale := float32(1.0 + 0.07*math.Log(float64(zoomRatio)))

		// Very tight clamp range for extremely subtle effect
		if zoomScale > 1.1 {
			zoomScale = 1.1
		} else if zoomScale < 0.9 {
			zoomScale = 0.9
		}

		// Base sensitivity that feels perfect at zoom 0.590
		baseSensitivity := float32(0.008)
		sensitivity := baseSensitivity * zoomScale

		// Update camera rotation (inverted for natural feel)
		r.cameraRotationX += dx * sensitivity
		r.cameraRotationY += dy * sensitivity

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
	gl.DeleteProgram(r.shaderProgram)
	gl.DeleteVertexArrays(1, &r.quadVAO)
	gl.DeleteBuffers(1, &r.voxelSSBO)
	gl.DeleteBuffers(1, &r.shellSSBO)
	if r.lonCountSSBO != 0 {
		gl.DeleteBuffers(1, &r.lonCountSSBO)
	}
	r.window.Destroy()
	glfw.Terminate()
}
