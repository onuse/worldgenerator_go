#include <metal_stdlib>
using namespace metal;

struct Vertex {
    float3 position;
    float height;
    int plateID;
};

struct Plate {
    float3 center;
    float3 velocity;
    int type; // 0 = Continental, 1 = Oceanic
};

kernel void updateTectonics(device Vertex* vertices [[buffer(0)]],
                           constant Plate* plates [[buffer(1)]],
                           constant float& deltaTime [[buffer(2)]],
                           uint index [[thread_position_in_grid]],
                           uint numVertices [[threads_per_grid]]) {
    
    if (index >= numVertices) return;
    
    int plateID = vertices[index].plateID;
    if (plateID < 0 || plateID >= 32) return;
    
    float3 velocity = plates[plateID].velocity;
    float velocityMag = length(velocity);
    float uplift = velocityMag * deltaTime * 0.00001f;
    
    if (plates[plateID].type == 0) { // Continental
        vertices[index].height += uplift;
    } else { // Oceanic
        vertices[index].height -= uplift * 0.5f;
    }
    
    // Clamp heights
    vertices[index].height = clamp(vertices[index].height, -0.02f, 0.02f);
}