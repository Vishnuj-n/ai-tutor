# Platform Support

## Windows-First (MVP)

**Primary:** Windows 10/11 (x64). Single platform during MVP to avoid cross-platform native library complexity.

### Windows Dependencies

| Component | File | Purpose |
|-----------|------|---------|
| ONNX Runtime | `onnxruntime.dll` | Local embedding inference |
| Vector Storage | `vec0.dll` | SQLite vector search |
| Build Scripts | `sync-deps.sh`, `windows-sync-deps.ps1` | Dependency management |

### Build Requirements

- Go 1.26 + CGO enabled (MSYS2/MinGW)
- MSVC or MinGW toolchain
- PowerShell for dependency scripts

---

## Future Platforms

**macOS:** Replace `.dll` with `.dylib`, update app-data paths, add build constraints.
**Linux:** Replace `.dll` with `.so`, validate CGO across distros, handle Linux paths.

---

## Rationale

Single-platform MVP enables: deterministic testing, simpler asset management, faster iteration.

Use Go build constraints (`//go:build windows`) for platform code. No half-finished `runtime.GOOS` switches — either implemented or documented as future work.
