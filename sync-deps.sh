#!/usr/bin/env bash
# Sync Go dependencies and prepare for build with ONNX + sqlite-vec
# Usage: ./sync-deps.sh

set -e

MISSING_ASSETS=0

echo "================================"
echo "AI Tutor: Syncing Dependencies"
echo "================================"

# Ensure we're in the project root
if [ ! -f "go.mod" ]; then
    echo "❌ Error: go.mod not found. Run this script from the project root."
    exit 1
fi

echo ""
echo "Step 1: Downloading Go module dependencies..."
go mod tidy
echo "✓ Go modules synced"

echo ""
echo "Step 2: Verifying key imports..."
if go list github.com/yalue/onnxruntime_go > /dev/null 2>&1; then
    echo "✓ github.com/yalue/onnxruntime_go available"
else
    echo "⚠ github.com/yalue/onnxruntime_go not yet cached (will download on build)"
fi

if go list github.com/daulet/tokenizers > /dev/null 2>&1; then
    echo "✓ github.com/daulet/tokenizers available"
else
    echo "⚠ github.com/daulet/tokenizers not yet cached (will download on build)"
fi

if go list github.com/mattn/go-sqlite3 > /dev/null 2>&1; then
    echo "✓ github.com/mattn/go-sqlite3 available"
else
    echo "⚠ github.com/mattn/go-sqlite3 not yet cached (will download on build)"
fi

echo ""
echo "Step 3: Validating and acquiring RAG assets..."

# Resolve OS-specific asset cache directory
OS_TYPE="$(uname -s)"
if [ "$OS_TYPE" = "Darwin" ]; then
    TARGET_DIR="$HOME/Library/Caches/ai-tutor/assets"
else
    TARGET_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/ai-tutor/assets"
fi

# Resolve app version — read from VERSION file if present, otherwise default to v1.0.0
APP_VERSION="v1.0.0"
if [ -f "VERSION" ]; then
    APP_VERSION="$(cat VERSION | tr -d '[:space:]')"
fi

# Normalize: strip leading "v"
NORMALIZED_VERSION="${APP_VERSION#v}"

# Determine the correct zip filename for this version and platform.
# v1.0.0 was released as "rag-assets.zip" (legacy, Windows-only).
# Subsequent releases follow "asset_<os>.zip".
if [ "$NORMALIZED_VERSION" = "1.0.0" ]; then
    ZIP_FILENAME="rag-assets.zip"
elif [ "$OS_TYPE" = "Darwin" ]; then
    ZIP_FILENAME="asset_darwin.zip"
else
    ZIP_FILENAME="asset_linux.zip"
fi

DOWNLOAD_URL="https://github.com/Vishnuj-n/ai-tutor/releases/download/${APP_VERSION}/${ZIP_FILENAME}"
echo "Asset version: ${APP_VERSION}  Archive: ${ZIP_FILENAME}"

ZIP_PATH="$TARGET_DIR/rag-assets.zip"

# Platform-specific required file names
if [ "$OS_TYPE" = "Darwin" ]; then
    REQUIRED_FILES=("tokenizer.json" "model_int8.onnx" "libonnxruntime.dylib" "vec0.dylib")
else
    REQUIRED_FILES=("tokenizer.json" "model_int8.onnx" "libonnxruntime.so" "vec0.so")
fi

# Check cache
MISSING_FROM_CACHE=0
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$TARGET_DIR/$file" ]; then
        MISSING_FROM_CACHE=1
        break
    fi
done

# Check local workspace
MISSING_FROM_WORKSPACE=0
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "asset/$file" ]; then
        MISSING_FROM_WORKSPACE=1
        break
    fi
done

if [ "$MISSING_FROM_CACHE" -eq 0 ]; then
    echo "✓ RAG assets present in cache ($TARGET_DIR)"
elif [ "$MISSING_FROM_WORKSPACE" -eq 0 ]; then
    echo "RAG assets detected in local workspace (asset/). Syncing to cache ($TARGET_DIR)..."
    mkdir -p "$TARGET_DIR"
    for file in "${REQUIRED_FILES[@]}"; do
        cp "asset/$file" "$TARGET_DIR/$file"
    done
    echo "✓ Sync complete."
else
    echo "RAG assets missing from both cache and local workspace."
    echo "Attempting to download assets from $DOWNLOAD_URL..."
    mkdir -p "$TARGET_DIR"
    
    DOWNLOAD_SUCCESS=0
    if command -v curl >/dev/null 2>&1; then
        if curl -L -f -o "$ZIP_PATH" "$DOWNLOAD_URL"; then
            DOWNLOAD_SUCCESS=1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -O "$ZIP_PATH" "$DOWNLOAD_URL"; then
            DOWNLOAD_SUCCESS=1
        fi
    fi
    
    if [ "$DOWNLOAD_SUCCESS" -eq 1 ] && [ -f "$ZIP_PATH" ]; then
        ZIP_SIZE=$(wc -c < "$ZIP_PATH" 2>/dev/null || stat -c %s "$ZIP_PATH" 2>/dev/null || echo 0)
        if [ "$ZIP_SIZE" -lt 10240 ]; then
            echo "⚠ Downloaded file is too small ($ZIP_SIZE bytes), likely a 404 page."
            rm -f "$ZIP_PATH"
        else
            echo "Download complete. Extracting..."
            if command -v unzip >/dev/null 2>&1; then
                unzip -o "$ZIP_PATH" -d "$TARGET_DIR"
                rm -f "$ZIP_PATH"
                echo "Extraction complete."
            else
                echo "❌ Error: unzip utility not found. Cannot extract downloaded assets."
                rm -f "$ZIP_PATH"
            fi
        fi
    else
        echo "⚠ Warning: Could not download RAG assets from GitHub (release may not be published yet, or offline)."
        echo "Please place the following files manually in asset/ directory:"
        for file in "${REQUIRED_FILES[@]}"; do
            echo "  - $file"
        done
    fi
fi

# Ensure local workspace `asset/` has them for dev/build coupling
mkdir -p asset
for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$TARGET_DIR/$file" ]; then
        if [ ! -f "asset/$file" ]; then
            echo "Copying $file from cache to local workspace (asset/)..."
            cp "$TARGET_DIR/$file" "asset/$file"
        else
            echo "✓ asset/$file present"
        fi
    fi
done

echo ""
echo "================================"
echo "✓ Dependency sync complete!"
echo "================================"
echo ""
echo "Next steps:"
echo "1. Verify all required assets are present in asset/ directory"
if [ "$OS_TYPE" = "Darwin" ]; then
    echo "2. Build with: CGO_ENABLED=1 go build -tags sqlite_extension -o build/bin/ai-tutor ."
else
    echo "2. Build with: CGO_ENABLED=1 go build -tags sqlite_extension -o build/bin/ai-tutor ."
fi
echo ""
