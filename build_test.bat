@echo off
echo Testing multi-package build...
set CGO_ENABLED=1
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin

REM Just try to build
go build -o voxel_planet.exe .

if %ERRORLEVEL% EQU 0 (
    echo Build successful!
) else (
    echo Build failed - checking errors...
    go build -o voxel_planet.exe . 2>&1 | findstr /V "imported and not used"
)