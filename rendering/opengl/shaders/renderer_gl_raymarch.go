package shaders

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// voxelRayMarchVertexShader remains the same - fullscreen quad
const voxelRayMarchVertexShaderV2 = `
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

// voxelRayMarchFragmentShaderV2 implements proper volume ray marching
const voxelRayMarchFragmentShaderV2 = `
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

// Voxel data textures
uniform sampler2DArray materialTexture;
uniform sampler2DArray temperatureTexture;
uniform sampler2DArray velocityTexture;
uniform sampler1D shellInfoTexture;

// Constants
const float EPSILON = 0.001;
const int MAX_STEPS = 200;
const float STEP_SCALE = 0.01; // Balance between quality and performance

// Material properties
struct MaterialProps {
    vec3 color;
    float opacity;
    float emissive;
};

MaterialProps getMaterialProps(int matType) {
    MaterialProps props;
    props.emissive = 0.0;
    
    switch(matType) {
        case 0: // Air
            props.color = vec3(0.7, 0.8, 1.0);
            props.opacity = 0.001; // Extremely transparent air
            break;
        case 1: // Water
            props.color = vec3(0.0, 0.5, 1.0);
            props.opacity = 1.0; // Fully opaque ocean
            break;
        case 2: // Basalt
            props.color = vec3(0.3, 0.3, 0.35);
            props.opacity = 1.0;
            break;
        case 3: // Granite
            props.color = vec3(0.2, 0.7, 0.2);
            props.opacity = 1.0;
            break;
        case 4: // Peridotite
            props.color = vec3(0.5, 0.4, 0.3);
            props.opacity = 0.8;
            break;
        case 5: // Magma
            props.color = vec3(1.0, 0.3, 0.0);
            props.opacity = 0.9;
            props.emissive = 0.5;
            break;
        case 6: // Sediment
            props.color = vec3(0.9, 0.8, 0.6);
            props.opacity = 1.0;
            break;
        case 7: // Ice
            props.color = vec3(0.95, 0.95, 1.0);
            props.opacity = 0.7;
            break;
        case 8: // Sand
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
        vec4 shellInfo = texelFetch(shellInfoTexture, i, 0);
        float innerR = shellInfo.x;
        float outerR = shellInfo.y;
        
        if (r >= innerR && r <= outerR) {
            return i;
        }
    }
    return -1;
}

// Sample voxel data at a 3D position with smoothing
vec4 sampleVoxelData(vec3 pos) {
    float r = length(pos);
    
    // Find shell
    int shell = findShell(r);
    if (shell < 0) return vec4(0.0);
    
    // Convert to spherical coordinates
    vec3 normalized = normalize(pos);
    float lat = asin(clamp(normalized.z, -1.0, 1.0)); // -PI/2 to PI/2
    float lon = atan(normalized.y, normalized.x); // -PI to PI
    
    // Convert to texture coordinates matching the texture generation
    float u = (lon + 3.14159265) / (2.0 * 3.14159265); // 0 to 1
    float v = (lat + 1.57079633) / 3.14159265; // 0 to 1, where v=0 is south pole
    
    // Sample with proper filtering
    vec3 texCoord = vec3(u, v, float(shell));
    float matType = texture(materialTexture, texCoord).r; // Use nearest filtering for materials
    float temp = texture(temperatureTexture, texCoord).r;
    vec2 vel = texture(velocityTexture, texCoord).rg;
    
    return vec4(matType, temp, vel.x, vel.y);
}

