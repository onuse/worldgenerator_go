# Find all cross-package references that need fixing

Write-Host "Analyzing cross-package references..." -ForegroundColor Green

# Common types that are referenced across packages
$types = @(
    "VoxelPlanet",
    "VoxelMaterial", 
    "SphericalShell",
    "MaterialType",
    "VoxelCoord",
    "GPUVoxelMaterial",
    "SharedGPUBuffers",
    "VoxelPhysics",
    "PlateTectonics"
)

foreach ($type in $types) {
    Write-Host "`nSearching for $type references:" -ForegroundColor Yellow
    
    # Find in gpu package
    $gpuRefs = Get-ChildItem -Path gpu -Filter *.go -Recurse | Select-String -Pattern "\*?$type\b" -CaseSensitive
    if ($gpuRefs) {
        Write-Host "  In gpu/: Found $($gpuRefs.Count) references" -ForegroundColor Cyan
    }
    
    # Find in physics package  
    $physicsRefs = Get-ChildItem -Path physics -Filter *.go -Recurse | Select-String -Pattern "\*?$type\b" -CaseSensitive
    if ($physicsRefs) {
        Write-Host "  In physics/: Found $($physicsRefs.Count) references" -ForegroundColor Cyan
    }
    
    # Find in rendering package
    $renderRefs = Get-ChildItem -Path rendering -Filter *.go -Recurse | Select-String -Pattern "\*?$type\b" -CaseSensitive
    if ($renderRefs) {
        Write-Host "  In rendering/: Found $($renderRefs.Count) references" -ForegroundColor Cyan
    }
}