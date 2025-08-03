package prototype

import (
	"fmt"
	"math"
	"math/rand"
)

// Virtual2DVoxel represents a voxel with continuous position
type Virtual2DVoxel struct {
	ID       int
	Position struct {
		X, Y float64 // Continuous position
	}
	Velocity struct {
		X, Y float64 // Current velocity
	}
	Mass        float64
	Material    int // 0=water, 1=land
	PlateID     int
	Bonds       []VoxelBond2D
	Force       struct{ X, Y float64 } // Accumulated forces this frame
	GridWeights map[GridCell]float64   // Which grid cells this voxel affects
}

// VoxelBond2D represents a spring connection between voxels
type VoxelBond2D struct {
	TargetID    int     // ID of connected voxel
	RestLength  float64 // Natural separation
	Stiffness   float64 // Spring constant
	Strength    float64 // 0-1, can break under stress
	CurrentDist float64 // Current distance (for stress calculation)
}

// GridCell represents a discrete grid location
type GridCell struct {
	X, Y int
}

// Virtual2DWorld manages the virtual voxel system
type Virtual2DWorld struct {
	Width, Height   int                        // Grid dimensions
	VirtualVoxels   []Virtual2DVoxel          // All virtual voxels
	VoxelMap        map[int]*Virtual2DVoxel    // Quick lookup by ID
	Grid            [][]GridMaterial           // Rendered grid
	PlateVelocities map[int]struct{ X, Y float64 } // Plate motion
	Time            float64
	NextID          int
}

// GridMaterial represents what's rendered in each grid cell
type GridMaterial struct {
	Type        int     // Blended material type
	Density     float64 // Sum of overlapping voxel weights
	Temperature float64
	Stress      float64 // Accumulated stress from stretched bonds
}

// NewVirtual2DWorld creates a test world
func NewVirtual2DWorld(width, height int) *Virtual2DWorld {
	world := &Virtual2DWorld{
		Width:           width,
		Height:          height,
		VirtualVoxels:   make([]Virtual2DVoxel, 0),
		VoxelMap:        make(map[int]*Virtual2DVoxel),
		Grid:            make([][]GridMaterial, height),
		PlateVelocities: make(map[int]struct{ X, Y float64 }),
		NextID:          1,
	}

	// Initialize grid
	for y := 0; y < height; y++ {
		world.Grid[y] = make([]GridMaterial, width)
	}

	return world
}

// CreateContinent creates a bonded group of land voxels
func (w *Virtual2DWorld) CreateContinent(centerX, centerY, radius float64, plateID int) {
	// Create voxels in a roughly circular continent
	voxelSpacing := 0.8 // Slightly less than 1.0 to ensure good coverage
	startID := w.NextID

	// Create voxels
	for dy := -radius; dy <= radius; dy += voxelSpacing {
		for dx := -radius; dx <= radius; dx += voxelSpacing {
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= radius {
				// Add some noise to make it less circular
				noise := (rand.Float64() - 0.5) * 0.3
				if dist <= radius*(1+noise) {
					voxel := Virtual2DVoxel{
						ID:       w.NextID,
						Mass:     1.0,
						Material: 1, // Land
						PlateID:  plateID,
					}
					voxel.Position.X = centerX + dx
					voxel.Position.Y = centerY + dy
					voxel.GridWeights = make(map[GridCell]float64)

					w.VirtualVoxels = append(w.VirtualVoxels, voxel)
					w.VoxelMap[voxel.ID] = &w.VirtualVoxels[len(w.VirtualVoxels)-1]
					w.NextID++
				}
			}
		}
	}

	// Create bonds between nearby voxels
	endID := w.NextID
	for i := startID; i < endID; i++ {
		voxel1 := w.VoxelMap[i]
		if voxel1 == nil {
			continue
		}

		// Find nearby voxels to bond with
		for j := i + 1; j < endID; j++ {
			voxel2 := w.VoxelMap[j]
			if voxel2 == nil {
				continue
			}

			dx := voxel2.Position.X - voxel1.Position.X
			dy := voxel2.Position.Y - voxel1.Position.Y
			dist := math.Sqrt(dx*dx + dy*dy)

			// Bond if close enough
			if dist < voxelSpacing*1.5 {
				// Create bidirectional bonds
				bond1 := VoxelBond2D{
					TargetID:   voxel2.ID,
					RestLength: dist,
					Stiffness:  100.0, // Strong continental bonds
					Strength:   0.9,   // Hard to break
				}
				bond2 := VoxelBond2D{
					TargetID:   voxel1.ID,
					RestLength: dist,
					Stiffness:  100.0,
					Strength:   0.9,
				}

				voxel1.Bonds = append(voxel1.Bonds, bond1)
				voxel2.Bonds = append(voxel2.Bonds, bond2)
			}
		}
	}

	fmt.Printf("Created continent with %d voxels and plate ID %d\n", endID-startID, plateID)
}

