package core

import (
	"math"
	"net/http"
	"github.com/gorilla/websocket"
)

// Websocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Vector3 represents a 3D vector
type Vector3 struct {
	X, Y, Z float64
}

func (v Vector3) Add(other Vector3) Vector3 {
	return Vector3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

func (v Vector3) Scale(s float64) Vector3 {
	return Vector3{v.X * s, v.Y * s, v.Z * s}
}

func (v Vector3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func (v Vector3) Normalize() Vector3 {
	length := v.Length()
	if length == 0 {
		return Vector3{0, 0, 0}
	}
	return Vector3{v.X / length, v.Y / length, v.Z / length}
}

// MeshData is sent to the frontend for rendering
type MeshData struct {
	Type         string        `json:"type"`
	Vertices     [][3]float64  `json:"vertices"`
	Indices      []int         `json:"indices"`
	Heights      []float64     `json:"heights"`
	PlateIDs     []int         `json:"plateIds"`
	Temperatures []float64     `json:"temperatures"`
	CrustalAges  []float64     `json:"crustalAges"`
	RockTypes    []int         `json:"rockTypes"`
	SeaLevel     float64       `json:"seaLevel"`
	Boundaries   []BoundaryData `json:"boundaries"`
	Time         float64       `json:"time"`
	TimeSpeed    float64       `json:"timeSpeed"`
}

// BoundaryData represents a plate boundary for rendering
type BoundaryData struct {
	Type     string `json:"type"`
	Vertices []int  `json:"vertices"`
	Color    string `json:"color"`
}

// serveHome serves the main HTML page
func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

