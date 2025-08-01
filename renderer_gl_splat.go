package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// Splat-based rendering shader that treats each voxel as a smooth sphere
const splatVertexShader = `
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

const splatFragmentShader = `
#version 410 core

in vec2 fragCoord;
out vec4 outColor;

// Uniforms
uniform mat4 invViewProj;
uniform vec3 cameraPos;
uniform float planetRadius;
uniform int shellCount;

// Voxel data textures
uniform sampler2DArray materialTexture;
uniform sampler2DArray temperatureTexture;
uniform sampler1D shellInfoTexture;

// Material colors
vec3 getMaterialColor(int matType) {
    switch(matType) {
        case 0: return vec3(0.7, 0.8, 1.0);      // Air
        case 1: return vec3(0.0, 0.5, 1.0);      // Water
        case 2: return vec3(0.3, 0.3, 0.35);     // Basalt
        case 3: return vec3(0.2, 0.7, 0.2);      // Granite
        case 4: return vec3(0.5, 0.4, 0.3);      // Peridotite
        case 5: return vec3(1.0, 0.3, 0.0);      // Magma
        case 6: return vec3(0.9, 0.8, 0.6);      // Sediment
        case 7: return vec3(0.95, 0.95, 1.0);    // Ice
        case 8: return vec3(0.8, 0.7, 0.5);      // Sand
        default: return vec3(1.0, 0.0, 1.0);     // Unknown
    }
}

// Ray-sphere intersection
bool raySphereIntersect(vec3 ro, vec3 rd, float radius, out float t) {
    vec3 oc = ro;
    float a = dot(rd, rd);
    float b = 2.0 * dot(oc, rd);
    float c = dot(oc, oc) - radius * radius;
    float discriminant = b * b - 4.0 * a * c;
    
    if (discriminant < 0.0) return false;
    
    t = (-b - sqrt(discriminant)) / (2.0 * a);
    return t > 0.0;
}

// Smooth density function for voxel splats
float voxelDensity(vec3 pos, vec3 voxelCenter, float voxelSize) {
    float dist = length(pos - voxelCenter);
    return smoothstep(voxelSize, voxelSize * 0.5, dist);
}

// Sample single voxel data
vec4 sampleVoxelData(vec3 pos) {
    float r = length(pos);
    
    // Find appropriate shell
    int targetShell = -1;
    for (int i = 0; i < shellCount; i++) {
        vec4 shellInfo = texelFetch(shellInfoTexture, i, 0);
        if (r >= shellInfo.x && r <= shellInfo.y) {
            targetShell = i;
            break;
        }
    }
    
    if (targetShell < 0) return vec4(0.0);
    
    // Convert to spherical coordinates
    float theta = acos(clamp(pos.z / r, -1.0, 1.0));
    float phi = atan(pos.y, pos.x);
    
    // Convert to texture coordinates
    float u = (phi + 3.14159265) / (2.0 * 3.14159265);
    float v = theta / 3.14159265;
    
    // Sample material
    float matType = texture(materialTexture, vec3(u, v, float(targetShell))).r;
    
    return vec4(matType, 0.0, 0.0, 0.0);
}

// Sample and blend voxels in a region
vec4 sampleVoxelRegion(vec3 pos) {
    float r = length(pos);
    
    // Find appropriate shell
    int targetShell = -1;
    for (int i = 0; i < shellCount; i++) {
        vec4 shellInfo = texelFetch(shellInfoTexture, i, 0);
        if (r >= shellInfo.x && r <= shellInfo.y) {
            targetShell = i;
            break;
        }
    }
    
    if (targetShell < 0) return vec4(0.0);
    
    // Sample multiple nearby voxels and blend
    vec3 accumColor = vec3(0.0);
    float accumWeight = 0.0;
    
    // Convert to spherical coordinates
    float theta = acos(clamp(pos.z / r, -1.0, 1.0));
    float phi = atan(pos.y, pos.x);
    
    // Sample in a small radius with gaussian weights
    const int samples = 3;
    const float sigma = 1.5;
    for (int i = -samples; i <= samples; i++) {
        for (int j = -samples; j <= samples; j++) {
            float sampleTheta = theta + float(i) * 0.005;
            float samplePhi = phi + float(j) * 0.005;
            
            // Convert back to texture coordinates
            float u = (samplePhi + 3.14159265) / (2.0 * 3.14159265);
            float v = sampleTheta / 3.14159265;
            
            // Wrap coordinates
            u = fract(u);
            v = clamp(v, 0.0, 1.0);
            
            // Sample material
            float matType = texture(materialTexture, vec3(u, v, float(targetShell))).r;
            int material = int(matType + 0.5);
            
            if (material > 0) { // Not air
                // Calculate weight based on distance
                float weight = exp(-float(i*i + j*j) * 0.5);
                accumColor += getMaterialColor(material) * weight;
                accumWeight += weight;
            }
        }
    }
    
    if (accumWeight > 0.0) {
        return vec4(accumColor / accumWeight, 1.0);
    }
    
    return vec4(0.0);
}

void main() {
    // Generate ray
    vec4 nearPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, -1.0, 1.0);
    vec4 farPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, 1.0, 1.0);
    
    vec3 ro = cameraPos;
    vec3 rd = normalize(farPoint.xyz / farPoint.w - nearPoint.xyz / nearPoint.w);
    
    // Ray-sphere intersection with planet
    float t;
    if (!raySphereIntersect(ro, rd, planetRadius, t)) {
        outColor = vec4(0.05, 0.05, 0.1, 1.0); // Space
        return;
    }
    
    vec3 hitPos = ro + rd * t;
    vec3 normal = normalize(hitPos);
    
    // Sample voxel directly without blending for crisp boundaries
    vec4 voxelData = sampleVoxelData(hitPos);
    int material = int(voxelData.x + 0.5);
    
    vec4 voxelColor;
    if (material > 0) {
        voxelColor = vec4(getMaterialColor(material), 1.0);
    } else {
        voxelColor = vec4(0.0, 0.5, 1.0, 1.0); // Default ocean
    }
    
    // Lighting
    vec3 lightDir = normalize(vec3(1.0, 1.0, 0.5));
    float NdotL = max(dot(normal, lightDir), 0.0);
    vec3 color = voxelColor.rgb * (0.6 + 0.6 * NdotL);
    
    // Atmosphere effect
    float fresnel = 1.0 - max(dot(normal, -rd), 0.0);
    color += vec3(0.05, 0.1, 0.2) * pow(fresnel, 3.0);
    
    outColor = vec4(color, 1.0);
}
`

// CompileSplatShaders compiles the splat rendering shaders
func CompileSplatShaders() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(splatVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(splatFragmentShader, gl.FRAGMENT_SHADER)
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
	
	fmt.Println("✅ Splat rendering shaders compiled successfully")
	
	return program, nil
}