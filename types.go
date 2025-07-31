package main

import rl "github.com/gen2brain/raylib-go/raylib"

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
	return sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func (v Vector3) Normalize() Vector3 {
	length := v.Length()
	if length == 0 {
		return Vector3{0, 0, 0}
	}
	return Vector3{v.X / length, v.Y / length, v.Z / length}
}

func (v Vector3) Dot(other Vector3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

type Vertex struct {
	Position    Vector3
	Height      float64
	PlateID     int
	Temperature float64
	Moisture    float64
}

type PlateType int

const (
	Continental PlateType = iota
	Oceanic
)

type Plate struct {
	ID           int
	Center       Vector3
	Velocity     Vector3
	Type         PlateType
	Color        rl.Color
	Vertices     []int // Indices of vertices belonging to this plate
	Boundaries   []PlateBoundary
}

type BoundaryType int

const (
	Convergent BoundaryType = iota
	Divergent
	Transform
)

type PlateBoundary struct {
	Plate1       int
	Plate2       int
	Type         BoundaryType
	EdgeVertices []int // Vertices along this boundary
}

type Planet struct {
	Vertices       []Vertex
	Indices        []int32
	Plates         []Plate
	Boundaries     []PlateBoundary
	TimeSpeed      float64
	GeologicalTime float64
	ShowWater      bool
	Hotspots       []Hotspot
	NeighborCache  map[int][]int // Cache vertex neighbors
}