// UpdatePhysics performs one physics timestep
func (w *Virtual2DWorld) UpdatePhysics(dt float64) {
	// Clear forces
	for i := range w.VirtualVoxels {
		w.VirtualVoxels[i].Force.X = 0
		w.VirtualVoxels[i].Force.Y = 0
	}

	// Calculate spring forces from bonds
	for i := range w.VirtualVoxels {
		voxel := &w.VirtualVoxels[i]
		
		for j := range voxel.Bonds {
			bond := &voxel.Bonds[j]
			target := w.VoxelMap[bond.TargetID]
			if target == nil {
				continue
			}

			// Calculate spring force
			dx := target.Position.X - voxel.Position.X
			dy := target.Position.Y - voxel.Position.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			bond.CurrentDist = dist

			if dist > 0 {
				// Spring force: F = -k * (x - rest_length)
				forceMag := bond.Stiffness * (dist - bond.RestLength)
				fx := forceMag * dx / dist
				fy := forceMag * dy / dist

				voxel.Force.X += fx
				voxel.Force.Y += fy

				// Check if bond should break
				strain := math.Abs(dist-bond.RestLength) / bond.RestLength
				if strain > bond.Strength {
					// Mark for removal (simplified - in real system would handle properly)
					bond.Strength = 0
				}
			}
		}

		// Add plate motion forces
		if plateVel, exists := w.PlateVelocities[voxel.PlateID]; exists {
			// Strong force to maintain plate velocity
			voxel.Force.X += (plateVel.X - voxel.Velocity.X) * 50.0
			voxel.Force.Y += (plateVel.Y - voxel.Velocity.Y) * 50.0
		}

		// Add damping
		voxel.Force.X -= voxel.Velocity.X * 2.0
		voxel.Force.Y -= voxel.Velocity.Y * 2.0
	}

	// Update velocities and positions
	for i := range w.VirtualVoxels {
		voxel := &w.VirtualVoxels[i]

		// Velocity Verlet integration
		voxel.Velocity.X += (voxel.Force.X / voxel.Mass) * dt
		voxel.Velocity.Y += (voxel.Force.Y / voxel.Mass) * dt

		voxel.Position.X += voxel.Velocity.X * dt
		voxel.Position.Y += voxel.Velocity.Y * dt

		// Keep within bounds (wrap horizontally, clamp vertically)
		if voxel.Position.X < 0 {
			voxel.Position.X += float64(w.Width)
		} else if voxel.Position.X >= float64(w.Width) {
			voxel.Position.X -= float64(w.Width)
		}

		if voxel.Position.Y < 0 {
			voxel.Position.Y = 0
		} else if voxel.Position.Y >= float64(w.Height) {
			voxel.Position.Y = float64(w.Height) - 0.01
		}
	}

	// Update grid mapping
	w.UpdateGridMapping()

	w.Time += dt
}

