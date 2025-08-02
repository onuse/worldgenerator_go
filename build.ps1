# PowerShell build script for Voxel Planet
Write-Host "Building Voxel Planet..." -ForegroundColor Green

# Set environment variables
$env:CGO_ENABLED = "1"
$env:PATH = "$env:PATH;C:\msys64\mingw64\bin;C:\msys64\usr\bin"

# Get all Go files except archive and test files
$goFiles = Get-ChildItem -Path . -Filter *.go -Recurse | 
    Where-Object { 
        $_.DirectoryName -notlike "*archive*" -and 
        $_.DirectoryName -notlike "*.git*" -and
        $_.Name -ne "perf_test.go"
    } | 
    ForEach-Object { $_.FullName }

Write-Host "Found $($goFiles.Count) Go files" -ForegroundColor Yellow

# Build command
$buildArgs = @("-o", "voxel_planet.exe") + $goFiles

# Execute build
& go build $buildArgs

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nBuild successful!" -ForegroundColor Green
    Write-Host "Run with: .\voxel_planet.exe" -ForegroundColor Cyan
} else {
    Write-Host "`nBuild failed!" -ForegroundColor Red
}