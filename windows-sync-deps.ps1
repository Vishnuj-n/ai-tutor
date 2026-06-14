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
Write-Host "Step 4: Validating and acquiring RAG assets..."

# Ensure TLS 1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Resolve app version — read from VERSION file if present, otherwise default to v1.0.0
$appVersion = "v1.0.0"
if (Test-Path ".\VERSION") {
    $appVersion = (Get-Content ".\VERSION" -Raw).Trim()
}

# Determine the correct zip filename for this version.
# v1.0.0 was released as "rag-assets.zip" (legacy name).
# Subsequent versions follow "asset_windows.zip".
$normalizedVersion = $appVersion.TrimStart("v")
if ($normalizedVersion -eq "1.0.0") {
    $zipFilename = "rag-assets.zip"
} else {
    $zipFilename = "asset_windows.zip"
}

$releaseTag = $appVersion
$downloadUrl = "https://github.com/Vishnuj-n/ai-tutor/releases/download/$releaseTag/$zipFilename"
Write-Host "Asset version: $appVersion  Archive: $zipFilename"

$appDataDir = Join-Path $env:LOCALAPPDATA "ai-tutor\assets"
$zipPath = Join-Path $appDataDir "rag-assets.zip"
$localAssetDir = ".\asset"
$assets = @(
    "tokenizer.json",
    "model_int8.onnx",
    "onnxruntime.dll",
    "vec0.dll"
)

# Check local workspace
$allLocalExist = $true
foreach ($file in $assets) {
    if (!(Test-Path (Join-Path $localAssetDir $file))) {
        $allLocalExist = $false
        break
    }
}

# Check AppData
$allAppDataExist = $true
foreach ($file in $assets) {
    if (!(Test-Path (Join-Path $appDataDir $file))) {
        $allAppDataExist = $false
        break
    }
}

if ($allAppDataExist) {
    Write-Host "RAG assets present in AppData directory ($appDataDir)."
}
elseif ($allLocalExist) {
    Write-Host "RAG assets detected in local workspace ($localAssetDir). Syncing to AppData ($appDataDir)..."
    if (!(Test-Path $appDataDir)) {
        New-Item -ItemType Directory -Force -Path $appDataDir | Out-Null
    }
    foreach ($file in $assets) {
        Copy-Item -Path (Join-Path $localAssetDir $file) -Destination (Join-Path $appDataDir $file) -Force
    }
    Write-Host "Sync complete."
}
else {
    Write-Host "RAG assets missing from both AppData and local workspace."
    Write-Host "Attempting to download RAG assets from $downloadUrl..."

    # Ensure target directory exists
    if (!(Test-Path $appDataDir)) {
        New-Item -ItemType Directory -Force -Path $appDataDir | Out-Null
    }

    try {
        if (Get-Command "curl.exe" -ErrorAction SilentlyContinue) {
            curl.exe -L -f -o $zipPath $downloadUrl
        } else {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath
        }
        
        # Check size of zip - if it's very small, it's likely a 404 error page / message
        $zipSize = (Get-Item $zipPath).Length
        if ($zipSize -lt 10240) { # Less than 10KB
            throw "Downloaded file is too small ($zipSize bytes), possibly a 404 page."
        }

        Write-Host "Download complete. Extracting files..."
        Expand-Archive -Path $zipPath -DestinationPath $appDataDir -Force
        Write-Host "Extraction complete."
    }
    catch {
        Write-Host "Warning: Could not download RAG assets: $_"
        Write-Host "Please place the following files manually in '$localAssetDir\':"
        foreach ($file in $assets) {
            Write-Host "  - $file"
        }
    }
    finally {
        if (Test-Path $zipPath) {
            Remove-Item $zipPath -Force
        }
    }
}

# If we have assets in AppData but missing in workspace, copy them to workspace
if (Test-Path $appDataDir) {
    foreach ($file in $assets) {
        $src = Join-Path $appDataDir $file
        $dst = Join-Path $localAssetDir $file
        if ((Test-Path $src) -and !(Test-Path $dst)) {
            Write-Host "Copying $file from AppData to local workspace ($localAssetDir)..."
            if (!(Test-Path $localAssetDir)) {
                New-Item -ItemType Directory -Force -Path $localAssetDir | Out-Null
            }
            Copy-Item -Path $src -Destination $dst -Force
        }
    }
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

