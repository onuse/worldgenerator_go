@echo off
set PATH=C:\msys64\mingw64\bin;C:\msys64\usr\bin;%PATH%

echo Building performance test...
go build -o perf_test.exe perf_test.go gpu_buffer_share_windows.go gpu_buffer_share_plates.go gpu_types.go voxel_planet.go voxel_texture_data.go voxel_physics.go voxel_physics_cpu.go voxel_advection.go voxel_mechanics.go voxel_plates.go voxel_interpolation.go types.go sphere_geometry.go gpu_interface.go gpu_cpu.go gpu_stub.go gpu_metal_interface_stub.go

echo Running performance test...
perf_test.exe