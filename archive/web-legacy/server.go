package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Global planet state
var globalPlanet *VoxelPlanet
var globalPhysics *VoxelPhysics
var clients = make(map[*websocket.Conn]*sync.Mutex)
var clientsMutex sync.RWMutex
var simSpeed = 10000.0 // years per second
var simSpeedMutex sync.RWMutex

// startServer starts the planet evolution server
func startVoxelServer() {
	// Load settings
	if err := loadSettings(); err != nil {
		log.Fatalf("Failed to load settings: %v", err)
	}
	
	// Initialize voxel planet
	fmt.Println("Initializing voxel planet...")
	startTime := time.Now()
	
	// Create planet with appropriate shell count based on detail level
	shellCount := 8 + globalSettings.Simulation.IcosphereLevel/2 // More shells for higher detail
	if shellCount > 15 {
		shellCount = 15 // Cap at reasonable level
	}
	
	globalPlanet = CreateVoxelPlanet(6371000, shellCount) // Earth radius
	
	fmt.Printf("Planet initialized in %.2fs\n", time.Since(startTime).Seconds())
	
	// Initialize physics system
	globalPhysics = NewVoxelPhysics(globalPlanet)
	
	// Initialize with some basic plate motion
	initializePlateMotion(globalPlanet)
	
	// Start simulation loop
	go simulationLoop()
	
	// HTTP handlers
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))
	
	fmt.Printf("Server starting on http://localhost:%d\n", globalSettings.Server.Port)
	addr := fmt.Sprintf(":%d", globalSettings.Server.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// initializePlateMotion sets up initial velocity fields
func initializePlateMotion(planet *VoxelPlanet) {
	// For demo purposes, create a simple, visible plate motion pattern
	// This is not realistic but will show the system working
	initializeSimpleDemoPlates(planet)
	return
	// Find surface shell
	surfaceShell := len(planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}
	
	shell := &planet.Shells[surfaceShell]
	
	// First, identify major continental and oceanic regions
	// Then create plate boundaries between them
	
	// Create several tectonic plates with different motion vectors
	plates := []struct {
		centerLat, centerLon float64
		velTheta, velPhi     float64 // Plate motion direction
		radius               float64 // Approximate plate size
		isOceanic           bool
	}{
		// Major plates - realistic velocities (cm/year converted to rad/s)
		// 1 cm/year ≈ 3e-10 m/s ≈ 5e-17 rad/s at Earth surface
		{20, -60, 1e-16, 2e-16, 40, false},    // North American plate
		{-20, -80, -1e-16, 1e-16, 35, false},  // South American plate
		{30, 20, -1e-16, -2e-16, 45, false},   // Eurasian plate
		{0, 30, 2e-16, 1e-16, 40, false},      // African plate
		{-30, 130, 1e-16, 3e-16, 30, false},   // Australian plate
		{0, -140, -2e-16, -3e-16, 50, true},   // Pacific plate (oceanic)
		{-60, 0, 1e-16, 1e-16, 25, false},     // Antarctic plate
		{20, 80, 3e-16, -1e-16, 25, false},    // Indian plate
		{40, -140, -1e-16, 2e-16, 20, true},   // Juan de Fuca (oceanic)
		{0, -30, 2e-16, -2e-16, 15, true},     // Mid-Atlantic (oceanic)
	}
	
	// Assign plate velocities based on nearest plate center
	for latIdx := range shell.Voxels {
		lat := getLatitudeForBand(latIdx, shell.LatBands)
		
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Skip non-crustal materials
			if voxel.Type != MatGranite && voxel.Type != MatBasalt {
				continue
			}
			
			lon := float64(lonIdx) / float64(shell.LonCounts[latIdx]) * 360.0 - 180.0
			
			// Find nearest plate
			minDist := 1e10
			var nearestPlate int
			
			for i, plate := range plates {
				// Calculate angular distance
				dlat := lat - plate.centerLat
				dlon := lon - plate.centerLon
				// Handle longitude wrapping
				if dlon > 180 {
					dlon -= 360
				} else if dlon < -180 {
					dlon += 360
				}
				
				dist := math.Sqrt(dlat*dlat + dlon*dlon*math.Cos(lat*math.Pi/180)*math.Cos(lat*math.Pi/180))
				
				if dist < minDist && dist < plate.radius {
					minDist = dist
					nearestPlate = i
				}
			}
			
			// Assign plate velocity
			if minDist < 1e9 {
				plate := plates[nearestPlate]
				voxel.VelTheta = float32(plate.velTheta)
				voxel.VelPhi = float32(plate.velPhi)
				
				// Add some random variation
				voxel.VelTheta += float32((math.Sin(float64(latIdx)*0.1) * 0.00001))
				voxel.VelPhi += float32((math.Cos(float64(lonIdx)*0.1) * 0.00001))
				
				// Mark plate boundaries with higher temperatures
				// Check if near edge of plate
				edgeDist := plate.radius - minDist
				if edgeDist < 5.0 {
					voxel.Temperature += float32(200 * (5.0 - edgeDist) / 5.0)
				}
			}
		}
	}
	
	// Create some specific features
	
	// Mid-Atlantic Ridge
	for latIdx := 10; latIdx < shell.LatBands-10; latIdx++ {
		lat := getLatitudeForBand(latIdx, shell.LatBands)
		ridgeLon := -30 + 10*math.Sin(lat*0.05) // Sinuous ridge
		
		targetLon := int((ridgeLon + 180.0) * float64(shell.LonCounts[latIdx]) / 360.0)
		if targetLon >= 0 && targetLon < shell.LonCounts[latIdx] {
			for dLon := -3; dLon <= 3; dLon++ {
				lonIdx := (targetLon + dLon + shell.LonCounts[latIdx]) % shell.LonCounts[latIdx]
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				if voxel.Type == MatBasalt {
					// Create spreading center
					voxel.VelR = 0.00002 // Upwelling
					voxel.Temperature += 300
					voxel.Age = 0 // New crust
					
					// Divergent motion
					if dLon > 0 {
						voxel.VelPhi = 0.00003
					} else if dLon < 0 {
						voxel.VelPhi = -0.00003
					}
				}
			}
		}
	}
	
	// Ring of Fire subduction zones
	ringOfFire := []struct{ lat, lon float64 }{
		{60, -150}, {50, -140}, {40, -130}, {30, -120}, // Eastern Pacific
		{20, -110}, {10, -100}, {0, -90}, {-10, -80},   // Central America
		{-20, -75}, {-30, -70}, {-40, -72}, {-50, -75}, // South America
		{40, 145}, {30, 140}, {20, 130}, {10, 125},     // Western Pacific
		{0, 120}, {-10, 115}, {-20, 110}, {-30, 105},   // Indonesia
	}
	
	for _, point := range ringOfFire {
		targetLat := int((point.lat + 90.0) * float64(shell.LatBands) / 180.0)
		if targetLat < 0 || targetLat >= shell.LatBands {
			continue
		}
		
		targetLon := int((point.lon + 180.0) * float64(shell.LonCounts[targetLat]) / 360.0)
		
		for dLat := -2; dLat <= 2; dLat++ {
			latIdx := targetLat + dLat
			if latIdx < 0 || latIdx >= shell.LatBands {
				continue
			}
			
			for dLon := -2; dLon <= 2; dLon++ {
				lonIdx := (targetLon + dLon + shell.LonCounts[latIdx]) % shell.LonCounts[latIdx]
				voxel := &shell.Voxels[latIdx][lonIdx]
				
				if voxel.Type == MatBasalt {
					// Create subduction
					voxel.VelR = -0.00002 // Downward
					voxel.Temperature -= 100 // Cooler subducting slab
				}
			}
		}
	}
	
	fmt.Println("Realistic plate configuration initialized with:")
	fmt.Printf("- %d major tectonic plates\n", len(plates))
	fmt.Println("- Mid-Atlantic Ridge spreading center")
	fmt.Printf("- Ring of Fire with %d subduction points\n", len(ringOfFire))
}

