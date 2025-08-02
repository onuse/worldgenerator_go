@echo off
echo Upgrading OpenGL imports from 4.1 to 4.3...

powershell -Command "(Get-Content renderer_gl.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl.go"
powershell -Command "(Get-Content voxel_texture_data.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content voxel_texture_data.go"
powershell -Command "(Get-Content gpu_buffer_share_plates.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content gpu_buffer_share_plates.go"
powershell -Command "(Get-Content gpu_buffer_share_cuda.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content gpu_buffer_share_cuda.go"
powershell -Command "(Get-Content renderer_gl_raymarch.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl_raymarch.go"
powershell -Command "(Get-Content renderer_gl_ssbo.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl_ssbo.go"
powershell -Command "(Get-Content gpu_buffer_share_windows.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content gpu_buffer_share_windows.go"
powershell -Command "(Get-Content renderer_gl_volume.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl_volume.go"
powershell -Command "(Get-Content renderer_gl_ssbo_v2.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl_ssbo_v2.go"
powershell -Command "(Get-Content gpu_buffer_extended.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content gpu_buffer_extended.go"
powershell -Command "(Get-Content sphere_geometry.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content sphere_geometry.go"
powershell -Command "(Get-Content renderer_gl_splat.go) -replace 'github.com/go-gl/gl/v4.1-core/gl', 'github.com/go-gl/gl/v4.3-core/gl' | Set-Content renderer_gl_splat.go"

echo Done! Now run: go get github.com/go-gl/gl/v4.3-core/gl