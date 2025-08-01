package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var textureClients = make(map[*websocket.Conn]*sync.Mutex)
var textureClientsMutex sync.RWMutex

// startTextureServer starts the texture-based voxel server
func startTextureServer() {
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
	go textureSimulationLoop()
	
	// HTTP handlers
	http.HandleFunc("/", serveTextureHome)
	http.HandleFunc("/ws_texture", handleTextureWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))
	
	fmt.Printf("Texture server starting on http://localhost:%d\n", globalSettings.Server.Port)
	addr := fmt.Sprintf(":%d", globalSettings.Server.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// serveTextureHome serves the texture-based HTML page
func serveTextureHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index_texture.html")
}

// handleTextureWebSocket handles WebSocket connections for texture mode
func handleTextureWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()
	
	// Create mutex for this connection
	connMutex := &sync.Mutex{}
	textureClientsMutex.Lock()
	textureClients[conn] = connMutex
	textureClientsMutex.Unlock()
	
	// Clean up on disconnect
	defer func() {
		textureClientsMutex.Lock()
		delete(textureClients, conn)
		textureClientsMutex.Unlock()
		fmt.Printf("Client disconnected. Active clients: %d\n", len(textureClients))
	}()
	
	fmt.Printf("New texture client connected. Active clients: %d\n", len(textureClients))
	
	// Send initial texture data
	sendTextureData(conn)
	
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

// sendTextureData sends texture data to a specific client
func sendTextureData(conn *websocket.Conn) {
	textureClientsMutex.RLock()
	mutex, ok := textureClients[conn]
	textureClientsMutex.RUnlock()
	if !ok {
		return
	}
	
	textureData := globalPlanet.CreateTextureData()
	mutex.Lock()
	conn.WriteJSON(textureData)
	mutex.Unlock()
}

// textureSimulationLoop updates the planet and broadcasts texture data
func textureSimulationLoop() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(globalSettings.Server.UpdateIntervalMs))
	defer ticker.Stop()
	
	lastUpdate := time.Now()
	frameCount := 0
	lastFrameReport := time.Now()
	
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
		maxTimeStep := 3600.0 * 24.0 * 365.0 // 1 year max timestep
		stepsThisFrame := 0
		maxStepsPerFrame := 100 // Limit steps to prevent hanging
		
		for physicsTime > 0 && stepsThisFrame < maxStepsPerFrame {
			dt := min(physicsTime, maxTimeStep)
			globalPhysics.UpdatePhysics(dt)
			physicsTime -= dt
			stepsThisFrame++
		}
		
		// Update planet time
		globalPlanet.Time += yearsToSimulate
		
		// Frame rate tracking
		frameCount++
		if now.Sub(lastFrameReport) > time.Second {
			fps := float64(frameCount) / now.Sub(lastFrameReport).Seconds()
			fmt.Printf("Server FPS: %.1f, Time: %.1f My, Clients: %d\n", 
				fps, globalPlanet.Time/1000000, len(textureClients))
			frameCount = 0
			lastFrameReport = now
		}
		
		// Broadcast texture updates to clients
		broadcastTextureUpdate()
	}
}

// broadcastTextureUpdate sends texture data to all connected clients
func broadcastTextureUpdate() {
	textureData := globalPlanet.CreateTextureData()
	
	textureClientsMutex.RLock()
	clientCount := len(textureClients)
	connList := make([]*websocket.Conn, 0, clientCount)
	mutexes := make([]*sync.Mutex, 0, clientCount)
	for conn, mutex := range textureClients {
		connList = append(connList, conn)
		mutexes = append(mutexes, mutex)
	}
	textureClientsMutex.RUnlock()
	
	// Send to all clients in parallel
	for i, conn := range connList {
		mutex := mutexes[i]
		go func(c *websocket.Conn, m *sync.Mutex) {
			m.Lock()
			err := c.WriteJSON(textureData)
			m.Unlock()
			if err != nil {
				log.Println("WebSocket write error:", err)
				textureClientsMutex.Lock()
				delete(textureClients, c)
				textureClientsMutex.Unlock()
			}
		}(conn, mutex)
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}