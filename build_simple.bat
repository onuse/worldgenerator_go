@echo off
echo Building Voxel Planet...
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin
set CGO_ENABLED=1

REM Build with all Go files in all subdirectories
go build -o voxel_planet.exe ./...

if %ERRORLEVEL% EQU 0 (
    echo Build successful!
    echo Run with: voxel_planet.exe
) else (
    echo Build failed! Trying alternative method...
    REM List all go files explicitly
    go build -o voxel_planet.exe main.go core/*.go physics/*.go gpu/*.go rendering/opengl/*.go rendering/textures/*.go simulation/*.go config/*.go ui/*.go gpu/metal/*.go gpu/opencl/*.go rendering/opengl/shaders/*.go rendering/opengl/overlay/*.go
)