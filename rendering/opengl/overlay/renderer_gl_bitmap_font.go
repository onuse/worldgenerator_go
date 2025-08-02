package overlay

// Simple bitmap font for overlay text
// Using a monospace font atlas approach

const bitmapFontVertexShader = `
#version 430 core

layout (location = 0) in vec2 position;
layout (location = 1) in vec2 texCoord;

out vec2 fragTexCoord;

uniform mat4 projection;

void main() {
    gl_Position = projection * vec4(position, 0.0, 1.0);
    fragTexCoord = texCoord;
}
`

const bitmapFontFragmentShader = `
#version 430 core

in vec2 fragTexCoord;
out vec4 outColor;

uniform sampler2D fontTexture;
uniform vec4 textColor;

void main() {
    float alpha = texture(fontTexture, fragTexCoord).r;
    outColor = vec4(textColor.rgb, textColor.a * alpha);
}
`

// For now, we'll use a procedural approach to render numbers
const numberOverlayShader = `
#version 430 core

in vec2 fragCoord;
out vec4 outColor;

uniform vec2 screenSize;
uniform float fps;
uniform float zoom;
uniform float distance;

// Simple digit rendering using SDF
float digit(vec2 p, int d) {
    p = abs(p);
    
    if (d == 0) {
        return max(abs(length(p - vec2(0.5, 0.5)) - 0.5), 
                   abs(max(p.x, p.y) - 0.5)) < 0.1 ? 1.0 : 0.0;
    } else if (d == 1) {
        return p.x < 0.1 && p.y < 1.0 ? 1.0 : 0.0;
    } else if (d == 2) {
        float d1 = abs(p.y - 0.8) < 0.1 && p.x < 0.5 ? 1.0 : 0.0;
        float d2 = abs(p.x - 0.5) < 0.1 && p.y > 0.5 && p.y < 0.8 ? 1.0 : 0.0;
        float d3 = abs(p.y - 0.5) < 0.1 && p.x < 0.5 ? 1.0 : 0.0;
        float d4 = abs(p.x) < 0.1 && p.y > 0.2 && p.y < 0.5 ? 1.0 : 0.0;
        float d5 = abs(p.y - 0.2) < 0.1 && p.x < 0.5 ? 1.0 : 0.0;
        return max(max(max(d1, d2), max(d3, d4)), d5);
    }
    // Simplified - just show shape for other digits
    return length(p - vec2(0.5, 0.5)) < 0.4 ? 1.0 : 0.0;
}

// Render a number at position
float renderNumber(vec2 pos, float number, int maxDigits) {
    float result = 0.0;
    int n = int(number);
    
    for (int i = 0; i < maxDigits; i++) {
        int digit_val = n % 10;
        n /= 10;
        
        vec2 digitPos = (pos - vec2(float(maxDigits - i - 1) * 8.0, 0.0)) / 10.0;
        if (digitPos.x >= 0.0 && digitPos.x <= 1.0 && 
            digitPos.y >= 0.0 && digitPos.y <= 1.0) {
            result = max(result, digit(digitPos, digit_val));
        }
        
        if (n == 0) break;
    }
    
    return result;
}

void main() {
    vec2 pixelPos = fragCoord * screenSize;
    
    // Background area
    if (pixelPos.x < 260.0 && pixelPos.y > screenSize.y - 90.0) {
        float bgY = screenSize.y - pixelPos.y; // Flip Y for bottom-left
        
        if (pixelPos.x > 10.0 && pixelPos.x < 260.0 && bgY > 10.0 && bgY < 90.0) {
            // Dark blue background
            outColor = vec4(0.1, 0.1, 0.3, 0.8);
            
            // Text labels (white)
            vec2 textPos = pixelPos - vec2(15.0, screenSize.y - 85.0);
            
            // FPS text and bar
            if (bgY > 15.0 && bgY < 30.0) {
                // Text "FPS:" would go here - for now just show number
                if (pixelPos.x > 15.0 && pixelPos.x < 60.0) {
                    // Simple white text area for label
                    outColor = vec4(1.0, 1.0, 1.0, 1.0);
                }
                // Green bar
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(fps, 150.0) && 
                    bgY > 20.0 && bgY < 30.0) {
                    outColor = vec4(0.0, 1.0, 0.0, 1.0);
                }
            }
            
            // Zoom text and bar
            if (bgY > 35.0 && bgY < 50.0) {
                if (pixelPos.x > 15.0 && pixelPos.x < 60.0) {
                    outColor = vec4(1.0, 1.0, 1.0, 1.0);
                }
                // Blue bar
                float zoomBar = zoom * 100.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(zoomBar, 200.0) &&
                    bgY > 40.0 && bgY < 50.0) {
                    outColor = vec4(0.5, 0.5, 1.0, 1.0);
                }
            }
            
            // Distance text and bar
            if (bgY > 55.0 && bgY < 70.0) {
                if (pixelPos.x > 15.0 && pixelPos.x < 60.0) {
                    outColor = vec4(1.0, 1.0, 1.0, 1.0);
                }
                // Yellow bar
                float distBar = distance / 100000.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(distBar, 200.0) &&
                    bgY > 60.0 && bgY < 70.0) {
                    outColor = vec4(1.0, 1.0, 0.0, 1.0);
                }
            }
        } else {
            discard;
        }
    } else {
        discard;
    }
}
`

