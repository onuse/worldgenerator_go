class TerraRenderer {
    constructor() {
        this.scene = new THREE.Scene();
        this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
        this.renderer = new THREE.WebGLRenderer({ antialias: true });
        this.controls = null;
        this.planetMesh = null;
        this.boundaryMesh = null;
        this.showWater = true;
        this.heightmapMode = false;
        
        this.init();
        this.connectWebSocket();
    }

    init() {
        // Setup renderer
        this.renderer.setSize(window.innerWidth, window.innerHeight);
        this.renderer.setClearColor(0x000000);
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        document.getElementById('container').appendChild(this.renderer.domElement);

        // Setup camera
        this.camera.position.set(0, 0, 3);

        // Add orbital controls (simplified mouse controls)
        this.setupControls();

        // Add enhanced lighting for terrain detail
        const ambientLight = new THREE.AmbientLight(0x404040, 0.2); // Lower ambient
        this.scene.add(ambientLight);

        // Primary directional light
        const directionalLight = new THREE.DirectionalLight(0xffffff, 1.0);
        directionalLight.position.set(2, 2, 1);
        directionalLight.castShadow = true;
        directionalLight.shadow.mapSize.width = 2048;
        directionalLight.shadow.mapSize.height = 2048;
        this.scene.add(directionalLight);
        
        // Secondary light for fill
        const fillLight = new THREE.DirectionalLight(0x8888ff, 0.3);
        fillLight.position.set(-1, -1, -0.5);
        this.scene.add(fillLight);
        
        // Rim light for edge definition
        const rimLight = new THREE.DirectionalLight(0xffffaa, 0.4);
        rimLight.position.set(0, 0, -2);
        this.scene.add(rimLight);

        // Handle window resize
        window.addEventListener('resize', () => this.onWindowResize());

        // Start render loop
        this.animate();
    }

    setupControls() {
        // Improved mouse controls that preserve zoom
        let isMouseDown = false;
        let mouseX = 0, mouseY = 0;
        let rotationX = 0, rotationY = 0;
        let distance = 3; // Current zoom distance

        this.renderer.domElement.addEventListener('mousedown', (e) => {
            isMouseDown = true;
            mouseX = e.clientX;
            mouseY = e.clientY;
        });

        this.renderer.domElement.addEventListener('mousemove', (e) => {
            if (!isMouseDown) return;
            
            const deltaX = e.clientX - mouseX;
            const deltaY = e.clientY - mouseY;
            
            rotationY += deltaX * 0.01;
            rotationX += deltaY * 0.01;
            
            rotationX = Math.max(-Math.PI/2, Math.min(Math.PI/2, rotationX));
            
            // Use current distance instead of fixed 3
            this.camera.position.x = distance * Math.sin(rotationY) * Math.cos(rotationX);
            this.camera.position.y = distance * Math.sin(rotationX);
            this.camera.position.z = distance * Math.cos(rotationY) * Math.cos(rotationX);
            
            this.camera.lookAt(0, 0, 0);
            
            mouseX = e.clientX;
            mouseY = e.clientY;
        });

        this.renderer.domElement.addEventListener('mouseup', () => {
            isMouseDown = false;
        });

        // Zoom with mouse wheel - update distance variable
        this.renderer.domElement.addEventListener('wheel', (e) => {
            const scale = e.deltaY > 0 ? 1.1 : 0.9;
            distance *= scale;
            distance = Math.max(1.2, Math.min(8, distance)); // Clamp zoom range
            
            // Apply new distance
            this.camera.position.x = distance * Math.sin(rotationY) * Math.cos(rotationX);
            this.camera.position.y = distance * Math.sin(rotationX);
            this.camera.position.z = distance * Math.cos(rotationY) * Math.cos(rotationX);
        });
    }

    connectWebSocket() {
        const ws = new WebSocket(`ws://${window.location.host}/ws`);
        
        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            console.log('Received data:', data.type, 'Time:', data.time, 'Speed:', data.timeSpeed);
            if (data.type === 'mesh_update') {
                this.updateMesh(data);
                this.updateUI(data);
                this.lastData = data; // Store for mode changes
            }
        };

        ws.onopen = () => {
            console.log('Connected to Terra server');
            document.getElementById('connection').textContent = 'Connected';
            document.getElementById('connection').style.color = 'green';
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            document.getElementById('connection').textContent = 'Error';
            document.getElementById('connection').style.color = 'red';
        };
        
        ws.onclose = () => {
            console.log('WebSocket closed');
            document.getElementById('connection').textContent = 'Disconnected';
            document.getElementById('connection').style.color = 'red';
            document.getElementById('time').textContent = '--';
            document.getElementById('speed').textContent = '--';
        };

        this.ws = ws;
    }

    updateMesh(data) {
        // Remove existing mesh
        if (this.planetMesh) {
            this.scene.remove(this.planetMesh);
        }
        if (this.boundaryMesh) {
            this.scene.remove(this.boundaryMesh);
        }

        // Create geometry
        const geometry = new THREE.BufferGeometry();
        
        // Convert vertices
        const vertices = new Float32Array(data.vertices.length * 3);
        const colors = new Float32Array(data.vertices.length * 3);
        
        for (let i = 0; i < data.vertices.length; i++) {
            vertices[i * 3] = data.vertices[i][0];
            vertices[i * 3 + 1] = data.vertices[i][1];
            vertices[i * 3 + 2] = data.vertices[i][2];
            
            // Color based on height or plate
            const color = this.getVertexColor(data.heights[i], data.plateIds[i]);
            colors[i * 3] = color.r;
            colors[i * 3 + 1] = color.g;
            colors[i * 3 + 2] = color.b;
        }

        geometry.setAttribute('position', new THREE.BufferAttribute(vertices, 3));
        geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
        geometry.setIndex(Array.from(data.indices));
        geometry.computeVertexNormals();

        // Create material with enhanced shading
        const material = new THREE.MeshPhongMaterial({ 
            vertexColors: true,
            side: THREE.DoubleSide,
            shininess: 10,
            specular: 0x111111,
            flatShading: false // Smooth shading for subtle terrain
        });

        // Create mesh
        this.planetMesh = new THREE.Mesh(geometry, material);
        this.scene.add(this.planetMesh);

        // Skip boundary visualization - not needed for terrain view
        this.addBoundaries(data);
    }

    getVertexColor(height, plateId) {
        if (this.plateViewMode) {
            // Color by plate ID
            const colors = [
                { r: 0.8, g: 0.2, b: 0.2 }, // Red
                { r: 0.2, g: 0.8, b: 0.2 }, // Green
                { r: 0.2, g: 0.2, b: 0.8 }, // Blue
                { r: 0.8, g: 0.8, b: 0.2 }, // Yellow
                { r: 0.8, g: 0.2, b: 0.8 }, // Magenta
                { r: 0.2, g: 0.8, b: 0.8 }, // Cyan
                { r: 0.8, g: 0.5, b: 0.2 }, // Orange
                { r: 0.5, g: 0.2, b: 0.8 }, // Purple
                { r: 0.5, g: 0.8, b: 0.5 }, // Light green
                { r: 0.8, g: 0.5, b: 0.5 }, // Pink
                { r: 0.5, g: 0.5, b: 0.8 }, // Light blue
                { r: 0.8, g: 0.8, b: 0.5 }  // Light yellow
            ];
            const color = colors[plateId % colors.length];
            // Darken underwater areas
            if (height < 0 && this.showWater) {
                return {
                    r: color.r * 0.3,
                    g: color.g * 0.3,
                    b: color.b * 0.3
                };
            }
            return color;
        } else if (this.heightmapMode) {
            // Pure heightmap coloring
            const normalized = (height + 0.1) / 0.2; // Normalize height range
            const clamped = Math.max(0, Math.min(1, normalized));
            return {
                r: clamped,
                g: 0.5,
                b: 1 - clamped
            };
        } else {
            // Realistic terrain coloring
            const seaLevel = -0.005; // Match actual ocean depths
            
            if (height < -0.008 && this.showWater) {
                // Deep ocean trenches - dark blue
                return { r: 0.0, g: 0.1, b: 0.3 };
            } else if (height < seaLevel && this.showWater) {
                // Shallow ocean - medium blue
                const depth = Math.abs(height / 0.008);
                return { r: 0.1, g: 0.3 + depth * 0.2, b: 0.6 + depth * 0.3 };
            } else if (height < seaLevel && !this.showWater) {
                // Ocean floor - dark brown
                return { r: 0.3, g: 0.2, b: 0.1 };
            } else if (height < 0.002) {
                // Coastal plains - light green
                return { r: 0.4, g: 0.7, b: 0.3 };
            } else if (height < 0.006) {
                // Lowlands - medium green
                return { r: 0.2, g: 0.6, b: 0.2 };
            } else if (height < 0.010) {
                // Hills - darker green/brown
                return { r: 0.3, g: 0.5, b: 0.1 };
            } else if (height < 0.015) {
                // Mountains - brown
                return { r: 0.6, g: 0.4, b: 0.2 };
            } else {
                // High peaks - white/snow
                const snowLevel = Math.min(1, (height - 0.015) / 0.005);
                return { 
                    r: 0.6 + snowLevel * 0.4, 
                    g: 0.4 + snowLevel * 0.6, 
                    b: 0.2 + snowLevel * 0.8 
                };
            }
        }
    }

    addBoundaries(data) {
        // Create boundary lines
        const boundaryGeometry = new THREE.BufferGeometry();
        const positions = [];
        const colors = [];

        data.boundaries.forEach(boundary => {
            boundary.vertices.forEach(vertexIndex => {
                if (vertexIndex < data.vertices.length) {
                    const vertex = data.vertices[vertexIndex];
                    positions.push(vertex[0] * 1.01, vertex[1] * 1.01, vertex[2] * 1.01); // Slightly above surface
                    
                    // Color based on boundary type
                    if (boundary.color === '#ff0000') {
                        colors.push(1, 0, 0); // Red for convergent
                    } else if (boundary.color === '#0000ff') {
                        colors.push(0, 0, 1); // Blue for divergent
                    } else {
                        colors.push(1, 1, 0); // Yellow for transform
                    }
                }
            });
        });

        boundaryGeometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
        boundaryGeometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));

        const boundaryMaterial = new THREE.PointsMaterial({ 
            size: 0.01, 
            vertexColors: true 
        });

        this.boundaryMesh = new THREE.Points(boundaryGeometry, boundaryMaterial);
        this.scene.add(this.boundaryMesh);
    }

    updateUI(data) {
        const timeInMy = (data.time / 1000000).toFixed(1);
        const speedText = data.timeSpeed.toFixed(0);
        console.log('Updating UI - Time:', timeInMy, 'My, Speed:', speedText, 'years/sec');
        document.getElementById('time').textContent = timeInMy + ' My';
        document.getElementById('speed').textContent = speedText + ' years/sec';
    }

    animate() {
        requestAnimationFrame(() => this.animate());
        this.renderer.render(this.scene, this.camera);
    }

    onWindowResize() {
        this.camera.aspect = window.innerWidth / window.innerHeight;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(window.innerWidth, window.innerHeight);
    }
    
    toggleHeightmap() {
        this.heightmapMode = !this.heightmapMode;
        this.plateViewMode = false; // Disable plate view
        // Re-render with new mode
        if (this.lastData) {
            this.updateMesh(this.lastData);
        }
    }
    
    togglePlateView() {
        this.plateViewMode = !this.plateViewMode;
        this.heightmapMode = false; // Disable heightmap
        // Re-render with new mode
        if (this.lastData) {
            this.updateMesh(this.lastData);
        }
    }
    
    toggleBoundaries() {
        this.showBoundaries = !this.showBoundaries;
        if (this.boundaryMesh) {
            this.boundaryMesh.visible = this.showBoundaries;
        }
    }
}

