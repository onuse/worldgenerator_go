# PowerShell script to revert all files back to package main
Write-Host "Reverting all packages back to 'package main' for now..." -ForegroundColor Yellow

# Find all Go files and change package declarations back to main
Get-ChildItem -Path . -Filter *.go -Recurse | 
    Where-Object { $_.DirectoryName -notlike "*archive*" -and $_.DirectoryName -notlike "*.git*" } |
    ForEach-Object {
        $content = Get-Content $_.FullName -Raw
        if ($content -match '^package \w+') {
            $newContent = $content -replace '^package \w+', 'package main'
            Set-Content -Path $_.FullName -Value $newContent -NoNewline
            Write-Host "  Updated: $($_.FullName)" -ForegroundColor Gray
        }
    }

# Also remove the module imports from main.go
$mainContent = Get-Content main.go -Raw
$mainContent = $mainContent -replace '(?s)import \([^)]+\)', 'import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
)'
$mainContent = $mainContent -replace 'core\.', ''
$mainContent = $mainContent -replace 'gpu\.', ''
$mainContent = $mainContent -replace 'physics\.', ''
$mainContent = $mainContent -replace 'rendering\.', ''
$mainContent = $mainContent -replace 'simulation\.', ''
$mainContent = $mainContent -replace 'metal\.', ''
$mainContent = $mainContent -replace 'opencl\.', ''
Set-Content -Path main.go -Value $mainContent -NoNewline

Write-Host "`nDone! Now try building with:" -ForegroundColor Green
Write-Host "  go build ." -ForegroundColor Cyan