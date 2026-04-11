# Build Constraints for Local RAG

This document captures the build-time requirements for compiling the AI Tutor with local ONNX + sqlite-vec RAG support.

## Required Environment Variables at Build Time

```bash
# Windows x86-64 build
export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64

# Recommended: Ensure MSVC toolchain is available (for CGO compilation)
# On Windows: Run from Visual Studio Developer Command Prompt or set up MinGW
```

## Build Commands

### Windows (x86-64, with sqlite extension support)

**Development build**:
```bash
CGO_ENABLED=1 go build -tags sqlite_extension -o build/bin/windows/ai-tutor.exe .
```

**Production build (Wails)**:
```bash
CGO_ENABLED=1 wails build -platform windows/amd64 -o ai-tutor.exe
```

### macOS / Linux

**Build**:
```bash
CGO_ENABLED=1 go build -tags sqlite_extension -o build/bin/ai-tutor .
```

Note: onnxruntime and vec0 are platform-specific; ensure correct binaries for target platform (coming in Phase 10).

## Build Tag Explanation

- `-tags sqlite_extension`: Enables SQLite extension loading support via `load_extension()` SQL function.
  - Required by `mattn/go-sqlite3` to allow loading `vec0.dll` at runtime.
  - Without this tag, `SELECT load_extension(...)` will fail.

## CGO Requirement Explanation

- **Why**: Both `yalue/onnxruntime_go` and `sqlite-vec` (via `mattn/go-sqlite3` extension loading) require C interop.
  - `onnxruntime_go` binds to native ONNX Runtime inference library.
  - `mattn/go-sqlite3` with extensions binds to native SQLite C API.
- **How**: Set `CGO_ENABLED=1` before build.
- **Compiler**: On Windows, requires MSVC (part of Visual Studio) or MinGW-w64.

## Dependency Chain

1. `go.mod` declares:
   - `github.com/yalue/onnxruntime_go` (ONNX inference)
   - `github.com/daulet/tokenizers` (Hugging Face tokenizer wrapper)
   - `github.com/mattn/go-sqlite3` (SQLite driver with extension support)

2. These wrap native C libraries:
   - `onnxruntime.dll` (must be available at runtime, placed in `asset/`)
   - `vec0.dll` (must be available at runtime, placed in `asset/`)
   - `msvcrt.dll` / standard C runtime (usually present on Windows)

## Validation at Startup

On app startup, `internal/runtime/AssetValidator` checks:
1. `asset/tokenizer.json` exists and is readable
2. `asset/model_int8.onnx` exists and is readable
3. `asset/onnxruntime.dll` exists and is readable
4. `asset/vec0.dll` exists and is readable

If any file is missing, app logs explicit error and fails startup with message:
```
missing required assets: [tokenizer.json, ...]
```

## Troubleshooting

### Build Error: "CGO compiler not found"
- **Cause**: CGO enabled but no C compiler in PATH.
- **Fix (Windows)**: 
  - Use Visual Studio Developer Command Prompt, OR
  - Install mingw-w64 and add to PATH, OR
  - Disable CGO (not recommended for this project): `CGO_ENABLED=0`

### Build Error: "sqlite_extension tag not recognized"
- **Cause**: Build without `-tags` flag or typo.
- **Fix**: Use exact flag: `-tags sqlite_extension`

### Runtime Error: "error while loading shared libraries: onnxruntime.dll not found"
- **Cause**: DLL not in PATH or not in expected location.
- **Fix**: Ensure `asset/onnxruntime.dll` exists and Phase 10 packaging embeds it correctly.

### Runtime Error: "no such module: vec0"
- **Cause**: sqlite extension not loaded or failed to load.
- **Fix**: Ensure `asset/vec0.dll` exists and build used `-tags sqlite_extension`.

## Cross-Platform Support

Currently configured for **Windows x86-64 only** (Phase 10 will expand to macOS/Linux with platform-specific binaries).

| Platform | Status | Notes |
|----------|--------|-------|
| Windows x86-64 | ✓ In scope | Builds with MSVC or MinGW-w64 |
| macOS x86-64 | ⚠ Future | Requires ONNX Runtime SDK for macOS |
| macOS ARM64 | ⚠ Future | Requires M1/M2-native ONNX Runtime |
| Linux x86-64 | ⚠ Future | Requires ONNX Runtime SDK for Linux |
