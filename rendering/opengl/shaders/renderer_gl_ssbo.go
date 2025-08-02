package shaders

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// SSBO-based vertex shader for ray marching
const voxelSSBOVertexShader = `
#version 430 core

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

// SSBO-based fragment shader for voxel ray marching
const voxelSSBOFragmentShader = `
#version 430 core

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

// GPU voxel material structure matching GPUVoxelMaterial
struct VoxelMaterial {
    uint type;
    float density;
    float temperature;
    float pressure;
    float velTheta;
    float velPhi;
    float velR;
    float age;
    int plateID;
    int isBoundary;
    int _padding0;
    int _padding1;
};

// Shell metadata structure matching SphericalShellMetadata
struct ShellMetadata {
    float innerRadius;
    float outerRadius;
    int latBands;
    int voxelOffset;
};

// SSBOs for voxel data
layout(std430, binding = 0) readonly buffer VoxelData {
    VoxelMaterial voxels[];
} voxelData;

layout(std430, binding = 1) readonly buffer ShellData {
    ShellMetadata shells[];
} shellData;

// Constants
const float EPSILON = 0.001;
const int MAX_STEPS = 200;
const float STEP_SCALE = 0.02;

// Material properties
struct MaterialProps {
    vec3 color;
    float opacity;
    float emissive;
};

MaterialProps getMaterialProps(uint matType) {
    MaterialProps props;
    props.emissive = 0.0;
    
    switch(matType) {
        case 0u: // Air
            props.color = vec3(0.7, 0.8, 1.0);
            props.opacity = 0.001;
            break;
        case 1u: // Water
            props.color = vec3(0.0, 0.5, 1.0);
            props.opacity = 1.0;
            break;
        case 2u: // Basalt
            props.color = vec3(0.3, 0.3, 0.35);
            props.opacity = 1.0;
            break;
        case 3u: // Granite  
            props.color = vec3(0.2, 0.7, 0.2);
            props.opacity = 1.0;
            break;
        case 4u: // Peridotite
            props.color = vec3(0.5, 0.4, 0.3);
            props.opacity = 0.8;
            break;
        case 5u: // Magma
            props.color = vec3(1.0, 0.3, 0.0);
            props.opacity = 0.9;
            props.emissive = 0.5;
            break;
        case 6u: // Sediment
            props.color = vec3(0.9, 0.8, 0.6);
            props.opacity = 1.0;
            break;
        case 7u: // Ice
            props.color = vec3(0.95, 0.95, 1.0);
            props.opacity = 0.7;
            break;
        case 8u: // Sand
            props.color = vec3(0.8, 0.7, 0.5);
            props.opacity = 1.0;
            break;
        default:
            props.color = vec3(1.0, 0.0, 1.0);
            props.opacity = 1.0;
    }
    
    return props;
}

// Find which shell contains a given radius
int findShell(float r) {
    for (int i = 0; i < shellCount; i++) {
        if (r >= shellData.shells[i].innerRadius && r <= shellData.shells[i].outerRadius) {
            return i;
        }
    }
    return -1;
}

// Calculate voxel index for a position
int getVoxelIndex(vec3 pos, int shellIdx) {
    ShellMetadata shell = shellData.shells[shellIdx];
    
    // Convert to spherical coordinates
    vec3 normalized = normalize(pos);
    float lat = asin(clamp(normalized.z, -1.0, 1.0)); // -PI/2 to PI/2
    float lon = atan(normalized.y, normalized.x); // -PI to PI
    
    // Convert to latitude band (0 to latBands-1)
    float latDeg = degrees(lat) + 90.0; // 0 to 180
    int latBand = int(latDeg / 180.0 * float(shell.latBands));
    latBand = clamp(latBand, 0, shell.latBands - 1);
    
    // For now, assume uniform longitude distribution within each band
    // In reality, we'd need the actual longitude counts per band
    float lonNorm = (lon + 3.14159265) / (2.0 * 3.14159265); // 0 to 1
    int lonCount = 360; // Approximate - would need actual count
    int lonIdx = int(lonNorm * float(lonCount));
    lonIdx = lonIdx % lonCount;
    
    // Calculate linear index (this is approximate without actual lon counts)
    int voxelIdx = shell.voxelOffset + latBand * lonCount + lonIdx;
    
    return voxelIdx;
}

