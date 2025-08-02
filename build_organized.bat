@echo off
echo Building with new organized structure...
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin
set CGO_ENABLED=1

REM Build from root, including all subdirectories
go build -o voxel_planet.exe ./cmd/voxel_planet

if %ERRORLEVEL% EQU 0 (
    echo Build successful!
) else (
    echo Build failed! Trying with all files...
    REM Fallback: build with explicit file listing
    go build -o voxel_planet.exe ./cmd/voxel_planet/*.go ./core/*.go ./physics/*.go ./gpu/*.go ./rendering/opengl/*.go ./rendering/textures/*.go ./simulation/*.go ./config/*.go ./ui/*.go ./gpu/metal/*.go ./gpu/opencl/*.go ./rendering/opengl/shaders/*.go ./rendering/opengl/overlay/*.go
)