// Ray-sphere intersection
bool raySphereIntersect(vec3 ro, vec3 rd, float radius, out float t0, out float t1) {
    vec3 oc = ro; // ray origin relative to sphere center (at origin)
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

// Volume ray marching with proper opacity accumulation
vec4 rayMarchVolume(vec3 ro, vec3 rd) {
    // Find entry and exit points
    float t0, t1;
    float atmosphereRadius = planetRadius * 1.01; // Include thin atmosphere
    
    if (!raySphereIntersect(ro, rd, atmosphereRadius, t0, t1)) {
        return vec4(0.05, 0.05, 0.1, 1.0); // Space background
    }
    
    // Use surface rendering for better performance and appearance
    float t0_surface, t1_surface;
    if (raySphereIntersect(ro, rd, planetRadius, t0_surface, t1_surface)) {
        if (t0_surface > 0.0) {
            // Hit the planet surface
            vec3 hitPos = ro + rd * t0_surface;
            vec3 normal = normalize(hitPos);
            
            // Sample slightly inside the surface to avoid shell boundary issues
            vec3 samplePos = hitPos * 0.999; // Move 0.1% inward
            vec4 voxelData = sampleVoxelData(samplePos);
            
            // Add procedural noise to break up the grid pattern
            float noiseScale = 50.0;
            vec3 noisePos = hitPos / planetRadius * noiseScale;
            float noise = sin(noisePos.x * 12.96) * sin(noisePos.y * 17.87) * sin(noisePos.z * 15.37);
            noise = noise * 0.5 + 0.5; // 0 to 1
            
            // Use noise to blend between water and land more organically
            float coastNoise = noise * 0.3 - 0.15; // -0.15 to 0.15
            
            float matTypeFloat = voxelData.x;
            int matType = int(matTypeFloat + 0.5);
            
            // Get base material color
            MaterialProps props = getMaterialProps(matType);
            vec3 color = props.color;
            
            // Keep colors pure without noise variation to avoid dithering
            // The noise multiplication was contributing to the dithering pattern
            
            // Bright lighting
            vec3 lightDir = normalize(vec3(1.0, 1.0, 0.5));
            float NdotL = max(dot(normal, lightDir), 0.0);
            color = color * (0.7 + 0.5 * NdotL);
            
            // Add subtle atmosphere effect
            float fresnel = 1.0 - max(dot(normal, -rd), 0.0);
            color += vec3(0.05, 0.1, 0.2) * pow(fresnel, 3.0);
            
            return vec4(color, 1.0);
        }
    }
    
    // Start from entry point
    float tStart = max(t0, 0.0);
    float tEnd = t1;
    
    // Adaptive step size based on distance from camera
    float baseStep = planetRadius * STEP_SCALE;
    
    // Accumulate color and opacity
    vec3 accumColor = vec3(0.0);
    float accumAlpha = 0.0;
    
    // Ray march through the volume
    float t = tStart;
    int steps = 0;
    
    while (t < tEnd && accumAlpha < 0.99 && steps < MAX_STEPS) {
        vec3 pos = ro + rd * t;
        
        // Cross-section culling
        if (crossSection > 0) {
            float coord = (crossSectionAxis == 0) ? pos.x :
                         (crossSectionAxis == 1) ? pos.y : pos.z;
            if (coord < crossSectionPos) {
                t += baseStep;
                steps++;
                continue;
            }
        }
        
        // Sample voxel data
        vec4 voxelData = sampleVoxelData(pos);
        int matType = int(voxelData.x + 0.5);
        float temperature = voxelData.y;
        
        // Get material properties with smooth blending
        MaterialProps props = getMaterialProps(matType);
        
        // FIXED: Removed fractional material blending to eliminate dithering
        // Just use the discrete material type without blending
        
        // Visualization modes
        vec3 color = props.color;
        if (renderMode == 1) { // Temperature
            float normalizedTemp = clamp((temperature - 273.0) / 3000.0, 0.0, 1.0);
            color = mix(vec3(0.0, 0.0, 1.0), vec3(1.0, 0.0, 0.0), normalizedTemp);
            props.opacity = 0.1; // Make temperature semi-transparent
        } else if (renderMode == 2) { // Velocity
            float vel = length(voxelData.zw) * 1e9; // Convert to cm/year
            color = mix(vec3(0.0, 0.0, 0.5), vec3(1.0, 1.0, 0.0), clamp(vel / 10.0, 0.0, 1.0));
            props.opacity = 0.1;
        } else if (renderMode == 4) { // Plates - use material type as a proxy for now
            // Without plate data in textures, show different colors for rock types
            if (matType == 2 || matType == 3) { // Basalt or Granite
                float pseudoPlate = float(matType + int(pos.x * 10.0) + int(pos.z * 10.0)) * 137.5;
                float hue = mod(pseudoPlate, 360.0) / 360.0;
                vec3 hsv = vec3(hue, 0.7, 0.8);
                vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);
                vec3 p = abs(fract(hsv.xxx + K.xyz) * 6.0 - K.www);
                color = hsv.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), hsv.y);
            }
        }
        
        // Enhanced lighting with camera-relative light
        vec3 normal = normalize(pos);
        vec3 lightDir = normalize(cameraPos); // Light from camera direction
        float NdotL = max(dot(normal, lightDir), 0.0);
        
        // Strong ambient light to ensure visibility
        float rimLight = 1.0 - max(dot(normal, -rd), 0.0);
        rimLight = pow(rimLight, 2.0) * 0.3;
        
        vec3 lighting = vec3(0.8) + vec3(0.6) * NdotL + vec3(rimLight);
        
        // Apply lighting and emissive
        color = color * lighting + color * props.emissive;
        
        // Opacity accumulation (front-to-back)
        float stepOpacity = props.opacity * baseStep / planetRadius * 20.0; // Strong opacity for visibility
        stepOpacity = clamp(stepOpacity, 0.0, 1.0);
        
        float alpha = stepOpacity * (1.0 - accumAlpha);
        accumColor += color * alpha;
        accumAlpha += alpha;
        
        // Early termination for opaque surfaces
        if (props.opacity > 0.9 && matType != 0) {
            // Hit solid material, add remaining opacity and stop
            accumColor += color * (1.0 - accumAlpha);
            accumAlpha = 1.0;
            break;
        }
        
        // Adaptive step size
        float distanceFromSurface = abs(length(pos) - planetRadius);
        float adaptiveStep = baseStep * (1.0 + distanceFromSurface / planetRadius);
        
        t += adaptiveStep;
        steps++;
    }
    
    return vec4(accumColor, accumAlpha);
}

void main() {
    // Generate ray from screen coordinates
    vec4 nearPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, -1.0, 1.0);
    vec4 farPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, 1.0, 1.0);
    
    vec3 ro = cameraPos;
    vec3 rd = normalize(farPoint.xyz / farPoint.w - nearPoint.xyz / nearPoint.w);
    
    // Volume ray marching
    vec4 result = rayMarchVolume(ro, rd);
    
    // Composite over background
    vec3 background = vec3(0.05, 0.05, 0.1);
    vec3 finalColor = result.rgb + background * (1.0 - result.a);
    
    // Debug: if we hit nothing, show bright red
    if (result.a < 0.01) {
        finalColor = vec3(1.0, 0.0, 0.0);
    }
    
    outColor = vec4(finalColor, 1.0);
}
`

// CompileVoxelRayMarchShaders compiles the volume ray marching shaders
func CompileVoxelRayMarchShaders() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(voxelRayMarchVertexShaderV2, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(voxelRayMarchFragmentShaderV2, gl.FRAGMENT_SHADER)
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