// initializeSimpleDemoPlates creates a simple demo with visible plate motion
func initializeSimpleDemoPlates(planet *VoxelPlanet) {
	surfaceShell := len(planet.Shells) - 2
	if surfaceShell < 0 {
		return
	}
	
	shell := &planet.Shells[surfaceShell]
	
	// Create a simple east-west drift pattern
	// This will make continents visibly move over time
	for latIdx := range shell.Voxels {
		for lonIdx := range shell.Voxels[latIdx] {
			voxel := &shell.Voxels[latIdx][lonIdx]
			
			// Skip non-crustal materials
			if voxel.Type != MatGranite && voxel.Type != MatBasalt {
				continue
			}
			
			// Simple pattern: everything drifts east
			// But at different rates based on latitude
			lat := getLatitudeForBand(latIdx, shell.LatBands)
			
			// Faster at equator, slower at poles
			speedFactor := math.Cos(lat * math.Pi / 180.0)
			
			// Set velocity in m/s (not rad/s)
			// This is about 10 cm/year at the equator
			voxel.VelPhi = float32(3e-9 * speedFactor)
			
			// Add some north-south variation
			voxel.VelTheta = float32(1e-9 * math.Sin(float64(lonIdx) * 0.1))
			
			// No vertical motion for now
			voxel.VelR = 0
		}
	}
	
	fmt.Println("Simple demo plate motion initialized")
	fmt.Println("- All plates drift eastward at ~10 cm/year")
	fmt.Println("- Motion visible at 10+ Myr/s speeds")
}