// Sample voxel data at a 3D position
VoxelMaterial sampleVoxel(vec3 pos) {
    float r = length(pos);
    
    // Find shell
    int shellIdx = findShell(r);
    if (shellIdx < 0) {
        VoxelMaterial empty;
        empty.type = 0u; // Air
        empty.temperature = 0.0;
        empty.density = 0.0;
        return empty;
    }
    
    // Get voxel index
    int idx = getVoxelIndex(pos, shellIdx);
    
    // Bounds check
    if (idx < 0 || idx >= voxelData.voxels.length()) {
        VoxelMaterial empty;
        empty.type = 0u;
        return empty;
    }
    
    return voxelData.voxels[idx];
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

// Surface rendering with SSBO data
vec4 renderSurface(vec3 ro, vec3 rd) {
    float t0, t1;
    
    if (!raySphereIntersect(ro, rd, planetRadius, t0, t1)) {
        return vec4(0.05, 0.05, 0.1, 1.0); // Space background
    }
    
    if (t0 > 0.0) {
        vec3 hitPos = ro + rd * t0;
        
        // Apply cross-section
        if (!applyCrossSection(hitPos)) {
            return vec4(0.05, 0.05, 0.1, 1.0);
        }
        
        vec3 normal = normalize(hitPos);
        
        // Sample voxel data
        VoxelMaterial voxel = sampleVoxel(hitPos * 0.999); // Sample slightly inside
        
        // Get material properties
        MaterialProps matProps = getMaterialProps(voxel.type);
        
        // Basic lighting
        vec3 lightDir = normalize(vec3(1.0, 1.0, 0.5));
        float NdotL = max(dot(normal, lightDir), 0.0);
        float ambient = 0.3;
        
        vec3 color = matProps.color;
        
        // Apply render mode
        if (renderMode == 1) { // Temperature
            float temp = voxel.temperature;
            float t = clamp((temp - 273.0) / 100.0, 0.0, 1.0);
            color = mix(vec3(0.0, 0.0, 1.0), vec3(1.0, 0.0, 0.0), t);
        } else if (renderMode == 2) { // Velocity
            float vel = length(vec3(voxel.velR, voxel.velTheta, voxel.velPhi));
            color = vec3(vel * 10.0, vel * 5.0, 0.0);
        } else if (renderMode == 3) { // Age
            float age = voxel.age / 1e9; // Convert to billions of years
            color = mix(vec3(1.0, 0.0, 0.0), vec3(0.0, 0.0, 1.0), clamp(age / 4.0, 0.0, 1.0));
        } else if (renderMode == 4) { // Plates
            if (voxel.plateID > 0) {
                // Use a hash function to generate unique colors per plate
                float hue = float(voxel.plateID * 1337 % 360) / 360.0;
                vec3 hsv = vec3(hue, 0.8, 0.9);
                // Convert HSV to RGB
                vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);
                vec3 p = abs(fract(hsv.xxx + K.xyz) * 6.0 - K.www);
                color = hsv.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), hsv.y);
                
                // Highlight boundaries
                if (voxel.isBoundary > 0) {
                    color = mix(color, vec3(1.0, 1.0, 0.0), 0.5); // Yellow for boundaries
                }
            } else {
                color = vec3(0.2, 0.2, 0.2); // Dark gray for no plate
            }
        }
        
        color = color * (ambient + (1.0 - ambient) * NdotL);
        color += matProps.emissive * matProps.color;
        
        return vec4(color, 1.0);
    }
    
    return vec4(0.05, 0.05, 0.1, 1.0);
}

void main() {
    // Ray generation
    vec4 ndcPos = vec4(fragCoord * 2.0 - 1.0, 0.0, 1.0);
    vec4 worldPos = invViewProj * ndcPos;
    vec3 rayDir = normalize(worldPos.xyz / worldPos.w - cameraPos);
    
    // Render
    outColor = renderSurface(cameraPos, rayDir);
}
`

// CreateSSBOProgram creates the SSBO-based ray marching shader program
func CreateSSBOProgram() (uint32, error) {
	// Create vertex shader
	vertShader := gl.CreateShader(gl.VERTEX_SHADER)
	source, free := gl.Strs(voxelSSBOVertexShader + "\x00")
	gl.ShaderSource(vertShader, 1, source, nil)
	free()
	gl.CompileShader(vertShader)
	
	// Check vertex shader compilation
	var status int32
	gl.GetShaderiv(vertShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(vertShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(vertShader, logLength, nil, &log[0])
		return 0, fmt.Errorf("vertex shader compilation failed: %v", string(log))
	}
	
	// Create fragment shader
	fragShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	source, free = gl.Strs(voxelSSBOFragmentShader + "\x00")
	gl.ShaderSource(fragShader, 1, source, nil)
	free()
	gl.CompileShader(fragShader)
	
	// Check fragment shader compilation
	gl.GetShaderiv(fragShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(fragShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(fragShader, logLength, nil, &log[0])
		return 0, fmt.Errorf("fragment shader compilation failed: %v", string(log))
	}
	
	// Create program
	program := gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)
	
	// Check program linking
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(program, logLength, nil, &log[0])
		return 0, fmt.Errorf("program linking failed: %v", string(log))
	}
	
	// Clean up shaders
	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)
	
	return program, nil
}
