package physics

import (
	"sync"
	"sync/atomic"
	"time"
	"worldgenerator/core"
	"worldgenerator/gpu"
)

// ThreadedPhysicsEngine runs physics calculations in a background thread
type ThreadedPhysicsEngine struct {
	// Thread control
	running    atomic.Bool
	wg         sync.WaitGroup
	updateChan chan physicsUpdate

	// Double buffering for thread-safe data exchange
	planetA      *core.VoxelPlanet
	planetB      *core.VoxelPlanet
	currentRead  atomic.Pointer[core.VoxelPlanet]
	currentWrite atomic.Pointer[core.VoxelPlanet]
	swapMutex    sync.Mutex

	// Physics state
	physics    *VoxelPhysics
	gpuCompute gpu.GPUCompute
	simSpeed   float64

	// Performance tracking
	lastPhysicsTime   time.Time
	physicsFrameTime  float64
	physicsUpdateRate float64 // Updates per second
}

type physicsUpdate struct {
	deltaTime float64
	simSpeed  float64
}

// NewThreadedPhysicsEngine creates a new background physics engine
func NewThreadedPhysicsEngine(planet *core.VoxelPlanet, gpuCompute gpu.GPUCompute, simSpeed float64) *ThreadedPhysicsEngine {
	// Create a deep copy of the planet for double buffering
	planetCopy := deepCopyPlanet(planet)

	engine := &ThreadedPhysicsEngine{
		updateChan:        make(chan physicsUpdate, 10),
		planetA:           planet,
		planetB:           planetCopy,
		gpuCompute:        gpuCompute,
		simSpeed:          simSpeed,
		lastPhysicsTime:   time.Now(),
		physicsUpdateRate: 10.0, // 10 physics updates per second
	}

	// Set initial read/write pointers
	engine.currentRead.Store(planet)
	engine.currentWrite.Store(planetCopy)

	// Create physics system
	engine.physics = NewVoxelPhysics(planetCopy)

	return engine
}

// Start begins the physics thread
func (e *ThreadedPhysicsEngine) Start() {
	e.running.Store(true)
	e.wg.Add(1)
	go e.physicsThread()
}

// Stop halts the physics thread
func (e *ThreadedPhysicsEngine) Stop() {
	e.running.Store(false)
	close(e.updateChan)
	e.wg.Wait()
}

// GetCurrentPlanet returns the current read-safe planet data
func (e *ThreadedPhysicsEngine) GetCurrentPlanet() *core.VoxelPlanet {
	return e.currentRead.Load()
}

// SwapBuffers exchanges the read and write buffers
func (e *ThreadedPhysicsEngine) SwapBuffers() {
	e.swapMutex.Lock()
	defer e.swapMutex.Unlock()

	// Swap the pointers
	readPlanet := e.currentRead.Load()
	writePlanet := e.currentWrite.Load()
	e.currentRead.Store(writePlanet)
	e.currentWrite.Store(readPlanet)
}

// UpdateSimSpeed changes the simulation speed
func (e *ThreadedPhysicsEngine) UpdateSimSpeed(speed float64) {
	e.simSpeed = speed
}

// physicsThread runs in the background
func (e *ThreadedPhysicsEngine) physicsThread() {
	defer e.wg.Done()

	ticker := time.NewTicker(time.Duration(1000.0/e.physicsUpdateRate) * time.Millisecond)
	defer ticker.Stop()

	for e.running.Load() {
		select {
		case <-ticker.C:
			// Calculate time since last physics update
			now := time.Now()
			dt := now.Sub(e.lastPhysicsTime).Seconds()
			e.lastPhysicsTime = now

			// Get the write buffer
			writePlanet := e.currentWrite.Load()

			// Run physics simulation
			startTime := time.Now()
			UpdateVoxelPhysicsWrapper(writePlanet, dt*e.simSpeed, e.gpuCompute)
			e.physicsFrameTime = time.Since(startTime).Seconds()

			// Update simulation time
			writePlanet.Time += dt * e.simSpeed

			// Swap buffers for next frame
			e.SwapBuffers()

		case update := <-e.updateChan:
			// Handle parameter updates
			e.simSpeed = update.simSpeed
		}
	}
}

