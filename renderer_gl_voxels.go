package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// voxelRayMarchVertexShader is the vertex shader for ray marching
const voxelRayMarchVertexShader = `
#version 410 core

// Fullscreen quad vertices
const vec2 positions[4] = vec2[](
    vec2(-1.0, -1.0),
    vec2( 1.0, -1.0),
    vec2(-1.0,  1.0),
    vec2( 1.0,  1.0)
);

out vec2 fragCoord;

void main() {
    vec2 pos = positions[gl_VertexID];
    fragCoord = pos * 0.5 + 0.5;
    gl_Position = vec4(pos, 0.0, 1.0);
}
`

// voxelRayMarchFragmentShader is the fragment shader for voxel ray marching
const voxelRayMarchFragmentShader = `
#version 410 core

in vec2 fragCoord;
out vec4 outColor;

// Uniforms
uniform mat4 invViewProj;
uniform vec3 cameraPos;
uniform float planetRadius;
uniform int renderMode;
uniform int crossSection;
uniform int crossSectionAxis;
uniform float crossSectionPos;

// Voxel data passed as uniforms for now (SSBOs are tricky on macOS)
uniform int shellCount;
uniform float time;

// Voxel data textures
uniform sampler2DArray materialTexture;
uniform sampler2DArray temperatureTexture;
uniform sampler2DArray velocityTexture;
uniform sampler1D shellInfoTexture;

// Material colors
vec3 getMaterialColor(int matType) {
    if (matType == 0) return vec3(0.7, 0.8, 1.0);      // Air - light blue
    else if (matType == 1) return vec3(0.0, 0.5, 1.0); // Water - ocean blue
    else if (matType == 2) return vec3(0.3, 0.3, 0.35); // Basalt - dark grey
    else if (matType == 3) return vec3(0.2, 0.7, 0.2);  // Granite - green land
    else if (matType == 4) return vec3(0.5, 0.4, 0.3);  // Peridotite - brown
    else if (matType == 5) return vec3(1.0, 0.2, 0.0);  // Magma - red/orange
    else if (matType == 6) return vec3(0.9, 0.8, 0.6);  // Sediment - tan
    else if (matType == 7) return vec3(0.95, 0.95, 1.0); // Ice - white
    else if (matType == 8) return vec3(0.8, 0.7, 0.5);  // Sand - sandy
    else return vec3(1.0, 0.0, 1.0); // Unknown - magenta
}

// Simple ray-sphere intersection
bool raySphereIntersect(vec3 ro, vec3 rd, float radius, out float t0, out float t1) {
    vec3 oc = ro;
    float b = dot(oc, rd);
    float c = dot(oc, oc) - radius * radius;
    float discriminant = b * b - c;
    
    if (discriminant < 0.0) return false;
    
    float sqrtD = sqrt(discriminant);
    t0 = -b - sqrtD;
    t1 = -b + sqrtD;
    
    return true;
}

// For now, simple surface rendering based on position
vec3 renderPlanetWithVoxels(vec3 ro, vec3 rd) {
    float t0, t1;
    if (!raySphereIntersect(ro, rd, planetRadius, t0, t1)) {
        return vec3(0.05, 0.05, 0.1); // Space background
    }
    
    float t = t0 > 0.0 ? t0 : t1;
    if (t < 0.0) {
        return vec3(0.05, 0.05, 0.1);
    }
    
    vec3 pos = ro + rd * t;
    vec3 normal = normalize(pos);
    
    // Convert to spherical coordinates
    float r = length(pos);
    float theta = acos(pos.z / r); // 0 to PI
    float phi = atan(pos.y, pos.x); // -PI to PI
    
    // Convert to latitude/longitude
    float lat = 90.0 - degrees(theta); // -90 to 90
    float lon = degrees(phi); // -180 to 180
    
    // Sample voxel data from textures
    vec3 baseColor = vec3(0.0, 0.5, 1.0); // Default ocean blue
    
    // Find which shell this surface belongs to
    int surfaceShell = shellCount - 2; // Second to last is surface
    if (surfaceShell >= 0 && surfaceShell < shellCount) {
        // Convert lat/lon to texture coordinates
        float u = (lon + 180.0) / 360.0;
        float v = (lat + 90.0) / 180.0;
        
        // Sample material type from texture
        float matType = texture(materialTexture, vec3(u, v, float(surfaceShell))).r;
        int material = int(matType + 0.5);
        
        // Get color based on material
        if (material > 0) {
            baseColor = getMaterialColor(material);
        }
        
        // If still using test pattern as fallback
        if (material == 0) {
            // Simple continent pattern for testing
            float continent = 0.0;
            
            // Europe/Africa
            if (lat > -35.0 && lat < 70.0 && lon > -20.0 && lon < 50.0) {
                continent = 1.0;
            }
            
            // Americas
            if (lon > -170.0 && lon < -30.0) {
                continent = 0.7 * sin((lat + 20.0) * 0.02);
            }
            
            // Asia
            if (lat > 0.0 && lat < 80.0 && lon > 40.0 && lon < 180.0) {
                continent = max(continent, 0.8);
            }
            
            if (continent > 0.3) {
                baseColor = vec3(0.2, 0.7, 0.2); // Land green
            }
        }
    }
    
    // Simple lighting
    vec3 lightDir = normalize(vec3(0.5, 1.0, 0.3));
    float NdotL = max(dot(normal, lightDir), 0.0);
    vec3 ambient = vec3(0.3);
    vec3 diffuse = vec3(0.7) * NdotL;
    
    return baseColor * (ambient + diffuse);
}

void main() {
    // Generate ray from screen coordinates
    vec4 nearPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, -1.0, 1.0);
    vec4 farPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, 1.0, 1.0);
    
    vec3 ro = cameraPos;
    vec3 rd = normalize(farPoint.xyz / farPoint.w - nearPoint.xyz / nearPoint.w);
    
    // Render with voxel data
    vec3 color = renderPlanetWithVoxels(ro, rd);
    outColor = vec4(color, 1.0);
}
`

// CompileVoxelShaders compiles the voxel ray marching shaders
func CompileVoxelShaders() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(voxelRayMarchVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(voxelRayMarchFragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(fragShader)
	
	// Link program
	program := gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)
	
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(program, logLength, nil, &log[0])
		return 0, fmt.Errorf("program link error: %s", log)
	}
	
	return program, nil
}