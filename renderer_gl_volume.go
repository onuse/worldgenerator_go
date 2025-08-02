package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// Volume ray marching vertex shader (same as before)
const volumeRayMarchVertexShader = `
#version 410 core

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

// True volume ray marching fragment shader
const volumeRayMarchFragmentShader = `
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
uniform int shellCount;
uniform float time;

// Volume rendering parameters
uniform float opacityScale;     // Controls overall opacity
uniform float stepSize;          // Ray marching step size multiplier
uniform int maxStepsVolume;      // Maximum steps for volume rendering
uniform float densityThreshold;  // Minimum density to render

// Voxel data textures
uniform sampler2DArray materialTexture;
uniform sampler2DArray temperatureTexture;
uniform sampler2DArray velocityTexture;
uniform sampler1D shellInfoTexture;

// Constants
const float EPSILON = 0.001;
const int DEFAULT_MAX_STEPS = 400;
const float DEFAULT_STEP_SIZE = 0.005; // 0.5% of radius per step

// Material properties with transparency for volume rendering
struct MaterialProps {
    vec3 color;
    float opacity;
    float emissive;
};

MaterialProps getVolumeMaterialProps(int matType) {
    MaterialProps props;
    props.emissive = 0.0;
    
    switch(matType) {
        case 0: // Air - very transparent
            props.color = vec3(0.7, 0.8, 1.0);
            props.opacity = 0.01;
            break;
        case 1: // Water - semi-transparent
            props.color = vec3(0.0, 0.5, 1.0);
            props.opacity = 0.3;
            break;
        case 2: // Basalt - semi-opaque
            props.color = vec3(0.3, 0.3, 0.35);
            props.opacity = 0.5;
            break;
        case 3: // Granite - semi-opaque
            props.color = vec3(0.2, 0.7, 0.2);
            props.opacity = 0.5;
            break;
        case 4: // Peridotite - translucent
            props.color = vec3(0.5, 0.4, 0.3);
            props.opacity = 0.2;
            break;
        case 5: // Magma - glowing and semi-transparent
            props.color = vec3(1.0, 0.3, 0.0);
            props.opacity = 0.6;
            props.emissive = 0.8;
            break;
        case 6: // Sediment - opaque
            props.color = vec3(0.9, 0.8, 0.6);
            props.opacity = 0.7;
            break;
        case 7: // Ice - translucent
            props.color = vec3(0.95, 0.95, 1.0);
            props.opacity = 0.3;
            break;
        case 8: // Sand - opaque
            props.color = vec3(0.8, 0.7, 0.5);
            props.opacity = 0.6;
            break;
        default:
            props.color = vec3(1.0, 0.0, 1.0);
            props.opacity = 0.5;
    }
    
    // Scale opacity for volume rendering
    props.opacity *= opacityScale;
    
    return props;
}

// Find which shell contains a given radius
int findShell(float r) {
    for (int i = 0; i < shellCount; i++) {
        vec4 shellInfo = texelFetch(shellInfoTexture, i, 0);
        float innerR = shellInfo.x;
        float outerR = shellInfo.y;
        
        if (r >= innerR && r <= outerR) {
            return i;
        }
    }
    return -1;
}

// Sample voxel data at a 3D position
vec4 sampleVoxelData(vec3 pos) {
    float r = length(pos);
    
    // Skip if outside planet
    if (r > planetRadius * 1.01) return vec4(0.0);
    
    // Find shell
    int shell = findShell(r);
    if (shell < 0) return vec4(0.0);
    
    // Convert to spherical coordinates
    vec3 normalized = normalize(pos);
    float lat = asin(clamp(normalized.z, -1.0, 1.0));
    float lon = atan(normalized.y, normalized.x);
    
    // Convert to texture coordinates
    float u = (lon + 3.14159265) / (2.0 * 3.14159265);
    float v = (lat + 1.57079633) / 3.14159265;
    
    // Sample with proper filtering
    vec3 texCoord = vec3(u, v, float(shell));
    float matType = texture(materialTexture, texCoord).r;
    float temp = texture(temperatureTexture, texCoord).r;
    vec2 vel = texture(velocityTexture, texCoord).rg;
    
    return vec4(matType, temp, vel.x, vel.y);
}

// Ray-sphere intersection
bool raySphereIntersect(vec3 ro, vec3 rd, float radius, out float t0, out float t1) {
    vec3 oc = ro;
    float a = dot(rd, rd);
    float b = 2.0 * dot(oc, rd);
    float c = dot(oc, oc) - radius * radius;
    float discriminant = b * b - 4.0 * a * c;
    
    if (discriminant < 0.0) return false;
    
    float sqrtD = sqrt(discriminant);
    t0 = (-b - sqrtD) / (2.0 * a);
    t1 = (-b + sqrtD) / (2.0 * a);
    
    return true;
}

// Apply cross-section cutting
bool applyCrossSection(vec3 pos) {
    if (crossSection == 0) return true;
    
    switch(crossSectionAxis) {
        case 0: // X axis
            return pos.x >= crossSectionPos;
        case 1: // Y axis  
            return pos.y >= crossSectionPos;
        case 2: // Z axis
            return pos.z >= crossSectionPos;
    }
    return true;
}

