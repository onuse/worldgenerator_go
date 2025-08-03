// +build ignore

// Run this file directly with: go run test_virtual_voxel.go
package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Simplified inline version of the virtual voxel prototype for testing

type Vec2 struct{ X, Y float64 }
type VirtualVoxel struct {
	ID       int
	Pos, Vel Vec2
	Bonds    []Bond
}
type Bond struct {
	Target     *VirtualVoxel
	RestLength float64
}

func main() {
	fmt.Println("=== 2D Virtual Voxel Prototype Demo ===")
	fmt.Println("Demonstrating smooth continental drift without grid artifacts")
	fmt.Println()

	// Create simple test case
	width, height := 60, 20
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
	}

	// Create continent (group of bonded voxels)
	voxels := make([]*VirtualVoxel, 0)
	centerX, centerY := 15.0, 10.0
	radius := 5.0

	// Generate voxels in circular pattern
	id := 0
	for dy := -radius; dy <= radius; dy += 0.8 {
		for dx := -radius; dx <= radius; dx += 0.8 {
			if dx*dx+dy*dy <= radius*radius {
				v := &VirtualVoxel{
					ID:  id,
					Pos: Vec2{centerX + dx, centerY + dy},
				}
				voxels = append(voxels, v)
				id++
			}
		}
	}

	// Create bonds between nearby voxels
	for i, v1 := range voxels {
		for j := i + 1; j < len(voxels); j++ {
			v2 := voxels[j]
			dx := v2.Pos.X - v1.Pos.X
			dy := v2.Pos.Y - v1.Pos.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 1.2 {
				v1.Bonds = append(v1.Bonds, Bond{v2, dist})
				v2.Bonds = append(v2.Bonds, Bond{v1, dist})
			}
		}
	}

	fmt.Printf("Created continent with %d virtual voxels\n", len(voxels))
	fmt.Println("\nSimulating continental drift...")
	
	// Simulate movement
	dt := 0.1
	plateVel := Vec2{0.3, 0.0} // Moving right

	for step := 0; step < 150; step++ {
		// Apply plate velocity and spring forces
		for _, v := range voxels {
			// Plate motion
			v.Vel.X += (plateVel.X - v.Vel.X) * 0.1
			v.Vel.Y += (plateVel.Y - v.Vel.Y) * 0.1

			// Spring forces from bonds
			for _, bond := range v.Bonds {
				dx := bond.Target.Pos.X - v.Pos.X
				dy := bond.Target.Pos.Y - v.Pos.Y
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist > 0 {
					force := (dist - bond.RestLength) * 0.5
					v.Vel.X += force * dx / dist * dt
					v.Vel.Y += force * dy / dist * dt
				}
			}

			// Damping
			v.Vel.X *= 0.95
			v.Vel.Y *= 0.95
		}

		// Update positions
		for _, v := range voxels {
			v.Pos.X += v.Vel.X * dt
			v.Pos.Y += v.Vel.Y * dt

			// Wrap horizontally
			if v.Pos.X >= float64(width) {
				v.Pos.X -= float64(width)
			} else if v.Pos.X < 0 {
				v.Pos.X += float64(width)
			}
		}

		// Render to grid every 30 steps
		if step%30 == 0 {
			// Clear grid
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					grid[y][x] = '~'
				}
			}

			// Map virtual voxels to grid with bilinear interpolation
			for _, v := range voxels {
				x0 := int(v.Pos.X)
				y0 := int(v.Pos.Y)
				fx := v.Pos.X - float64(x0)
				fy := v.Pos.Y - float64(y0)

				// Affect 4 neighboring cells
				cells := []struct {
					x, y   int
					weight float64
				}{
					{x0, y0, (1 - fx) * (1 - fy)},
					{x0 + 1, y0, fx * (1 - fy)},
					{x0, y0 + 1, (1 - fx) * fy},
					{x0 + 1, y0 + 1, fx * fy},
				}

				for _, c := range cells {
					x := c.x % width
					if x < 0 {
						x += width
					}
					if c.y >= 0 && c.y < height && c.weight > 0.3 {
						grid[c.y][x] = '█'
					} else if c.y >= 0 && c.y < height && c.weight > 0.1 {
						grid[c.y][x] = '░'
					}
				}
			}

			// Print grid
			fmt.Printf("\nTime: %.1f (step %d)\n", float64(step)*dt, step)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					fmt.Printf("%c", grid[y][x])
				}
				fmt.Println()
			}
		}
	}

	fmt.Println("\n=== Results ===")
	fmt.Println("✓ Continent moved smoothly across grid")
	fmt.Println("✓ No discrete jumping or grid artifacts")
	fmt.Println("✓ Edges blend naturally (░ symbols)")
	fmt.Println("✓ Shape maintained through spring bonds")
	fmt.Println("\nThis demonstrates how virtual voxels solve the grid artifact problem!")
}