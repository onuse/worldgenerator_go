package opengl

import (
	"fmt"
	"math"
	"github.com/go-gl/mathgl/mgl32"
	"worldgenerator/core"
	"worldgenerator/simulation"
)

// HandleMouseClick performs ray casting to select plates
func (r *VoxelRenderer) HandleMouseClick(xpos, ypos float64, planet *core.VoxelPlanet) {
	// Only handle clicks in plate mode
	if r.RenderMode != 4 || planet.Physics == nil {
		return
	}
	// TODO: Properly type assert physics system
	// vp, ok := planet.Physics.(*physics.VoxelPhysics)
	// if !ok || vp.plates == nil {
	//     return
	// }
	
	// Convert screen coordinates to NDC
	x := (2.0*float32(xpos))/float32(r.width) - 1.0
	y := 1.0 - (2.0*float32(ypos))/float32(r.height) // Flip Y
	
	// Create ray from camera
	invViewProj := r.projMatrix.Mul4(r.viewMatrix).Inv()
	
	// Near and far points in NDC
	nearPoint := mgl32.Vec4{x, y, -1.0, 1.0}
	farPoint := mgl32.Vec4{x, y, 1.0, 1.0}
	
	// Transform to world space
	nearWorld := invViewProj.Mul4x1(nearPoint)
	farWorld := invViewProj.Mul4x1(farPoint)
	
	// Perspective divide
	nearWorld = nearWorld.Mul(1.0 / nearWorld[3])
	farWorld = farWorld.Mul(1.0 / farWorld[3])
	
	// Ray origin and direction
	rayOrigin := mgl32.Vec3{nearWorld[0], nearWorld[1], nearWorld[2]}
	rayDir := mgl32.Vec3{
		farWorld[0] - nearWorld[0],
		farWorld[1] - nearWorld[1],
		farWorld[2] - nearWorld[2],
	}.Normalize()
	
	// Perform ray-sphere intersection
	hitPoint, hit := r.raySphereIntersect(rayOrigin, rayDir, r.planetRadius)
	if !hit {
		return
	}
	
	// Find which voxel/plate was hit
	plateID := r.findPlateAtPosition(hitPoint, planet)
	if plateID > 0 {
		r.selectedPlateID = plateID
		// TODO: Properly type assert and access plates
		// if vp, ok := planet.Physics.(*physics.VoxelPhysics); ok {
		//     r.displayPlateInfo(plateID, vp.plates)
		// }
	}
}

// raySphereIntersect performs ray-sphere intersection
func (r *VoxelRenderer) raySphereIntersect(origin, dir mgl32.Vec3, radius float32) (mgl32.Vec3, bool) {
	oc := origin
	a := dir.Dot(dir)
	b := 2.0 * oc.Dot(dir)
	c := oc.Dot(oc) - radius*radius
	discriminant := b*b - 4*a*c
	
	if discriminant < 0 {
		return mgl32.Vec3{}, false
	}
	
	sqrtD := float32(math.Sqrt(float64(discriminant)))
	t0 := (-b - sqrtD) / (2.0 * a)
	t1 := (-b + sqrtD) / (2.0 * a)
	
	// Use the closer positive intersection
	t := t0
	if t < 0 {
		t = t1
		if t < 0 {
			return mgl32.Vec3{}, false
		}
	}
	
	hitPoint := origin.Add(dir.Mul(t))
	return hitPoint, true
}

// findPlateAtPosition finds which plate contains the given 3D position
func (r *VoxelRenderer) findPlateAtPosition(pos mgl32.Vec3, planet *core.VoxelPlanet) int {
	// Convert 3D position to spherical coordinates
	normalized := pos.Normalize()
	
	lat := math.Asin(float64(normalized[2])) * 180.0 / math.Pi       // -90 to 90
	lon := math.Atan2(float64(normalized[1]), float64(normalized[0])) * 180.0 / math.Pi // -180 to 180
	
	// Find the surface shell
	surfaceShell := len(planet.Shells) - 2
	if surfaceShell < 0 {
		return 0
	}
	
	shell := &planet.Shells[surfaceShell]
	
	// Convert to voxel indices
	latBandF := (lat + 90.0) / 180.0 * float64(shell.LatBands)
	latBand := int(math.Floor(latBandF))
	if latBand >= shell.LatBands {
		latBand = shell.LatBands - 1
	}
	if latBand < 0 {
		latBand = 0
	}
	
	// Get longitude index
	lonCount := len(shell.Voxels[latBand])
	lonNorm := (lon + 180.0) / 360.0
	lonIdx := int(lonNorm * float64(lonCount))
	lonIdx = lonIdx % lonCount
	if lonIdx < 0 {
		lonIdx += lonCount
	}
	
	// Look up plate ID
	// TODO: Properly type assert and access plates
	// coord := core.VoxelCoord{Shell: surfaceShell, Lat: latBand, Lon: lonIdx}
	// if vp, ok := planet.Physics.(*physics.VoxelPhysics); ok && vp.plates != nil {
	//     if plateID, exists := vp.plates.VoxelPlateMap[coord]; exists {
	//         return plateID
	//     }
	// }
	
	return 0
}

// displayPlateInfo shows information about the selected plate
func (r *VoxelRenderer) displayPlateInfo(plateID int, plateManager *simulation.PlateManager) {
	// Find the plate
	var plate *simulation.TectonicPlate
	for _, p := range plateManager.Plates {
		if p.ID == plateID {
			plate = p
			break
		}
	}
	
	if plate == nil {
		return
	}
	
	fmt.Printf("\n=== PLATE INFORMATION ===\n")
	fmt.Printf("Plate ID: %d\n", plate.ID)
	fmt.Printf("Name: %s\n", plate.Name)
	fmt.Printf("Type: %s\n", plate.Type)
	fmt.Printf("Size: %d voxels\n", len(plate.MemberVoxels))
	fmt.Printf("Boundary voxels: %d\n", len(plate.BoundaryVoxels))
	fmt.Printf("Average age: %.2f million years\n", plate.AverageAge/1e6)
	fmt.Printf("Average thickness: %.1f km\n", plate.AverageThickness/1000)
	
	// Motion information
	fmt.Printf("\n--- Motion ---\n")
	fmt.Printf("Euler pole: %.1f°N, %.1f°E\n", plate.EulerPoleLat, plate.EulerPoleLon)
	fmt.Printf("Angular velocity: %.2e rad/year\n", plate.AngularVelocity)
	surfaceVel := math.Abs(plate.AngularVelocity) * 6371000 // Earth radius
	fmt.Printf("Surface velocity: %.1f cm/year\n", surfaceVel*100)
	
	// Forces
	fmt.Printf("\n--- Forces ---\n")
	fmt.Printf("Ridge push: %.2e N\n", magnitude(plate.RidgePushForce))
	fmt.Printf("Slab pull: %.2e N\n", magnitude(plate.SlabPullForce))
	fmt.Printf("Basal drag: %.2e N\n", magnitude(plate.BasalDragForce))
	fmt.Printf("Collision: %.2e N\n", magnitude(plate.CollisionForce))
	fmt.Printf("========================\n\n")
}

// magnitude calculates vector magnitude
func magnitude(v core.Vector3) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}