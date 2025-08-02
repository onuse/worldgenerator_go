package main

import (
	"math"
	"github.com/go-gl/gl/v4.3-core/gl"
	"unsafe"
)

// createSphere creates a UV sphere mesh and stores it in the renderer
func (r *VoxelRenderer) createSphere() {
	const (
		segments = 64
		rings    = 32
	)
	
	// Generate vertices
	var vertices []float32
	var indices []uint32
	
	// Create vertices
	for ring := 0; ring <= rings; ring++ {
		theta := float32(ring) * math.Pi / float32(rings)
		sinTheta := float32(math.Sin(float64(theta)))
		cosTheta := float32(math.Cos(float64(theta)))
		
		for seg := 0; seg <= segments; seg++ {
			phi := float32(seg) * 2.0 * math.Pi / float32(segments)
			sinPhi := float32(math.Sin(float64(phi)))
			cosPhi := float32(math.Cos(float64(phi)))
			
			// Position
			x := cosPhi * sinTheta
			y := cosTheta
			z := sinPhi * sinTheta
			
			// Scale to planet radius
			vertices = append(vertices, x*r.planetRadius, y*r.planetRadius, z*r.planetRadius)
			
			// Normal (same as position for unit sphere)
			vertices = append(vertices, x, y, z)
			
			// Texture coordinates
			u := float32(seg) / float32(segments)
			v := float32(ring) / float32(rings)
			vertices = append(vertices, u, v)
		}
	}
	
	// Create indices
	for ring := 0; ring < rings; ring++ {
		for seg := 0; seg < segments; seg++ {
			current := uint32(ring*(segments+1) + seg)
			next := current + uint32(segments) + 1
			
			// First triangle
			indices = append(indices, current, next, current+1)
			
			// Second triangle
			indices = append(indices, current+1, next, next+1)
		}
	}
	
	// Create VAO
	gl.GenVertexArrays(1, &r.sphereVAO)
	gl.BindVertexArray(r.sphereVAO)
	
	// Create VBO
	gl.GenBuffers(1, &r.sphereVBO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.sphereVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)
	
	// Position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 8*4, unsafe.Pointer(uintptr(0)))
	gl.EnableVertexAttribArray(0)
	
	// Normal attribute
	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, 8*4, unsafe.Pointer(uintptr(3*4)))
	gl.EnableVertexAttribArray(1)
	
	// TexCoord attribute
	gl.VertexAttribPointer(2, 2, gl.FLOAT, false, 8*4, unsafe.Pointer(uintptr(6*4)))
	gl.EnableVertexAttribArray(2)
	
	// Create EBO
	gl.GenBuffers(1, &r.sphereEBO)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.sphereEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, unsafe.Pointer(&indices[0]), gl.STATIC_DRAW)
	
	r.sphereIndexCount = int32(len(indices))
	
	// Unbind
	gl.BindVertexArray(0)
}
