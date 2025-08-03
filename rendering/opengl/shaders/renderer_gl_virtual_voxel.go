package shaders

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
)

// Virtual voxel point rendering shaders
const virtualVoxelVertexShader = `#version 430 core

// Virtual voxel structure matching GPU compute shader
struct VirtualVoxel {
    vec3 position;      // Spherical coords: r, theta, phi
    float mass;
    vec3 velocity;      
    float temperature;
    vec3 force;         
    int plateID;
    int material;       
    int bondOffset;     
    int bondCount;      
    float padding;
};

// SSBO containing virtual voxels
layout(std430, binding = 0) buffer VoxelBuffer {
    VirtualVoxel voxels[];
};

// Uniforms
uniform mat4 viewMatrix;
uniform mat4 projMatrix;
uniform float planetRadius;
uniform int renderMode; // 0=material, 1=temperature, 2=velocity, 3=plateID
uniform float pointSize;

// Output to fragment shader
out vec4 fragColor;
out float fragDepth;

// Material colors
vec4 getMaterialColor(int mat) {
    vec4 colors[6] = vec4[6](
        vec4(0.8, 0.9, 1.0, 0.1),  // Air (transparent)
        vec4(0.2, 0.4, 0.8, 0.8),  // Water (blue)
        vec4(0.7, 0.6, 0.5, 1.0),  // Granite (tan)
        vec4(0.3, 0.3, 0.3, 1.0),  // Basalt (dark gray)
        vec4(0.8, 0.4, 0.2, 1.0),  // Mantle (orange)
        vec4(1.0, 0.3, 0.1, 1.0)   // Magma (red)
    );
    return colors[clamp(mat, 0, 5)];
}

// Temperature to color mapping
vec4 temperatureToColor(float temp) {
    // Normalize temperature (0-4000K range)
    float t = clamp((temp - 273.15) / 3726.85, 0.0, 1.0);
    
    // Heat map: blue -> green -> yellow -> red
    vec4 cold = vec4(0.0, 0.0, 1.0, 1.0);
    vec4 cool = vec4(0.0, 1.0, 1.0, 1.0);
    vec4 warm = vec4(1.0, 1.0, 0.0, 1.0);
    vec4 hot = vec4(1.0, 0.0, 0.0, 1.0);
    
    if (t < 0.33) {
        return mix(cold, cool, t * 3.0);
    } else if (t < 0.66) {
        return mix(cool, warm, (t - 0.33) * 3.0);
    } else {
        return mix(warm, hot, (t - 0.66) * 3.0);
    }
}

// Velocity to color mapping
vec4 velocityToColor(vec3 vel) {
    float speed = length(vel) * 100000.0; // Scale for visibility
    return vec4(speed, 0.5 - speed * 0.5, 1.0 - speed, 1.0);
}

// Plate ID to color
vec4 plateToColor(int plateID) {
    // Generate pseudo-random color from plate ID
    float hue = float(plateID * 139) / 360.0;
    float sat = 0.7;
    float val = 0.8;
    
    // HSV to RGB conversion
    float c = val * sat;
    float x = c * (1.0 - abs(mod(hue * 6.0, 2.0) - 1.0));
    float m = val - c;
    
    vec3 rgb;
    if (hue < 1.0/6.0) rgb = vec3(c, x, 0);
    else if (hue < 2.0/6.0) rgb = vec3(x, c, 0);
    else if (hue < 3.0/6.0) rgb = vec3(0, c, x);
    else if (hue < 4.0/6.0) rgb = vec3(0, x, c);
    else if (hue < 5.0/6.0) rgb = vec3(x, 0, c);
    else rgb = vec3(c, 0, x);
    
    return vec4(rgb + m, 1.0);
}

// Convert spherical to Cartesian coordinates
vec3 sphericalToCartesian(vec3 spherical) {
    float r = spherical.x;
    float theta = spherical.y; // latitude
    float phi = spherical.z;   // longitude
    
    float cosTheta = cos(theta);
    float sinTheta = sin(theta);
    float cosPhi = cos(phi);
    float sinPhi = sin(phi);
    
    return vec3(
        r * cosTheta * cosPhi,
        r * sinTheta,
        r * cosTheta * sinPhi
    );
}

void main() {
    // Get voxel for this vertex
    VirtualVoxel voxel = voxels[gl_VertexID];
    
    // Skip air voxels
    if (voxel.material == 0) {
        gl_Position = vec4(0, 0, 0, 0);
        return;
    }
    
    // Convert to world coordinates
    vec3 worldPos = sphericalToCartesian(voxel.position);
    
    // Transform to clip space
    vec4 viewPos = viewMatrix * vec4(worldPos, 1.0);
    gl_Position = projMatrix * viewPos;
    
    // Calculate point size based on distance
    float dist = length(viewPos.xyz);
    gl_PointSize = pointSize * planetRadius / dist;
    
    // Color based on render mode
    switch(renderMode) {
        case 0: // Material
            fragColor = getMaterialColor(voxel.material);
            break;
        case 1: // Temperature
            fragColor = temperatureToColor(voxel.temperature);
            break;
        case 2: // Velocity
            fragColor = velocityToColor(voxel.velocity);
            break;
        case 3: // Plate ID
            fragColor = plateToColor(voxel.plateID);
            break;
        default:
            fragColor = vec4(1.0, 0.0, 1.0, 1.0); // Magenta for unknown
    }
    
    fragDepth = viewPos.z;
}
`

const virtualVoxelFragmentShader = `#version 430 core

in vec4 fragColor;
in float fragDepth;

out vec4 outColor;

void main() {
    // Simple circular point sprite
    vec2 coord = 2.0 * gl_PointCoord - 1.0;
    float dist = dot(coord, coord);
    
    if (dist > 1.0) {
        discard;
    }
    
    // Soft edges
    float alpha = fragColor.a * (1.0 - smoothstep(0.7, 1.0, dist));
    
    // Apply simple shading
    float shade = 1.0 - dist * 0.3;
    outColor = vec4(fragColor.rgb * shade, alpha);
    
    // Manual depth for better sorting
    gl_FragDepth = gl_FragCoord.z;
}
`

// CreateVirtualVoxelProgram creates the shader program for rendering virtual voxels
func CreateVirtualVoxelProgram() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(virtualVoxelVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("virtual voxel vertex shader: %v", err)
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(virtualVoxelFragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("virtual voxel fragment shader: %v", err)
	}
	defer gl.DeleteShader(fragShader)
	
	// Link program
	program, err := linkProgram(vertShader, fragShader)
	if err != nil {
		return 0, fmt.Errorf("virtual voxel program: %v", err)
	}
	
	return program, nil
}