# PowerShell script to help identify and fix exports

Write-Host "Analyzing types and functions that need to be exported..." -ForegroundColor Green

# Find all undefined symbols in main.go
$mainContent = Get-Content main.go -Raw
$errors = @()

# Common patterns that need fixing
$patterns = @{
    "VoxelPlanet" = "core"
    "SphericalShell" = "core"
    "VoxelMaterial" = "core"
    "MaterialType" = "core"
    "VoxelCoord" = "core"
    "ComputePhysics" = "physics"
    "UpdateVoxelPhysicsWrapper" = "physics"
    "WindowsGPUBufferManager" = "gpu"
    "SharedGPUBuffers" = "gpu"
    "GPUVoxelMaterial" = "gpu"
    "UpdateSharedBuffersWithPlates" = "simulation"
}

Write-Host "`nTypes/functions that likely need to be exported (capitalized):" -ForegroundColor Yellow

foreach ($pattern in $patterns.Keys) {
    Write-Host "  $pattern -> $($patterns[$pattern]).$pattern" -ForegroundColor Cyan
}

Write-Host "`nTo fix: Capitalize the first letter of exported types/functions in their packages" -ForegroundColor Yellow