// Global functions for UI controls
let terra;

function setSpeed(speed) {
    console.log('setSpeed called with:', speed);
    if (!terra) {
        console.error('Terra not initialized yet, trying again in 100ms');
        setTimeout(() => setSpeed(speed), 100);
        return;
    }
    if (terra.ws && terra.ws.readyState === WebSocket.OPEN) {
        console.log('Sending speed to server:', speed);
        terra.ws.send(JSON.stringify({ timeSpeed: speed }));
    } else {
        console.error('WebSocket not ready. State:', terra.ws ? terra.ws.readyState : 'no ws');
    }
}

function toggleWater() {
    terra.showWater = !terra.showWater;
    document.getElementById('waterBtn').textContent = 'Water: ' + (terra.showWater ? 'ON' : 'OFF');
    if (terra && terra.ws) {
        terra.ws.send(JSON.stringify({ showWater: terra.showWater }));
    }
}

function toggleHeightmap() {
    if (terra) {
        terra.toggleHeightmap();
    }
}

function togglePlateView() {
    if (terra) {
        terra.togglePlateView();
    }
}

function toggleBoundaries() {
    if (terra) {
        terra.toggleBoundaries();
    }
}

// Initialize when page loads
window.addEventListener('load', () => {
    console.log('Page loaded, initializing Terra renderer...');
    terra = new TerraRenderer();
    console.log('Terra renderer initialized:', terra);
});

// Test if functions are accessible
console.log('Terra.js loaded successfully');