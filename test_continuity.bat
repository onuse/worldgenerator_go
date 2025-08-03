@echo off
voxel_planet.exe > output.txt 2>&1
timeout /t 5 >nul
taskkill /F /IM voxel_planet.exe >nul 2>&1
findstr /C:"FPS:" /C:"ADVECTION:" /C:"Plate" output.txt | findstr /V "Created voxel planet"
del output.txt