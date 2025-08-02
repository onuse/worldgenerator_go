@echo off
REM Fast build script with caching
set PATH=%PATH%;C:\msys64\mingw64\bin;C:\msys64\usr\bin
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

REM Use build cache
set GOCACHE=%LOCALAPPDATA%\go-build

echo Building with caching enabled...
go build -o voxel_planet.exe .
echo Build complete!