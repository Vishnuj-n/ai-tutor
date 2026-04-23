# sync-deps.ps1
# AI Tutor - Windows dependency sync script
# Usage:
# powershell -ExecutionPolicy Bypass -File .\sync-deps.ps1

$ErrorActionPreference = "Stop"
$missingAssets = $false

Write-Host "================================"
Write-Host "AI Tutor: Syncing Dependencies"
Write-Host "================================"

# Ensure project root
if (!(Test-Path ".\go.mod")) {
    Write-Host "Error: go.mod not found. Run this script from the project root."
    exit 1
}

Write-Host ""
Write-Host "Step 1: Verifying Go installation..."

try {
    $goVersion = go version
    Write-Host "Go detected: $goVersion"
}
catch {
    Write-Host "Error: Go is not installed or not in PATH."
    exit 1
}

Write-Host ""
Write-Host "Step 2: Downloading Go module dependencies..."
go mod tidy
Write-Host "Go modules synced"

Write-Host ""
Write-Host "Step 3: Verifying key packages..."

$packages = @(
    "github.com/yalue/onnxruntime_go",
    "github.com/daulet/tokenizers",
    "github.com/mattn/go-sqlite3"
)

foreach ($pkg in $packages) {
    try {
        go list $pkg *> $null
        Write-Host "$pkg available"
    }
    catch {
        Write-Host "$pkg will download during build"
    }
}

Write-Host ""
Write-Host "Step 4: Validating assets..."

$assets = @(
    "asset\tokenizer.json",
    "asset\model_int8.onnx",
    "asset\onnxruntime.dll",
    "asset\vec0.dll"
)

foreach ($file in $assets) {
    if (Test-Path $file) {
        Write-Host "$file present"
    }
    else {
        Write-Host "$file MISSING"
        $missingAssets = $true
    }
}

if ($missingAssets) {
    Write-Host ""
    Write-Host "================================"
    Write-Host "Dependency sync failed: missing assets"
    Write-Host "================================"
    exit 1
}

Write-Host ""
Write-Host "================================"
Write-Host "Dependency sync complete!"
Write-Host "================================"
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Build with:"
Write-Host "   go build -tags sqlite_extension -o build\bin\ai-tutor.exe ."
Write-Host "2. Or run:"
Write-Host "   wails dev"

