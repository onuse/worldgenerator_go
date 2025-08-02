# Simple multi-package build test
$env:CGO_ENABLED = "1"
$env:PATH = "$env:PATH;C:\msys64\mingw64\bin;C:\msys64\usr\bin"

Write-Host "Building multi-package project..." -ForegroundColor Green

# Build and capture errors
$output = go build -o voxel_planet.exe . 2>&1

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful!" -ForegroundColor Green
} else {
    Write-Host "Build failed. First 20 errors:" -ForegroundColor Red
    $output | Select-Object -First 20 | ForEach-Object { Write-Host $_ }
}