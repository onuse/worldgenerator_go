# PowerShell script to fix all imports and references

Write-Host "Fixing imports and type references..." -ForegroundColor Green

# Fix GPU package files
$gpuFiles = Get-ChildItem -Path gpu -Filter *.go -Recurse
foreach ($file in $gpuFiles) {
    $content = Get-Content $file.FullName -Raw
    $modified = $false
    
    # Check if file needs core import
    if ($content -match '\*?VoxelPlanet\b' -and $content -notmatch 'worldgenerator/core') {
        # Add import if not present
        if ($content -match 'package gpu\s*\n') {
            $content = $content -replace '(package gpu\s*\n)', "`$1`nimport (`n`t`"worldgenerator/core`"`n)`n"
            $modified = $true
        } elseif ($content -match 'import \(') {
            # Add to existing imports
            $content = $content -replace '(import \()', "`$1`n`t`"worldgenerator/core`""
            $modified = $true
        }
    }
    
    # Replace type references
    $content = $content -replace '\*VoxelPlanet\b', '*core.VoxelPlanet'
    $content = $content -replace '\(planet VoxelPlanet\)', '(planet *core.VoxelPlanet)'
    $content = $content -replace '\[\]VoxelMaterial\b', '[]core.VoxelMaterial'
    $content = $content -replace '\*VoxelMaterial\b', '*core.VoxelMaterial'
    $content = $content -replace '\bMaterialType\b', 'core.MaterialType'
    $content = $content -replace '\bSphericalShell\b', 'core.SphericalShell'
    $content = $content -replace '\bVoxelCoord\b', 'core.VoxelCoord'
    
    Set-Content -Path $file.FullName -Value $content -NoNewline
    Write-Host "  Fixed: $($file.Name)" -ForegroundColor Gray
}

# Fix physics package files
$physicsFiles = Get-ChildItem -Path physics -Filter *.go
foreach ($file in $physicsFiles) {
    $content = Get-Content $file.FullName -Raw
    
    # Add imports
    if ($content -match '\*?VoxelPlanet\b' -and $content -notmatch 'worldgenerator/core') {
        if ($content -match 'package physics\s*\n') {
            $content = $content -replace '(package physics\s*\n)', "`$1`nimport (`n`t`"worldgenerator/core`"`n)`n"
        }
    }
    
    # Replace type references
    $content = $content -replace '\*VoxelPlanet\b', '*core.VoxelPlanet'
    $content = $content -replace '\[\]VoxelMaterial\b', '[]core.VoxelMaterial'
    $content = $content -replace '\bVoxelCoord\b', 'core.VoxelCoord'
    
    Set-Content -Path $file.FullName -Value $content -NoNewline
    Write-Host "  Fixed: $($file.Name)" -ForegroundColor Gray
}

# Fix rendering package files
$renderingFiles = Get-ChildItem -Path rendering -Filter *.go -Recurse
foreach ($file in $renderingFiles) {
    $content = Get-Content $file.FullName -Raw
    
    # Add imports
    if ($content -match '\*?VoxelPlanet\b' -and $content -notmatch 'worldgenerator/core') {
        if ($content -match 'package rendering\s*\n') {
            $content = $content -replace '(package rendering\s*\n)', "`$1`nimport (`n`t`"worldgenerator/core`"`n)`n"
        }
    }
    
    # Replace type references
    $content = $content -replace '\*VoxelPlanet\b', '*core.VoxelPlanet'
    $content = $content -replace '\bVoxelTextureData\b', 'VoxelTextureData' # Keep local
    
    Set-Content -Path $file.FullName -Value $content -NoNewline
    Write-Host "  Fixed: $($file.Name)" -ForegroundColor Gray
}

Write-Host "`nDone! Try building now." -ForegroundColor Green