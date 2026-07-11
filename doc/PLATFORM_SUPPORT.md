# Platform Support

## Windows-First (MVP)

**Primary:** Windows 10/11 (x64). Single platform during MVP to avoid cross-platform native library complexity. See [doc/SPRINT.md](./doc/SPRINT.md) for current platform scope.

### Windows Dependencies

| Component | File | Purpose |
|-----------|------|---------|
| ONNX Runtime | `onnxruntime.dll` | Local embedding inference ([doc/ARCHITECTURE.md §6.1](./doc/ARCHITECTURE.md)) |
| Vector Storage | `vec0.dll` | SQLite vector search ([doc/ARCHITECTURE.md §6.2](./doc/ARCHITECTURE.md)) |
| Build Scripts | `sync-deps.sh`, `windows-sync-deps.ps1` | Dependency management |

### Build Requirements

- Go 1.26 + CGO enabled (MSYS2/MinGW)
- MSVC or MinGW toolchain
- PowerShell for dependency scripts
- Full setup guide: [doc/DEVELOPER_ONBOARDING.md](./doc/DEVELOPER_ONBOARDING.md)

---

## Future Platforms

**macOS:** Replace `.dll` with `.dylib`, update app-data paths, add build constraints.
**Linux:** Replace `.dll` with `.so`, validate CGO across distros, handle Linux paths.

---

## Rationale

Single-platform MVP enables: deterministic testing, simpler asset management, faster iteration.

Use Go build constraints (`//go:build windows`) for platform code. No half-finished `runtime.GOOS` switches — either implemented or documented as future work.
