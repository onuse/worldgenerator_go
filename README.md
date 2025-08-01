# World Generator - Voxel Planet Evolution Simulator

A voxel-based planet evolution simulator written in Go, featuring realistic geological processes, mantle convection, and plate tectonics.

## Features

- **Voxel-based architecture**: True 3D planet structure with material flow
- **Temperature dynamics**: Heat diffusion, solar heating, and radioactive decay
- **Material physics**: Phase transitions, pressure calculation, and density variations
- **Mantle convection**: Temperature-driven convection cells that drive plate motion
- **Real-time visualization**: Web-based 3D renderer using Three.js
- **Time controls**: Simulate from years to millions of years per second

## Architecture

The simulator uses a spherical voxel grid with exponentially-spaced shells from core to surface. Each voxel tracks:
- Material type (granite, basalt, water, magma, etc.)
- Temperature and pressure
- Density and velocity
- Age and composition

## Getting Started

1. Install Go 1.19 or later
2. Clone the repository
3. Run `go build .`
4. Start the server: `./worldgenerator`
5. Open http://localhost:8080 in your browser

## Configuration

Edit `settings.json` to adjust:
- `icosphereLevel`: Planet detail level (default: 7)
- `updateIntervalMs`: Simulation update rate
- `broadcastIntervalMs`: Frontend update rate
- `port`: Server port

## Development Roadmap

See `VOXEL_ROADMAP.md` for the complete development plan, including:
- Phase 1: Foundation ✅
- Phase 2: Material Physics ✅
- Phase 3: Mantle Convection (in progress)
- Phase 4: Plate Tectonics
- Phase 5: Surface Processes
- And more...

## Technical Details

- **Backend**: Go with goroutines for concurrent physics simulation
- **Frontend**: JavaScript/Three.js for 3D rendering
- **Communication**: WebSocket for real-time updates
- **Physics**: Finite difference methods for heat diffusion and material advection

## License

MIT