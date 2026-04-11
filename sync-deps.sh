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
echo "Step 3: Validating asset directory..."
if [ -f "asset/tokenizer.json" ]; then
    echo "✓ asset/tokenizer.json present"
else
    echo "❌ asset/tokenizer.json MISSING (required for ONNX)"
    MISSING_ASSETS=1
fi

if [ -f "asset/model_int8.onnx" ]; then
    echo "✓ asset/model_int8.onnx present"
else
    echo "❌ asset/model_int8.onnx MISSING (required for ONNX)"
    MISSING_ASSETS=1
fi

if [ -f "asset/onnxruntime.dll" ]; then
    echo "✓ asset/onnxruntime.dll present"
else
    echo "❌ asset/onnxruntime.dll MISSING (required for Windows build)"
    MISSING_ASSETS=1
fi

if [ -f "asset/vec0.dll" ]; then
    echo "✓ asset/vec0.dll present"
else
    echo "❌ asset/vec0.dll MISSING (required for sqlite-vec)"
    MISSING_ASSETS=1
fi

if [ "$MISSING_ASSETS" -eq 1 ]; then
    echo ""
    echo "================================"
    echo "❌ Dependency sync failed: required assets are missing"
    echo "================================"
    echo ""
    echo "Fix missing assets in asset/ and rerun ./sync-deps.sh"
    exit 1
fi

echo ""
echo "================================"
echo "✓ Dependency sync complete!"
echo "================================"
echo ""
echo "Next steps:"
echo "1. Verify all required assets are present in asset/ directory"
echo "2. Build with: CGO_ENABLED=1 go build -tags sqlite_extension -o build/bin/ai-tutor.exe ."
echo ""
