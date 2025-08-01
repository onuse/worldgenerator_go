// Texture-based planet renderer with dynamic voxel visualization
class TexturePlanetRenderer {
    constructor() {
        this.scene = new THREE.Scene();
        this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
        this.renderer = new THREE.WebGLRenderer({ antialias: true });
        this.renderer.setSize(window.innerWidth, window.innerHeight);
        document.getElementById('canvas-container').appendChild(this.renderer.domElement);
        
        // Textures for voxel data
        this.materialTexture = null;
        this.heightTexture = null;
        this.ageTexture = null;
        this.velocityTexture = null;
        
        // Create sphere with custom shader
        this.createPlanetSphere();
        
        // Setup camera and controls
        this.camera.position.z = 3;
        this.controls = new THREE.OrbitControls(this.camera, this.renderer.domElement);
        
        // Visualization mode
        this.visualizationMode = 'material';
        
        // Animation
        this.animate();
    }
    
    createPlanetSphere() {
        // Very high-resolution sphere geometry for detailed continents
        const geometry = new THREE.SphereGeometry(1, 512, 256);
        
        // Custom shader material
        const material = new THREE.ShaderMaterial({
            uniforms: {
                materialTexture: { value: null },
                heightTexture: { value: null },
                ageTexture: { value: null },
                velocityTexture: { value: null },
                time: { value: 0 },
                heightScale: { value: 0.1 }
            },
            vertexShader: `
                uniform sampler2D heightTexture;
                uniform sampler2D velocityTexture;
                uniform float time;
                uniform float heightScale;
                
                varying vec2 vUv;
                varying vec3 vNormal;
                varying float vHeight;
                
                void main() {
                    vUv = uv;
                    vNormal = normal;
                    
                    // Sample height from texture
                    vec4 heightData = texture2D(heightTexture, uv);
                    vHeight = heightData.r;
                    
                    // Sample velocity for advection
                    vec4 velocityData = texture2D(velocityTexture, uv);
                    vec2 velocity = (velocityData.rg - 0.5) * 2.0; // Denormalize
                    
                    // Apply velocity-based UV distortion to simulate movement
                    vec2 distortedUv = uv + velocity * time * 0.0001;
                    
                    // Re-sample height with distorted UV
                    vec4 advectedHeight = texture2D(heightTexture, distortedUv);
                    float finalHeight = advectedHeight.r * heightScale;
                    
                    // Displace vertex along normal
                    vec3 displaced = position + normal * finalHeight;
                    
                    gl_Position = projectionMatrix * modelViewMatrix * vec4(displaced, 1.0);
                }
            `,
            fragmentShader: `
                uniform sampler2D materialTexture;
                uniform sampler2D ageTexture;
                
                varying vec2 vUv;
                varying vec3 vNormal;
                varying float vHeight;
                
                vec3 getMaterialColor(float matType) {
                    if (matType < 0.5) return vec3(0.7, 0.8, 1.0); // MatAir (0) - light blue
                    else if (matType < 1.5) return vec3(0.0, 0.5, 1.0); // MatWater (1) - ocean blue
                    else if (matType < 2.5) return vec3(0.3, 0.3, 0.35); // MatBasalt (2) - dark ocean floor
                    else if (matType < 3.5) return vec3(0.2, 0.7, 0.2); // MatGranite (3) - green land
                    else if (matType < 4.5) return vec3(0.5, 0.4, 0.3); // MatPeridotite (4) - mantle
                    else if (matType < 5.5) return vec3(1.0, 0.2, 0.0); // MatMagma (5) - lava
                    else if (matType < 6.5) return vec3(0.9, 0.8, 0.6); // MatSediment (6) - sand
                    else if (matType < 7.5) return vec3(0.95, 0.95, 1.0); // MatIce (7) - ice caps
                    else if (matType < 8.5) return vec3(0.8, 0.7, 0.5); // MatSand (8) - sand
                    else return vec3(1.0, 0.0, 1.0); // Unknown (magenta for debug)
                }
                
                void main() {
                    // Sample material type
                    vec4 matData = texture2D(materialTexture, vUv);
                    float matType = matData.r * 255.0;
                    
                    // Sample age for shading
                    vec4 ageData = texture2D(ageTexture, vUv);
                    float age = ageData.r;
                    
                    // Base color from material
                    vec3 baseColor = getMaterialColor(matType);
                    
                    // Subtle age variation (much less darkening)
                    vec3 color = baseColor * (0.9 + age * 0.1); // Slight brightening with age
                    
                    // Better lighting with ambient and diffuse
                    vec3 lightDir = normalize(vec3(0.5, 1.0, 0.3));
                    float NdotL = max(dot(vNormal, lightDir), 0.0);
                    float ambient = 0.5;
                    float diffuse = 0.5;
                    float light = ambient + diffuse * NdotL;
                    
                    gl_FragColor = vec4(color * light, 1.0);
                }
            `
        });
        
        this.planetMesh = new THREE.Mesh(geometry, material);
        this.scene.add(this.planetMesh);
    }
    