// simulationLoop updates the planet
func simulationLoop() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(globalSettings.Server.UpdateIntervalMs))
	defer ticker.Stop()
	
	lastUpdate := time.Now()
	
	for range ticker.C {
		// Get current simulation speed
		simSpeedMutex.RLock()
		currentSpeed := simSpeed
		simSpeedMutex.RUnlock()
		
		// Calculate actual elapsed time to handle lag
		now := time.Now()
		deltaTime := now.Sub(lastUpdate).Seconds()
		lastUpdate = now
		
		// Calculate years to simulate
		yearsToSimulate := currentSpeed * deltaTime
		
		// Convert to seconds for physics (1 year = 365.25 * 24 * 3600 seconds)
		physicsTime := yearsToSimulate * 365.25 * 24 * 3600
		
		// Update physics with reasonable timestep
		maxTimeStep := 3600.0 * 24.0 * 365.0 // 1 year max timestep (was 1 day)
		stepsThisFrame := 0
		maxStepsPerFrame := 100 // Limit steps to prevent hanging
		
		for physicsTime > 0 && stepsThisFrame < maxStepsPerFrame {
			dt := math.Min(physicsTime, maxTimeStep)
			globalPhysics.UpdatePhysics(dt)
			physicsTime -= dt
			stepsThisFrame++
		}
		
		if stepsThisFrame >= maxStepsPerFrame {
			fmt.Printf("⚠️  Physics update limited to %d steps (%.0f years remaining)\n", 
				maxStepsPerFrame, physicsTime/365.25/24/3600)
		}
		
		// Update planet time
		globalPlanet.Time += yearsToSimulate
		
		// Debug: Print update info
		if int(globalPlanet.Time/100000) > int((globalPlanet.Time-yearsToSimulate)/100000) {
			fmt.Printf("⏰ Simulation time: %.2f Million years (speed: %.0f yr/s)\n", 
				globalPlanet.Time/1000000, currentSpeed)
			
			// Sample a surface voxel to see if it's moving
			if len(globalPlanet.Shells) > 2 {
				shell := &globalPlanet.Shells[len(globalPlanet.Shells)-2]
				if len(shell.Voxels) > 50 && len(shell.Voxels[50]) > 50 {
					voxel := &shell.Voxels[50][50]
					fmt.Printf("  Sample voxel: VelPhi=%.9f m/s, Type=%d, Age=%.1f My\n", 
						voxel.VelPhi, voxel.Type, voxel.Age/1000000)
					
					// Check a few adjacent voxels to see pattern
					for i := 48; i <= 52; i++ {
						if i >= 0 && i < len(shell.Voxels[50]) {
							v := &shell.Voxels[50][i]
							fmt.Printf("    Lon %d: Type=%d, Age=%.1f My\n", 
								i, v.Type, v.Age/1000000)
						}
					}
				}
			}
		}
		
		// Broadcast updates to clients
		broadcastUpdate()
	}
}

