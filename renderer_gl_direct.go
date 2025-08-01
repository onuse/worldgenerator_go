package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// Direct voxel rendering shader - samples voxels correctly
const directVertexShader = `
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

const directFragmentShader = `
#version 410 core

in vec2 fragCoord;
out vec4 outColor;

// Uniforms
uniform mat4 invViewProj;
uniform vec3 cameraPos;
uniform float planetRadius;
uniform int shellCount;

// Voxel data textures - but we'll interpret them differently
uniform sampler2DArray materialTexture;
uniform sampler1D shellInfoTexture;

// Material colors
vec3 getMaterialColor(int matType) {
    switch(matType) {
        case 0: return vec3(0.7, 0.8, 1.0);      // Air
        case 1: return vec3(0.0, 0.4, 0.8);      // Water - darker blue
        case 2: return vec3(0.3, 0.3, 0.35);     // Basalt
        case 3: return vec3(0.2, 0.6, 0.2);      // Granite - land green
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

void main() {
    // Generate ray
    vec4 nearPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, -1.0, 1.0);
    vec4 farPoint = invViewProj * vec4(fragCoord * 2.0 - 1.0, 1.0, 1.0);
    
    vec3 ro = cameraPos;
    vec3 rd = normalize(farPoint.xyz / farPoint.w - nearPoint.xyz / nearPoint.w);
    
    // Ray-sphere intersection
    float t;
    if (!raySphereIntersect(ro, rd, planetRadius, t)) {
        outColor = vec4(0.05, 0.05, 0.1, 1.0); // Space
        return;
    }
    
    vec3 hitPos = ro + rd * t;
    vec3 normal = normalize(hitPos);
    
    // Get surface shell (second to last)
    int surfaceShell = shellCount - 2;
    
    // Convert hit position to spherical coordinates
    float theta = acos(clamp(hitPos.z / planetRadius, -1.0, 1.0));
    float phi = atan(hitPos.y, hitPos.x);
    
    // Convert to texture coordinates
    float u = (phi + 3.14159265) / (2.0 * 3.14159265);
    float v = theta / 3.14159265;
    
    // Sample the texture directly
    float matType = texture(materialTexture, vec3(u, v, float(surfaceShell))).r;
    int material = int(matType + 0.5);
    
    vec3 color = getMaterialColor(material);
    
    // Simple lighting
    vec3 lightDir = normalize(vec3(1.0, 1.0, 0.5));
    float NdotL = max(dot(normal, lightDir), 0.0);
    color = color * (0.5 + 0.7 * NdotL);
    
    // Atmosphere
    float fresnel = 1.0 - max(dot(normal, -rd), 0.0);
    color += vec3(0.05, 0.1, 0.2) * pow(fresnel, 3.0);
    
    outColor = vec4(color, 1.0);
}
`

// CompileDirectShaders compiles the direct voxel rendering shaders
func CompileDirectShaders() (uint32, error) {
	// Compile vertex shader
	vertShader, err := compileShader(directVertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	defer gl.DeleteShader(vertShader)
	
	// Compile fragment shader
	fragShader, err := compileShader(directFragmentShader, gl.FRAGMENT_SHADER)
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
	
	fmt.Println("✅ Direct voxel rendering shaders compiled successfully")
	
	return program, nil
}