// GetPhysicsFrameTime returns the time taken for the last physics update
func (e *ThreadedPhysicsEngine) GetPhysicsFrameTime() float64 {
	return e.physicsFrameTime
}

// GetPhysicsUpdateInterval returns the fixed timestep interval for physics updates
func (e *ThreadedPhysicsEngine) GetPhysicsUpdateInterval() float64 {
	return 1.0 / e.physicsUpdateRate
}

// deepCopyPlanet creates a deep copy of the planet structure
func deepCopyPlanet(src *core.VoxelPlanet) *core.VoxelPlanet {
	dst := &core.VoxelPlanet{
		Shells:    make([]core.SphericalShell, len(src.Shells)),
		Radius:    src.Radius,
		Time:      src.Time,
		MeshDirty: src.MeshDirty,
		Physics:   src.Physics, // Physics state can be shared
	}

	// Deep copy each shell
	for i, srcShell := range src.Shells {
		dstShell := &dst.Shells[i]
		dstShell.InnerRadius = srcShell.InnerRadius
		dstShell.OuterRadius = srcShell.OuterRadius
		dstShell.LatBands = srcShell.LatBands
		dstShell.LonCounts = make([]int, len(srcShell.LonCounts))
		copy(dstShell.LonCounts, srcShell.LonCounts)

		// Deep copy voxels
		dstShell.Voxels = make([][]core.VoxelMaterial, len(srcShell.Voxels))
		for j, srcLatBand := range srcShell.Voxels {
			dstShell.Voxels[j] = make([]core.VoxelMaterial, len(srcLatBand))
			copy(dstShell.Voxels[j], srcLatBand)
		}
	}

	return dst
}

// ThreadedPhysicsInterface provides a simple interface for the main thread
type ThreadedPhysicsInterface struct {
	engine           *ThreadedPhysicsEngine
	lastUpdateTime   time.Time
	updateInterval   time.Duration
	lastReportedTime float64
}

// NewThreadedPhysicsInterface creates a new interface to the physics engine
func NewThreadedPhysicsInterface(planet *core.VoxelPlanet, gpuCompute gpu.GPUCompute, simSpeed float64) *ThreadedPhysicsInterface {
	engine := NewThreadedPhysicsEngine(planet, gpuCompute, simSpeed)
	engine.Start()

	return &ThreadedPhysicsInterface{
		engine:         engine,
		lastUpdateTime: time.Now(),
		updateInterval: 100 * time.Millisecond, // Update rendering data every 100ms
	}
}

// Update checks if new physics data is available
func (i *ThreadedPhysicsInterface) Update() (*core.VoxelPlanet, bool) {
	now := time.Now()
	if now.Sub(i.lastUpdateTime) >= i.updateInterval {
		i.lastUpdateTime = now
		planet := i.engine.GetCurrentPlanet()
		// Track update time
		if int(planet.Time/1e8) != int(i.lastReportedTime/1e8) {
			i.lastReportedTime = planet.Time
		}
		return planet, true
	}
	return nil, false
}

// Stop halts the physics engine
func (i *ThreadedPhysicsInterface) Stop() {
	i.engine.Stop()
}

// GetPhysicsFrameTime returns physics calculation time
func (i *ThreadedPhysicsInterface) GetPhysicsFrameTime() float64 {
	return i.engine.GetPhysicsFrameTime()
}

// UpdateSimSpeed updates the simulation speed multiplier
func (i *ThreadedPhysicsInterface) UpdateSimSpeed(speed float64) {
	i.engine.UpdateSimSpeed(speed)
}

// GetPhysicsUpdateInterval returns the fixed timestep interval for physics updates
func (i *ThreadedPhysicsInterface) GetPhysicsUpdateInterval() float64 {
	return i.engine.GetPhysicsUpdateInterval()
}
