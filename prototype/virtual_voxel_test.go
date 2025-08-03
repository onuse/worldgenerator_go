package prototype

import (
	"fmt"
	"testing"
)

// TestVirtualVoxel2D tests the 2D prototype
func TestVirtualVoxel2D(t *testing.T) {
	// Create small test world
	world := NewVirtual2DWorld(40, 20)
	
	// Create single continent
	world.CreateContinent(20, 10, 5, 1)
	
	// Set it moving
	world.SetPlateVelocity(1, 1.0, 0.0)
	
	// Update a few times
	for i := 0; i < 10; i++ {
		world.UpdatePhysics(0.1)
	}
	
	// Check that voxels moved
	moved := false
	for _, voxel := range world.VirtualVoxels {
		if voxel.Position.X > 21 { // Should have moved right
			moved = true
			break
		}
	}
	
	if !moved {
		t.Error("Virtual voxels did not move as expected")
	}
	
	// Check grid mapping
	hasLand := false
	for y := 0; y < world.Height; y++ {
		for x := 0; x < world.Width; x++ {
			if world.Grid[y][x].Type == 1 {
				hasLand = true
				break
			}
		}
	}
	
	if !hasLand {
		t.Error("No land found in grid after mapping")
	}
}

// TestBondPhysics tests spring connections
func TestBondPhysics(t *testing.T) {
	world := NewVirtual2DWorld(50, 50)
	
	// Create two connected voxels
	voxel1 := Virtual2DVoxel{
		ID:       1,
		Mass:     1.0,
		Material: 1,
	}
	voxel1.Position.X = 25
	voxel1.Position.Y = 25
	voxel1.GridWeights = make(map[GridCell]float64)
	
	voxel2 := Virtual2DVoxel{
		ID:       2,
		Mass:     1.0,
		Material: 1,
	}
	voxel2.Position.X = 26
	voxel2.Position.Y = 25
	voxel2.GridWeights = make(map[GridCell]float64)
	
	// Create bond
	bond1 := VoxelBond2D{
		TargetID:   2,
		RestLength: 1.0,
		Stiffness:  10.0,
		Strength:   0.5,
	}
	bond2 := VoxelBond2D{
		TargetID:   1,
		RestLength: 1.0,
		Stiffness:  10.0,
		Strength:   0.5,
	}
	
	voxel1.Bonds = append(voxel1.Bonds, bond1)
	voxel2.Bonds = append(voxel2.Bonds, bond2)
	
	world.VirtualVoxels = append(world.VirtualVoxels, voxel1, voxel2)
	world.VoxelMap[1] = &world.VirtualVoxels[0]
	world.VoxelMap[2] = &world.VirtualVoxels[1]
	
	// Pull them apart
	world.VirtualVoxels[1].Position.X = 28 // Stretch the bond
	
	// Update physics
	world.UpdatePhysics(0.01)
	
	// Check that forces were applied
	if world.VirtualVoxels[0].Force.X <= 0 {
		t.Error("No rightward force on voxel 1 from stretched bond")
	}
	if world.VirtualVoxels[1].Force.X >= 0 {
		t.Error("No leftward force on voxel 2 from stretched bond")
	}
}

// Example usage for documentation
func Example() {
	// Create a simple world
	world := NewVirtual2DWorld(60, 20)
	
	// Add a continent
	world.CreateContinent(30, 10, 5, 1)
	
	// Set it in motion
	world.SetPlateVelocity(1, 0.2, 0.0)
	
	// Simulate
	for i := 0; i < 50; i++ {
		world.UpdatePhysics(0.1)
	}
	
	// The continent will have moved smoothly to the right
	// without any grid artifacts or discrete jumping
	fmt.Println("Simulation complete")
	// Output: Simulation complete
}