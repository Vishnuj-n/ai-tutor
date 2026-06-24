# prepare_release_assets.ps1
# AI Tutor: Release asset packaging helper script
# Generates manifest.json with SHA-256 checksums and creates build\bin\rag-assets.zip

$ErrorActionPreference = "Stop"

$appVersion = "v1.0.0"
if (Test-Path ".\VERSION") {
    $appVersion = (Get-Content ".\VERSION" -Raw).Trim()
}
if (-not $appVersion.StartsWith("v")) {
    $appVersion = "v$appVersion"
}
$normalizedVersion = $appVersion.TrimStart("v")

Write-Host "=========================================="
Write-Host "AI Tutor: Packaging RAG Assets ($appVersion)"
Write-Host "=========================================="

$assetDir = ".\asset"
$outDir = ".\build\bin"

if (!(Test-Path $outDir)) {
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
}

$files = @("tokenizer.json", "model_int8.onnx", "onnxruntime.dll", "vec0.dll")

# Check that all source files exist
foreach ($file in $files) {
    $path = Join-Path $assetDir $file
    if (!(Test-Path $path)) {
        Write-Host "Error: Required asset file missing: $path" -ForegroundColor Red
        exit 1
    }
}

# Calculate SHA256 hashes
$hashes = @{}
foreach ($file in $files) {
    $path = Join-Path $assetDir $file
    Write-Host "Calculating SHA-256 hash for $file..."
    $hashVal = (Get-FileHash -Path $path -Algorithm SHA256).Hash.ToUpper()
    $hashes[$file] = $hashVal
}

# Generate manifest.json content
$manifest = [ordered]@{
    assetVersion = $normalizedVersion
    downloadUrl = "https://github.com/Vishnuj-n/ai-tutor/releases/download/$appVersion/rag-assets.zip"
    requiredFiles = $files
    fileHashes = $hashes
}

# Save manifest.json inside asset directory temporarily
$manifestPath = Join-Path $assetDir "manifest.json"
$manifest | ConvertTo-Json | Set-Content -Path $manifestPath -Force
Write-Host "Generated temporary manifest.json at $manifestPath"

# Zip them up
$zipPath = Join-Path $outDir "rag-assets.zip"
if (Test-Path $zipPath) {
    Remove-Item $zipPath -Force
}

Write-Host "Creating zip archive at $zipPath..."
$compressFiles = $files | ForEach-Object { Join-Path $assetDir $_ }
$compressFiles += $manifestPath

# Create Zip Archive
Compress-Archive -Path $compressFiles -DestinationPath $zipPath -Force

# Clean up temp manifest
Remove-Item $manifestPath -Force

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "Success! Generated rag-assets.zip in build\bin\" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
