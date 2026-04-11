#!/usr/bin/env bash
# Build script for AI Tutor with local ONNX + sqlite-vec RAG support
# Usage: ./build.sh [target]
# Targets: dev, prod, clean

set -e

TARGET=${1:-dev}
BINARY_NAME="ai-tutor"
OUTPUT_DIR="build/bin"

# Ensure CGO is enabled for ONNX and SQLite extension support
export CGO_ENABLED=1

# Windows-specific build (cross-compilation from non-Windows requires additional setup)
# For native Windows build, run in Windows PowerShell or cmd.exe
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "windows-nt" ]]; then
    export GOOS=windows
    export GOARCH=amd64
fi

case $TARGET in
    dev)
        echo "Building AI Tutor (dev, with sqlite extension support)..."
        go build -tags sqlite_extension -o "$OUTPUT_DIR/$BINARY_NAME" .
        echo "✓ Build complete: $OUTPUT_DIR/$BINARY_NAME"
        ;;
    prod)
        echo "Building AI Tutor with Wails (prod)..."
        CGO_ENABLED=1 wails build -platform windows/amd64 -o ai-tutor.exe
        echo "✓ Wails build complete"
        ;;
    clean)
        echo "Cleaning build artifacts..."
        rm -rf "$OUTPUT_DIR"
        go clean
        rm -rf build/bin
        echo "✓ Clean complete"
        ;;
    *)
        echo "Usage: $0 [dev|prod|clean]"
        exit 1
        ;;
esac
