Of course. Here is a summary of the server-side logic and development plan formatted as a `DESIGN.md` document.

-----

# DESIGN.md: Project Terra 🪐

This document outlines the design and development plan for a planetary simulation server. The project will be developed in Go, featuring a computation backend with graceful degradation (CUDA -\> MPS -\> CPU) and a phased development cycle that includes an initial local visualizer.

## Core Architecture

The server will be built around two main components: a **Simulation Core** and a swappable **Compute Backend**.

1.  **Simulation Core (Go):** The main application responsible for managing the simulation state, time steps, and orchestrating updates. It holds the planet's data structure but delegates heavy calculations.

2.  **Compute Backend (`interface`):** To handle graceful degradation, we'll use a Go interface. The Core will call methods on this interface, unaware of the underlying implementation (GPU or CPU).

    ```go
    // ComputeBackend defines the contract for heavy computation.
    type ComputeBackend interface {
        // Init initializes the backend with the planet's mesh data.
        Init(vertices []Vertex) error
        // TectonicUplift calculates mountain formation from plate pressure.
        TectonicUplift(plates []Plate)
        // HydraulicErosion simulates water flow and sediment transport.
        HydraulicErosion()
        // ... other simulation steps
    }
    ```

3.  **Local Visualizer:** For initial development and debugging, the server will **not** be headless. It will use a simple Go graphics library (**Raylib-go**) to render the simulation state directly in a window. This allows for immediate visual feedback before network logic is implemented.

## Data Representation

The planet will be represented as an **icosphere mesh**. This avoids pole distortion and provides evenly distributed vertices, ideal for simulation. Each vertex on the mesh will hold its own state.

```go
// Vertex represents a single point on the planet's surface.
type Vertex struct {
    Position      Vector3 // 3D coordinates
    Height        float64 // Elevation above/below sea level
    PlateID       int     // ID of the tectonic plate it belongs to
    Temperature   float64
    Moisture      float64
}
```

## Development Phases

The project will be built in stages, mimicking a planet's formation.

### Phase 1: The Molten Core

* **Objective:** Create the basic planet sphere and visualization framework.
* **Tasks:**
    1.  Implement icosphere generation logic.
    2.  Set up the local visualizer with **Raylib-go**.
    3.  Render a basic, rotating 3D icosphere.

-----

### Phase 2: The Crust Forms 🌋

* **Objective:** Generate the initial tectonic plates and continental landmasses.
* **Tasks:**
    1.  Implement **Voronoi diagrams** on the sphere's surface to define plate boundaries.
    2.  Assign properties to each plate (e.g., continental vs. oceanic).
    3.  Apply a noise function (e.g., Simplex noise) to create initial, non-simulated terrain.

-----

### Phase 3: The Earth Moves

* **Objective:** Simulate long-term tectonic movement to create realistic mountain ranges.
* **Tasks:**
    1.  Implement the `ComputeBackend` interface with a basic **CPU** version first.
    2.  Simulate plate drift and create uplift (mountains) at convergent boundaries.
    3.  Begin implementing the **CUDA** and **MPS** backends for GPU acceleration of these calculations.

-----

### Phase 4: The Rains Come 💧

* **Objective:** Simulate a basic climate and hydraulic erosion to create natural-looking landforms.
* **Tasks:**
    1.  Implement a simple climate model (temperature by latitude/altitude, prevailing winds).
    2.  Simulate rainfall and create rain shadows.
    3.  Implement a hydraulic erosion algorithm, heavily leveraging the `ComputeBackend` for performance. This step will transform the blocky mountains into realistic, eroded ranges.

-----

### Phase 5: The Great Divide 🌐

* **Objective:** Decouple the renderer into a separate client application.
* **Tasks:**
    1.  Remove the local **Raylib-go** visualizer from the server.
    2.  Implement a network layer (e.g., WebSockets or gRPC) to stream simplified world state.
    3.  The server is now a true headless application. A separate rendering client can be built in any technology (Unity, Godot, etc.) to connect to it.

## Technology Stack Summary

* **Language:** Go
* **GPU Backend (NVIDIA):** CUDA (via `gocuda` bindings)
* **GPU Backend (Apple):** Metal Performance Shaders (MPS)
* **CPU Fallback:** Standard Go with multi-threading (`goroutines`)
* **Initial Prototyping Renderer:** Raylib-go