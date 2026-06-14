# sync-deps.ps1
# AI Tutor - Windows dependency sync script
# Usage:
# powershell -ExecutionPolicy Bypass -File .\sync-deps.ps1

$ErrorActionPreference = "Stop"

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
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: 'go mod tidy' failed with exit code $LASTEXITCODE."
    exit 1
}
Write-Host "Go modules synced"

Write-Host ""
Write-Host "Step 3: Verifying key packages..."

$packages = @(
    "github.com/yalue/onnxruntime_go",
    "github.com/daulet/tokenizers",
    "github.com/mattn/go-sqlite3"
)

foreach ($pkg in $packages) {
    go list $pkg *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "$pkg available"
    } else {
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

# Normalize: the VERSION file may or may not include a leading "v", but the
# GitHub release tag always does. Make sure $appVersion has it before it's
# used as the release tag / asset version marker.
if (-not $appVersion.StartsWith("v")) {
    $appVersion = "v$appVersion"
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
$zipPath = Join-Path $appDataDir $zipFilename
$localAssetDir = ".\asset"
$assetVersionFile = "VERSION"
$assets = @(
    "tokenizer.json",
    "model_int8.onnx",
    "onnxruntime.dll",
    "vec0.dll"
)

function Test-AllAssetsExist($dir) {
    foreach ($file in $assets) {
        if (!(Test-Path (Join-Path $dir $file))) {
            return $false
        }
    }
    return $true
}

function Get-InstalledAssetVersion($dir) {
    $versionPath = Join-Path $dir $assetVersionFile
    if (Test-Path $versionPath) {
        return (Get-Content $versionPath -Raw).Trim()
    }
    return $null
}

function Set-InstalledAssetVersion($dir, $version) {
    Set-Content -Path (Join-Path $dir $assetVersionFile) -Value $version -NoNewline
}

$allLocalExist = Test-AllAssetsExist $localAssetDir
$allAppDataExist = Test-AllAssetsExist $appDataDir
$appDataAssetVersion = Get-InstalledAssetVersion $appDataDir

# AppData is only considered "good" if every file is present AND it was
# stamped as belonging to the current $appVersion. A stale cache from an
# older release (or one without a version marker at all) does not count.
$appDataCurrent = $allAppDataExist -and ($appDataAssetVersion -eq $appVersion)

if ($appDataCurrent) {
    Write-Host "RAG assets present in AppData directory ($appDataDir) and match version $appVersion."
}
elseif ($allLocalExist) {
    Write-Host "RAG assets detected in local workspace ($localAssetDir). Syncing to AppData ($appDataDir)..."
    if (!(Test-Path $appDataDir)) {
        New-Item -ItemType Directory -Force -Path $appDataDir | Out-Null
    }
    foreach ($file in $assets) {
        Copy-Item -Path (Join-Path $localAssetDir $file) -Destination (Join-Path $appDataDir $file) -Force
    }
    # Treat manually-placed local assets as belonging to the current version.
    Set-InstalledAssetVersion $appDataDir $appVersion
    Set-InstalledAssetVersion $localAssetDir $appVersion
    Write-Host "Sync complete."
}
else {
    if ($allAppDataExist) {
        $shownVersion = if ($appDataAssetVersion) { $appDataAssetVersion } else { "unknown" }
        Write-Host "AppData cache contains assets for version '$shownVersion', but '$appVersion' is required."
        Write-Host "Re-downloading assets for $appVersion..."
    } else {
        Write-Host "RAG assets missing from both AppData and local workspace."
        Write-Host "Attempting to download RAG assets from $downloadUrl..."
    }

    # Ensure target directory exists
    if (!(Test-Path $appDataDir)) {
        New-Item -ItemType Directory -Force -Path $appDataDir | Out-Null
    }

    try {
        if (Get-Command "curl.exe" -ErrorAction SilentlyContinue) {
            curl.exe -L -f -o $zipPath $downloadUrl
            if ($LASTEXITCODE -ne 0) {
                throw "curl.exe exited with code $LASTEXITCODE."
            }
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

        # Verify the archive actually produced the expected files before
        # declaring success or stamping the cache as current.
        if (Test-AllAssetsExist $appDataDir) {
            Set-InstalledAssetVersion $appDataDir $appVersion
            Write-Host "Extraction complete."
        } else {
            Write-Host "Warning: extraction finished, but expected asset files are missing:"
            foreach ($file in $assets) {
                if (!(Test-Path (Join-Path $appDataDir $file))) {
                    Write-Host "  - $file"
                }
            }
            Write-Host "Archive layout may have changed for $appVersion. Not marking AppData cache as current."
            $missingAssets = $true
        }
    } catch {
        Write-Host "Warning: Could not download RAG assets: $_"
        Write-Host "Please place the following files manually in '$localAssetDir\':"
        foreach ($file in $assets) {
            Write-Host "  - $file"
        }
        $missingAssets = $true
    } finally {
        if (Test-Path $zipPath) {
            Remove-Item $zipPath -Force
        }
    }
}

# If AppData now holds a complete, current set of assets, make sure the local
# workspace matches too (covers both "files missing locally" and "local files
# are present but stamped with an older/different version").
$appDataNowCurrent = (Test-AllAssetsExist $appDataDir) -and ((Get-InstalledAssetVersion $appDataDir) -eq $appVersion)
$localVersionNow = Get-InstalledAssetVersion $localAssetDir

if ($appDataNowCurrent -and ($localVersionNow -ne $appVersion)) {
    Write-Host "Refreshing local workspace assets ($localAssetDir) to version $appVersion from AppData cache..."
    if (!(Test-Path $localAssetDir)) {
        New-Item -ItemType Directory -Force -Path $localAssetDir | Out-Null
    }
    foreach ($file in $assets) {
        Copy-Item -Path (Join-Path $appDataDir $file) -Destination (Join-Path $localAssetDir $file) -Force
    }
    Set-InstalledAssetVersion $localAssetDir $appVersion
}

# Final check: confirm a complete, current asset set exists in AppData or
# locally, regardless of how we got there.
$appDataFinalCurrent = (Test-AllAssetsExist $appDataDir) -and ((Get-InstalledAssetVersion $appDataDir) -eq $appVersion)
$localFinalCurrent = (Test-AllAssetsExist $localAssetDir) -and ((Get-InstalledAssetVersion $localAssetDir) -eq $appVersion)
if (-not ($appDataFinalCurrent -or $localFinalCurrent)) {
    $missingAssets = $true
}

Write-Host ""
Write-Host "================================"
if ($missingAssets) {
    Write-Host "Dependency sync completed with warnings!"
    Write-Host "RAG assets for $appVersion are missing or incomplete - place them manually before building (see above)."
} else {
    Write-Host "Dependency sync complete!"
}
Write-Host "================================"
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Build with:"
Write-Host "   go build -tags sqlite_extension -o build\bin\ai-tutor.exe ."
Write-Host "2. Or run:"
Write-Host "   wails dev"