// handleWebSocket handles WebSocket connections
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()
	
	// Create mutex for this connection
	connMutex := &sync.Mutex{}
	clientsMutex.Lock()
	clients[conn] = connMutex
	clientsMutex.Unlock()
	
	// Clean up on disconnect
	defer func() {
		clientsMutex.Lock()
		delete(clients, conn)
		clientsMutex.Unlock()
	}()
	
	// Send initial mesh data
	sendMeshData(conn)
	
	// Handle incoming messages (speed controls, etc.)
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		
		// Handle speed changes
		if speed, ok := msg["timeSpeed"].(float64); ok {
			simSpeedMutex.Lock()
			simSpeed = speed
			simSpeedMutex.Unlock()
			fmt.Printf("Simulation speed changed to: %f years/second\n", speed)
		}
	}
}

// sendMeshData sends mesh extracted from planet data
func sendMeshData(conn *websocket.Conn) {
	clientsMutex.RLock()
	mutex, ok := clients[conn]
	clientsMutex.RUnlock()
	if !ok {
		return
	}
	
	meshData := createMeshData()
	mutex.Lock()
	conn.WriteJSON(meshData)
	mutex.Unlock()
}

// createMeshData converts planet to mesh format expected by frontend
func createMeshData() MeshData {
	// Extract surface mesh from planet
	mesh := globalPlanet.SimplifiedSurfaceExtraction()
	
	// Convert to format expected by frontend
	vertices := make([][3]float64, len(mesh.Vertices))
	heights := make([]float64, len(mesh.Vertices))
	plateIds := make([]int, len(mesh.Vertices))
	temperatures := make([]float64, len(mesh.Vertices))
	crustalAges := make([]float64, len(mesh.Vertices))
	rockTypes := make([]int, len(mesh.Vertices))
	
	for i, v := range mesh.Vertices {
		vertices[i] = [3]float64{v.X, v.Y, v.Z}
		
		// Calculate height from radius
		radius := v.Length()
		heights[i] = (radius - 1.0) / 3.0 // Convert to height units
		
		// Sample voxel data at this position
		lat := math.Asin(float64(v.Y)) * 180.0 / math.Pi
		lon := math.Atan2(float64(v.Z), float64(v.X)) * 180.0 / math.Pi
		
		latIdx := int((lat + 90.0) * float64(len(globalPlanet.Shells)-1) / 180.0)
		lonIdx := int((lon + 180.0) * float64(len(globalPlanet.Shells)-1) / 360.0)
		
		if latIdx >= 0 && latIdx < 180 && lonIdx >= 0 && lonIdx < 360 {
			// Get surface voxel
			voxel, _ := globalPlanet.GetSurfaceVoxel(latIdx, lonIdx)
			if voxel != nil {
				temperatures[i] = float64(voxel.Temperature)
				crustalAges[i] = float64(voxel.Age) / 1000000.0 // Convert to My
				rockTypes[i] = int(voxel.Type)
				
				// Fake plate ID based on position for now
				plateIds[i] = (latIdx/20 + lonIdx/30) % 12
			}
		}
	}
	
	// Get real plate boundaries
	plateBoundaries := globalPhysics.mechanics.DetectPlateBoundaries()
	boundaries := []BoundaryData{}
	
	// Convert plate boundaries to mesh boundary data
	for _, pb := range plateBoundaries {
		// Map voxel coordinates to mesh vertices
		// This is approximate - we're finding the nearest vertex
		lat := getLatitudeForBand(pb.LatIdx, len(globalPlanet.Shells)-1)
		lon := float64(pb.LonIdx) / float64(globalPlanet.Shells[len(globalPlanet.Shells)-2].LonCounts[pb.LatIdx]) * 360.0 - 180.0
		
		// Find nearest vertex
		nearestIdx := -1
		minDist := float64(1e10)
		for i, v := range mesh.Vertices {
			// Convert vertex to lat/lon
			vLat := math.Asin(float64(v.Y)) * 180.0 / math.Pi
			vLon := math.Atan2(float64(v.Z), float64(v.X)) * 180.0 / math.Pi
			
			dist := math.Sqrt((vLat-lat)*(vLat-lat) + (vLon-lon)*(vLon-lon))
			if dist < minDist {
				minDist = dist
				nearestIdx = i
			}
		}
		
		if nearestIdx >= 0 {
			color := "#888888"
			switch pb.Type {
			case "divergent":
				color = "#00ff00"
			case "convergent":
				color = "#ff0000"
			case "transform":
				color = "#ffff00"
			}
			
			boundaries = append(boundaries, BoundaryData{
				Type:     pb.Type,
				Vertices: []int{nearestIdx},
				Color:    color,
			})
		}
	}
	
	// Get current simulation speed
	simSpeedMutex.RLock()
	currentSpeed := simSpeed
	simSpeedMutex.RUnlock()
	
	// Convert int32 indices to int
	indices := make([]int, len(mesh.Triangles))
	for i, idx := range mesh.Triangles {
		indices[i] = int(idx)
	}
	
	return MeshData{
		Type:         "mesh_update",
		Vertices:     vertices,
		Indices:      indices,
		Heights:      heights,
		PlateIDs:     plateIds,
		Temperatures: temperatures,
		CrustalAges:  crustalAges,
		RockTypes:    rockTypes,
		SeaLevel:     0.0,
		Boundaries:   boundaries,
		Time:         globalPlanet.Time,
		TimeSpeed:    currentSpeed,
	}
}

