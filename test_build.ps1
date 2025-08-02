# Quick build test
$env:CGO_ENABLED = "1"

Write-Host "Testing build..." -ForegroundColor Green
go build . 2>&1 | Select-Object -First 30

Write-Host "`nCommon fixes needed:" -ForegroundColor Yellow
Write-Host "1. Export types/functions by capitalizing first letter" -ForegroundColor Cyan
Write-Host "2. Add package prefix for cross-package references" -ForegroundColor Cyan
Write-Host "3. Use interface{} to avoid import cycles" -ForegroundColor Cyan