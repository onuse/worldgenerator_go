# World Generator - Planet Evolution Simulator

A real-time planet evolution simulator with realistic tectonic plate simulation, written in Go with GPU acceleration.

## Features

- **Realistic Plate Tectonics**: Free-moving tectonic plates with proper subduction, collision, and rifting
- **GPU Acceleration**: Metal (macOS) and OpenCL backends for high-performance simulation
- **Real-time Visualization**: Web-based 3D rendering with Three.js
- **Advanced Geological Processes**: Volcanism, erosion, sedimentation, and isostatic adjustment
- **Configurable Resolution**: Support for different mesh resolutions via settings.json
- **Time Controls**: Simulate from 1 year/second to 100 million years/second

## Requirements

- Go 1.19 or higher
- macOS (for Metal backend) or system with OpenCL support
- Modern web browser with WebGL support

## Quick Start

1. Clone the repository
2. Configure settings (optional): Edit `settings.json`
3. Build and run:
   ```bash
   go build .
   ./worldgenerator
   ```
4. Open http://localhost:8080 in your browser

## Configuration

Edit `settings.json` to configure:
- `icosphereLevel`: Mesh resolution (5-8, higher = more detail)
- `port`: Server port (default: 8080)
- `updateIntervalMs`: Simulation update rate

See [README_SETTINGS.md](README_SETTINGS.md) for details.

## Project Structure

### Core Files
- `main.go` - Application entry point
- `types.go` - Core data structures
- `server.go` - WebSocket server and client communication
- `settings.go` - Configuration management

### Simulation
- `geometry.go` - Icosphere mesh generation
- `plates.go` - Tectonic plate generation and boundaries
- `tectonics.go` - Main tectonic simulation loop
- `realistic_plates_simple.go` - Realistic plate movement
- `continents.go` - Continent and terrain generation
- `volcanism.go` - Volcanic activity simulation
- `geological_processes.go` - Advanced geological features
- `erosion.go` - Erosion and weathering
- `smoothing.go` - Height smoothing and utilities
- `adaptive_time.go` - Adaptive timestep calculations

### GPU Backends
- `compute_backend.go` - GPU backend interface
- `metal_simple.go` - Metal (macOS) implementation
- `metal_stub.go` - Stub for non-macOS systems
- `opencl_compute.go` - OpenCL implementation
- `opencl_stub.go` - Stub for systems without OpenCL
- `shaders/` - GPU compute shaders

### Web Interface
- `web/index.html` - Main UI
- `web/static/terra.js` - 3D rendering and controls

## Controls

### Simulation Speed
- Click speed buttons to control simulation rate
- Current speed is highlighted and displayed

### View Controls
- **Mouse drag**: Rotate planet
- **Mouse wheel**: Zoom in/out
- **Water toggle**: Show/hide ocean
- **Heightmap mode**: Visualize elevation
- **Show Plates**: Color-code tectonic plates
- **Show Boundaries**: Display plate boundaries

## Performance

With GPU acceleration on Apple Silicon:
- Level 6 (40k vertices): 2-5ms per frame
- Level 7 (160k vertices): 10-20ms per frame
- Level 8 (650k vertices): 50-100ms per frame

## Development Notes

- See [DESIGN.md](DESIGN.md) for architecture details
- See [gpu_optimization_notes.md](gpu_optimization_notes.md) for GPU optimization opportunities