// Client represents a connected websocket client
type Client struct {
	conn  *websocket.Conn
	mutex *sync.Mutex
	mode  string // "mesh" or "texture"
}

var clientsV2 = make(map[*Client]bool)

// broadcastUpdate sends updates to all connected clients
func broadcastUpdate() {
	meshData := createMeshData()
	
	clientsMutex.RLock()
	clientCount := len(clients)
	connList := make([]*websocket.Conn, 0, clientCount)
	mutexes := make([]*sync.Mutex, 0, clientCount)
	for conn, mutex := range clients {
		connList = append(connList, conn)
		mutexes = append(mutexes, mutex)
	}
	clientsMutex.RUnlock()
	
	// Debug: log broadcast info occasionally
	if int(globalPlanet.Time) % 1000000 == 0 {
		if clientCount > 0 {
			fmt.Printf("📡 Broadcasting to %d clients at %.1f My\n", clientCount, globalPlanet.Time/1000000)
		} else {
			fmt.Printf("⚠️  No clients connected at %.1f My\n", globalPlanet.Time/1000000)
		}
	}
	
	for i, conn := range connList {
		mutex := mutexes[i]
		go func(c *websocket.Conn, m *sync.Mutex) {
			m.Lock()
			err := c.WriteJSON(meshData)
			m.Unlock()
			if err != nil {
				log.Println("WebSocket write error:", err)
				clientsMutex.Lock()
				delete(clients, c)
				clientsMutex.Unlock()
			}
		}(conn, mutex)
	}
}