    updateTextures(data) {
        // Convert base64 textures to THREE textures
        const loader = new THREE.TextureLoader();
        
        // Material texture - use linear filtering for smoother continents
        if (data.materialData) {
            loader.load('data:image/png;base64,' + data.materialData, (texture) => {
                texture.minFilter = THREE.LinearFilter;
                texture.magFilter = THREE.LinearFilter;
                texture.generateMipmaps = true;
                this.materialTexture = texture;
                this.planetMesh.material.uniforms.materialTexture.value = texture;
            });
        }
        
        // Height texture
        if (data.heightData) {
            loader.load('data:image/png;base64,' + data.heightData, (texture) => {
                texture.minFilter = THREE.LinearMipMapLinearFilter;
                texture.magFilter = THREE.LinearFilter;
                texture.generateMipmaps = true;
                this.heightTexture = texture;
                this.planetMesh.material.uniforms.heightTexture.value = texture;
            });
        }
        
        // Age texture
        if (data.ageData) {
            loader.load('data:image/png;base64,' + data.ageData, (texture) => {
                texture.minFilter = THREE.LinearMipMapLinearFilter;
                texture.magFilter = THREE.LinearFilter;
                texture.generateMipmaps = true;
                this.ageTexture = texture;
                this.planetMesh.material.uniforms.ageTexture.value = texture;
            });
        }
        
        // Velocity texture
        if (data.velocityData) {
            loader.load('data:image/png;base64,' + data.velocityData, (texture) => {
                texture.minFilter = THREE.LinearFilter;
                texture.magFilter = THREE.LinearFilter;
                this.velocityTexture = texture;
                this.planetMesh.material.uniforms.velocityTexture.value = texture;
            });
        }
        
        // Update time
        this.planetMesh.material.uniforms.time.value = data.time;
    }
    
    animate() {
        requestAnimationFrame(() => this.animate());
        
        this.controls.update();
        this.renderer.render(this.scene, this.camera);
    }
    
    handleResize() {
        this.camera.aspect = window.innerWidth / window.innerHeight;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(window.innerWidth, window.innerHeight);
    }
    
    setVisualizationMode(mode) {
        this.visualizationMode = mode;
        // Update shader uniform to change visualization
        if (this.planetMesh && this.planetMesh.material.uniforms.visualizationMode) {
            this.planetMesh.material.uniforms.visualizationMode.value = 
                mode === 'age' ? 1 : (mode === 'velocity' ? 2 : 0);
        }
    }
}

// WebSocket connection for texture updates
class TextureVoxelConnection {
    constructor(renderer) {
        this.renderer = renderer;
        this.connect();
    }
    
    connect() {
        const ws = new WebSocket(`ws://${window.location.host}/ws_texture`);
        
        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            console.log('Received texture data:', data.type);
            
            if (data.type === 'texture_update') {
                this.renderer.updateTextures(data);
            } else if (data.type === 'mesh_update') {
                // Ignore old mesh updates
                console.log('Ignoring mesh update in texture mode');
            }
        };
        
        ws.onopen = () => {
            console.log('Connected to voxel server');
            // Request texture mode
            ws.send(JSON.stringify({ mode: 'texture' }));
        };
        
        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
        
        ws.onclose = () => {
            console.log('Disconnected from server');
            setTimeout(() => this.connect(), 1000);
        };
        
        this.ws = ws;
    }
}