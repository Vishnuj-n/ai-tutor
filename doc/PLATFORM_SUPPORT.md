# Platform Support

## Current Status: Windows-First

**Primary Target:** Windows 10/11 (x64)

Windows is the exclusive build target for the MVP phase. This constraint eliminates cross-platform native library complexity while the core RAG pipeline and queue architecture stabilize.

### Windows-Specific Dependencies

| Component | File | Purpose |
|-----------|------|---------|
| ONNX Runtime | `onnxruntime.dll` | Local embedding inference |
| Vector Storage | `vec0.dll` | SQLite vector search extension |
| Build Scripts | `sync-deps.sh`, `windows-sync-deps.ps1` | Dependency management |

### Build Requirements

- Go 1.21+ with CGO enabled (MSYS2/MinGW on Windows)
- MSVC or MinGW toolchain
- PowerShell for dependency sync scripts

---

## Future Platforms

### macOS (Intel/Apple Silicon)

**Required Changes:**
- Replace `onnxruntime.dll` with `libonnxruntime.dylib`
- Compile `vec0.dylib` for Darwin
- Update `app.go` app-data directory handling for macOS paths
- Add macOS-specific build constraints

### Linux (x64/ARM64)

**Required Changes:**
- Replace `onnxruntime.dll` with `libonnxruntime.so`
- Compile `vec0.so` for target architecture
- Validate CGO build requirements across distributions
- Handle Linux-specific path conventions

---

## Rationale

Single-platform focus during MVP enables:

1. **Deterministic Testing:** ONNX-to-SQLite pipeline stabilizes without OS-specific memory/driver variables
2. **Simplified Asset Management:** Single `asset/` folder with Windows-only binaries
3. **Faster Iteration:** No conditional compilation paths or abstraction layers required

---

## Implementation Notes

Platform-specific code should use Go build constraints:

```go
//go:build windows
// +build windows

package embeddings
```

Remove half-finished `runtime.GOOS` switches. Platform support is either implemented or documented as future work—no intermediate states.