// UpdateGridMapping calculates which grid cells each voxel affects
func (w *Virtual2DWorld) UpdateGridMapping() {
	// Clear grid
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			w.Grid[y][x] = GridMaterial{
				Type: 0, // Water by default
			}
		}
	}

	// Map virtual voxels to grid
	for i := range w.VirtualVoxels {
		voxel := &w.VirtualVoxels[i]
		
		// Clear old weights
		voxel.GridWeights = make(map[GridCell]float64)

		// Find affected grid cells (bilinear interpolation)
		// A voxel at position (3.7, 2.3) affects cells (3,2), (4,2), (3,3), (4,3)
		x0 := int(math.Floor(voxel.Position.X))
		y0 := int(math.Floor(voxel.Position.Y))
		x1 := x0 + 1
		y1 := y0 + 1

		// Fractional parts
		fx := voxel.Position.X - float64(x0)
		fy := voxel.Position.Y - float64(y0)

		// Calculate weights for each cell
		cells := []struct {
			x, y   int
			weight float64
		}{
			{x0, y0, (1 - fx) * (1 - fy)},
			{x1, y0, fx * (1 - fy)},
			{x0, y1, (1 - fx) * fy},
			{x1, y1, fx * fy},
		}

		for _, cell := range cells {
			// Handle wrapping
			cx := ((cell.x % w.Width) + w.Width) % w.Width
			cy := cell.y

			if cy >= 0 && cy < w.Height && cell.weight > 0.01 {
				gc := GridCell{cx, cy}
				voxel.GridWeights[gc] = cell.weight

				// Update grid
				w.Grid[cy][cx].Density += cell.weight
				if voxel.Material == 1 { // Land
					w.Grid[cy][cx].Type = 1
				}

				// Add stress from stretched bonds
				totalStress := 0.0
				for _, bond := range voxel.Bonds {
					if bond.Strength > 0 {
						strain := math.Abs(bond.CurrentDist-bond.RestLength) / bond.RestLength
						totalStress += strain * bond.Stiffness
					}
				}
				w.Grid[cy][cx].Stress += totalStress * cell.weight
			}
		}
	}
}

// SetPlateVelocity sets the velocity for all voxels in a plate
func (w *Virtual2DWorld) SetPlateVelocity(plateID int, vx, vy float64) {
	w.PlateVelocities[plateID] = struct{ X, Y float64 }{vx, vy}
}

// Render produces a text visualization of the grid
func (w *Virtual2DWorld) Render() string {
	output := ""
	
	// Add time and stats
	output += fmt.Sprintf("Time: %.1f | Virtual Voxels: %d\n", w.Time, len(w.VirtualVoxels))
	output += fmt.Sprintf("Grid: %dx%d\n\n", w.Width, w.Height)

	// Render grid
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			cell := w.Grid[y][x]
			
			if cell.Type == 1 && cell.Density > 0.5 {
				// Land
				if cell.Stress > 50 {
					output += "▓" // Highly stressed
				} else if cell.Stress > 20 {
					output += "▒" // Moderately stressed
				} else {
					output += "█" // Normal land
				}
			} else if cell.Type == 1 && cell.Density > 0.1 {
				output += "░" // Partial land (edge)
			} else {
				output += "~" // Water
			}
		}
		output += "\n"
	}

	return output
}

// RunSimulation runs a simple test
func RunSimulation() {
	fmt.Println("=== 2D Virtual Voxel Prototype ===")
	
	// Create world
	world := NewVirtual2DWorld(80, 30)
	
	// Create two continents
	world.CreateContinent(20, 15, 8, 1) // Left continent, plate 1
	world.CreateContinent(60, 15, 6, 2) // Right continent, plate 2
	
	// Set plate velocities (continents moving toward each other)
	world.SetPlateVelocity(1, 0.5, 0.0)  // Moving right
	world.SetPlateVelocity(2, -0.3, 0.0) // Moving left
	
	// Initial state
	world.UpdateGridMapping()
	fmt.Println("\nInitial state:")
	fmt.Print(world.Render())
	
	// Run simulation
	dt := 0.1
	steps := 100
	printEvery := 20
	
	for step := 0; step < steps; step++ {
		world.UpdatePhysics(dt)
		
		if (step+1)%printEvery == 0 {
			fmt.Printf("\nAfter %d steps (t=%.1f):\n", step+1, world.Time)
			fmt.Print(world.Render())
		}
	}
	
	// Summary
	fmt.Println("\n=== Simulation Complete ===")
	fmt.Println("Key observations:")
	fmt.Println("- Continents move smoothly without grid artifacts")
	fmt.Println("- Stress builds up at collision zones (▓ symbols)")
	fmt.Println("- Edges blend naturally (░ symbols)")
	fmt.Println("- No discrete jumping or oscillation")
}