// Get color based on render mode
vec3 getVoxelColor(vec4 voxelData, int matType) {
    vec3 color;
    
    if (renderMode == 1) { // Temperature
        float temp = voxelData.y;
        // More detailed temperature gradient
        if (temp < 1000.0) {
            // Cold: blue to cyan
            float t = temp / 1000.0;
            color = mix(vec3(0.0, 0.0, 0.5), vec3(0.0, 0.5, 1.0), t);
        } else if (temp < 2000.0) {
            // Warm: cyan to yellow
            float t = (temp - 1000.0) / 1000.0;
            color = mix(vec3(0.0, 0.5, 1.0), vec3(1.0, 1.0, 0.0), t);
        } else if (temp < 3000.0) {
            // Hot: yellow to orange
            float t = (temp - 2000.0) / 1000.0;
            color = mix(vec3(1.0, 1.0, 0.0), vec3(1.0, 0.5, 0.0), t);
        } else {
            // Very hot: orange to white
            float t = clamp((temp - 3000.0) / 1000.0, 0.0, 1.0);
            color = mix(vec3(1.0, 0.5, 0.0), vec3(1.0, 1.0, 1.0), t);
        }
    } else if (renderMode == 2) { // Velocity
        float vel = length(vec3(0.0, voxelData.z, voxelData.w));
        color = vec3(vel * 10.0, vel * 5.0, vel * 2.0);
    } else if (renderMode == 3) { // Age
        float age = 0.0; // Age not stored in current implementation
        color = mix(vec3(1.0, 0.0, 0.0), vec3(0.0, 0.0, 1.0), age);
    } else { // Material
        MaterialProps props = getVolumeMaterialProps(matType);
        color = props.color;
    }
    
    return color;
}

// True volume ray marching
vec4 rayMarchVolume(vec3 ro, vec3 rd) {
    // Find entry and exit points
    float t0, t1;
    float atmosphereRadius = planetRadius * 1.01;
    
    if (!raySphereIntersect(ro, rd, atmosphereRadius, t0, t1)) {
        return vec4(0.05, 0.05, 0.1, 1.0); // Space background
    }
    
    // Clamp to positive values (in front of camera)
    t0 = max(t0, 0.0);
    t1 = max(t1, 0.0);
    
    if (t1 <= t0) {
        return vec4(0.05, 0.05, 0.1, 1.0);
    }
    
    // Calculate step size based on planet size and quality settings
    float stepSizeActual = planetRadius * DEFAULT_STEP_SIZE * stepSize;
    int maxSteps = maxStepsVolume > 0 ? maxStepsVolume : DEFAULT_MAX_STEPS;
    
    // Initialize accumulation
    vec4 accumColor = vec4(0.0);
    float accumAlpha = 0.0;
    
    // Start slightly inside to avoid edge artifacts
    float t = t0 + stepSizeActual * 0.5;
    
    // Ray march through the volume
    for (int i = 0; i < maxSteps && t < t1; i++) {
        vec3 pos = ro + rd * t;
        
        // Apply cross-section
        if (!applyCrossSection(pos)) {
            t += stepSizeActual;
            continue;
        }
        
        // Sample voxel data
        vec4 voxelData = sampleVoxelData(pos);
        int matType = int(voxelData.x + 0.5);
        
        // Skip air or very low density materials based on threshold
        if (matType == 0 && densityThreshold > 0.01) {
            t += stepSizeActual;
            continue;
        }
        
        // Get material properties
        MaterialProps props = getVolumeMaterialProps(matType);
        
        // Get color based on render mode
        vec3 color = getVoxelColor(voxelData, matType);
        
        // Add emissive contribution
        if (props.emissive > 0.0) {
            color += props.color * props.emissive;
        }
        
        // Calculate opacity for this step
        float stepOpacity = props.opacity * stepSizeActual / planetRadius;
        stepOpacity = clamp(stepOpacity, 0.0, 1.0);
        
        // Front-to-back compositing
        float weight = stepOpacity * (1.0 - accumAlpha);
        accumColor.rgb += color * weight;
        accumAlpha += weight;
        
        // Early termination
        if (accumAlpha > 0.99) {
            break;
        }
        
        t += stepSizeActual;
    }
    
    // Final color
    if (accumAlpha < 0.01) {
        return vec4(0.05, 0.05, 0.1, 1.0); // Space background
    }
    
    // Blend with background
    vec3 bgColor = vec3(0.05, 0.05, 0.1);
    vec3 finalColor = accumColor.rgb + bgColor * (1.0 - accumAlpha);
    
    return vec4(finalColor, 1.0);
}

void main() {
    // Ray generation
    vec4 ndcPos = vec4(fragCoord * 2.0 - 1.0, 0.0, 1.0);
    vec4 worldPos = invViewProj * ndcPos;
    vec3 rayDir = normalize(worldPos.xyz / worldPos.w - cameraPos);
    
    // Volume ray march
    outColor = rayMarchVolume(cameraPos, rayDir);
}
`

// CompileVolumeRayMarchShaders creates the volume ray marching shader program
func CompileVolumeRayMarchShaders() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(volumeRayMarchVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(volumeRayMarchFragmentShader, gl.FRAGMENT_SHADER)
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
	
	fmt.Println("✅ Volume ray marching shaders compiled successfully")
	
	return program, nil
}
