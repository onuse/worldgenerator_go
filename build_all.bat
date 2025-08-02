@echo off
echo Building Voxel Planet (collecting all .go files)...
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin
set CGO_ENABLED=1

REM Explicitly list all directories with Go files
set FILES=main.go
set FILES=%FILES% core\*.go
set FILES=%FILES% physics\*.go
set FILES=%FILES% gpu\*.go
set FILES=%FILES% gpu\metal\*.go
set FILES=%FILES% gpu\opencl\*.go
set FILES=%FILES% rendering\opengl\*.go
set FILES=%FILES% rendering\opengl\shaders\*.go
set FILES=%FILES% rendering\opengl\overlay\*.go
set FILES=%FILES% rendering\textures\*.go
set FILES=%FILES% simulation\*.go
set FILES=%FILES% config\*.go
set FILES=%FILES% ui\*.go

echo Building with files: %FILES%
go build -o voxel_planet.exe %FILES%

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Build successful!
    echo Run with: voxel_planet.exe
) else (
    echo.
    echo Build failed!
    echo Try running: go build -o voxel_planet.exe .
)