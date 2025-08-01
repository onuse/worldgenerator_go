package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type MeshData struct {
	Type       string      `json:"type"`
	Vertices   [][3]float64 `json:"vertices"`
	Indices    []int32     `json:"indices"`
	Heights    []float64   `json:"heights"`
	PlateIDs   []int       `json:"plateIds"`
	Boundaries []BoundaryData `json:"boundaries"`
	Time       float64     `json:"time"`
	TimeSpeed  float64     `json:"timeSpeed"`
}

type BoundaryData struct {
	Type     string `json:"type"`
	Vertices []int  `json:"vertices"`
	Color    string `json:"color"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

var globalPlanet Planet
var clients = make(map[*websocket.Conn]*sync.Mutex)
var clientsMutex sync.RWMutex

func startServer() {
	// Initialize planet with high resolution that browsers can handle
	globalPlanet = generateIcosphere(5) // Level 5 = ~10,000 vertices - good balance of detail/performance
	globalPlanet = generateTectonicPlates(globalPlanet, 8)
	globalPlanet.TimeSpeed = 10000.0 // Start at higher speed for visible changes
	globalPlanet.ShowWater = true
	
	fmt.Printf("Planet initialized with %d plates\n", len(globalPlanet.Plates))
	for i, plate := range globalPlanet.Plates {
		plateType := "Continental"
		if plate.Type == Oceanic {
			plateType = "Oceanic"
		}
		fmt.Printf("Plate %d: %s, Velocity(%.6f, %.6f, %.6f)\n", i, plateType, plate.Velocity.X, plate.Velocity.Y, plate.Velocity.Z)
	}
	
	// Check height distribution
	minH, maxH := globalPlanet.Vertices[0].Height, globalPlanet.Vertices[0].Height
	for _, v := range globalPlanet.Vertices {
		if v.Height < minH { minH = v.Height }
		if v.Height > maxH { maxH = v.Height }
	}
	fmt.Printf("Height range: %.6f to %.6f\n", minH, maxH)

	// Start simulation loop
	go simulationLoop()

	// HTTP handlers
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	connMutex := &sync.Mutex{}
	clientsMutex.Lock()
	clients[conn] = connMutex
	clientsMutex.Unlock()
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

		if speed, ok := msg["timeSpeed"].(float64); ok {
			fmt.Printf("SPEED CHANGE: %.0f yr/s -> %.0f yr/s\n", globalPlanet.TimeSpeed, speed)
			globalPlanet.TimeSpeed = speed
		}
		if showWater, ok := msg["showWater"].(bool); ok {
			fmt.Printf("SHOW WATER: %v\n", showWater)
			globalPlanet.ShowWater = showWater
		}
	}
}

func simulationLoop() {
	ticker := time.NewTicker(time.Millisecond * 100) // 10fps updates
	defer ticker.Stop()
	
	updateCount := 0
	lastPrintTime := time.Now()

	for range ticker.C {
		frameStart := time.Now()
		
		// Update simulation
		var simTime time.Duration
		if globalPlanet.TimeSpeed > 0 {
			// Calculate how many years pass in this update (100ms)
			yearsPerUpdate := globalPlanet.TimeSpeed / 10.0 // 10 updates per second
			
			// Optimize for different speed ranges
			simStart := time.Now()
			
			// Always do single update to keep things responsive
			// The tectonic simulation will handle large time steps internally
			globalPlanet = computeBackend.UpdateTectonics(globalPlanet, yearsPerUpdate)
			
			simTime = time.Since(simStart)
			
			// Debug output every 10 updates to see if simulation is running
			updateCount++
			
			// Measure simulation time
			simTime := time.Since(frameStart)
			
			// Print timing info every second
			if time.Since(lastPrintTime) > time.Second {
				lastPrintTime = time.Now()
				
				// Check if heights are changing
				minH, maxH := globalPlanet.Vertices[0].Height, globalPlanet.Vertices[0].Height
				for _, v := range globalPlanet.Vertices {
					if v.Height < minH { minH = v.Height }
					if v.Height > maxH { maxH = v.Height }
				}
				
				fmt.Printf("TIMING: SimTime=%v, Time=%.1f My, Speed=%.0f yr/s, YearsPerUpdate=%.0f, Heights=[%.4f,%.4f], Plates=%d\n", 
					simTime, globalPlanet.GeologicalTime/1000000.0, globalPlanet.TimeSpeed, yearsPerUpdate, minH, maxH, len(globalPlanet.Plates))
			}
		}

		// Broadcast to all clients
		broadcastStart := time.Now()
		broadcastMeshData()
		broadcastTime := time.Since(broadcastStart)
		
		// Total frame time
		totalTime := time.Since(frameStart)
		if totalTime > 90*time.Millisecond {
			fmt.Printf("SLOW FRAME: Total=%v (Sim=%v, Broadcast=%v)\n", totalTime, simTime, broadcastTime)
		}
	}
}

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

func broadcastMeshData() {
	meshData := createMeshData()
	clientsMutex.RLock()
	clientsToRemove := []*websocket.Conn{}
	for client, mutex := range clients {
		mutex.Lock()
		err := client.WriteJSON(meshData)
		mutex.Unlock()
		if err != nil {
			log.Println("WebSocket write error:", err)
			client.Close()
			clientsToRemove = append(clientsToRemove, client)
		}
	}
	clientsMutex.RUnlock()
	
	// Remove failed clients
	if len(clientsToRemove) > 0 {
		clientsMutex.Lock()
		for _, client := range clientsToRemove {
			delete(clients, client)
		}
		clientsMutex.Unlock()
	}
}

func createMeshData() MeshData {
	vertices := make([][3]float64, len(globalPlanet.Vertices))
	heights := make([]float64, len(globalPlanet.Vertices))
	plateIds := make([]int, len(globalPlanet.Vertices))

	for i, v := range globalPlanet.Vertices {
		// Apply height displacement to create 3D terrain
		// IMPORTANT: Position should already be normalized on unit sphere
		normalized := v.Position // Already normalized
		radius := 1.0 + v.Height*3.0 // Height displacement for visualization
		vertices[i] = [3]float64{
			normalized.X * radius,
			normalized.Y * radius,
			normalized.Z * radius,
		}
		heights[i] = v.Height
		plateIds[i] = v.PlateID
	}

	// Convert plate boundaries for visualization
	boundaries := make([]BoundaryData, len(globalPlanet.Boundaries))
	for i, b := range globalPlanet.Boundaries {
		var boundaryType string
		var color string
		
		switch b.Type {
		case Convergent:
			boundaryType = "convergent"
			color = "#ff0000" // Red
		case Divergent:
			boundaryType = "divergent"
			color = "#0000ff" // Blue
		case Transform:
			boundaryType = "transform"
			color = "#ffff00" // Yellow
		default:
			boundaryType = "unknown"
			color = "#ffffff" // White
		}
		
		boundaries[i] = BoundaryData{
			Type:     boundaryType,
			Vertices: b.EdgeVertices,
			Color:    color,
		}
	}

	return MeshData{
		Type:       "mesh_update",
		Vertices:   vertices,
		Indices:    globalPlanet.Indices,
		Heights:    heights,
		PlateIDs:   plateIds,
		Boundaries: boundaries,
		Time:       globalPlanet.GeologicalTime,
		TimeSpeed:  globalPlanet.TimeSpeed,
	}
}