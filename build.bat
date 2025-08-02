@echo off
echo Building Voxel Planet...
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin
set CGO_ENABLED=1

REM Simple build - Go will find all files in subdirectories
go build -o voxel_planet.exe .

if %ERRORLEVEL% EQU 0 (
    echo Build successful!
    echo Run with: voxel_planet.exe
) else (
    echo Build failed!
)