// Update the fullscreen overlay fragment shader to show text
const fullscreenOverlayWithTextShader = `
#version 430 core

in vec2 fragCoord;
out vec4 outColor;

uniform vec2 screenSize;
uniform float fps;
uniform float zoom;
uniform float distance;

// Simple text rendering - draws rectangles to simulate text
void drawText(vec2 pixelPos, vec2 textStart, float value, int precision) {
    // This is a placeholder - in reality we'd use a font texture
    // For now, just draw a white rectangle where text would be
    if (pixelPos.x > textStart.x && pixelPos.x < textStart.x + 50.0 &&
        pixelPos.y > textStart.y && pixelPos.y < textStart.y + 12.0) {
        outColor = vec4(1.0, 1.0, 1.0, 1.0);
    }
}

void main() {
    vec2 pixelPos = fragCoord * screenSize;
    
    // Bottom-left overlay
    if (pixelPos.x < 260.0 && pixelPos.y > screenSize.y - 90.0) {
        float bgY = screenSize.y - pixelPos.y; // Flip Y for bottom-left
        
        if (pixelPos.x > 10.0 && pixelPos.x < 260.0 && bgY > 10.0 && bgY < 90.0) {
            // Dark blue background
            outColor = vec4(0.1, 0.1, 0.3, 0.8);
            
            // FPS row
            if (bgY > 20.0 && bgY < 30.0) {
                // Label area
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    // White text placeholder
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Green bar
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(fps, 150.0)) {
                    outColor = vec4(0.0, 1.0, 0.0, 1.0);
                }
            }
            
            // Zoom row
            if (bgY > 40.0 && bgY < 50.0) {
                // Label area
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Blue bar
                float zoomBar = zoom * 100.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(zoomBar, 200.0)) {
                    outColor = vec4(0.5, 0.5, 1.0, 1.0);
                }
            }
            
            // Distance row
            if (bgY > 60.0 && bgY < 70.0) {
                // Label area  
                if (pixelPos.x > 15.0 && pixelPos.x < 65.0) {
                    outColor = vec4(0.9, 0.9, 0.9, 1.0);
                }
                // Yellow bar
                float distBar = distance / 100000.0;
                if (pixelPos.x > 70.0 && pixelPos.x < 70.0 + min(distBar, 200.0)) {
                    outColor = vec4(1.0, 1.0, 0.0, 1.0);
                }
            }
        } else {
            discard;
        }
    } else {
        discard;
    }
}
`