# Cross-Platform Version-Matched Asset Delivery

## Overview

Decouples heavy ML/DB-extension binaries from the Git source repository. Instead of bundling static binaries or managing complex API discovery, the system implements a rigid **version-matched string-interpolation protocol** for runtime asset retrieval.

---

## 1. Semantic Version Mapping

**Concept:** 1:1 parity between the compiled binary's internal version string and the target GitHub Release tag (e.g., `v1.0.0`).

**Implementation:** A mutable package-level variable in `internal/runtime`:

```go
var AppVersion = "v0.0.0-dev"
```

**Production injection via ldflags:**

```bash
wails build -ldflags "-X ai-tutor/internal/runtime.AppVersion=v1.0.0"
```

> **⚠️ CRITICAL:** Go module is `ai-tutor`, NOT `github.com/Vishnuj-n/ai-tutor`. Using the full GitHub path as the ldflags key silently fails — no error, `AppVersion` stays `v0.0.0-dev`. Verified in `go.mod`.

---

## 2. Platform-Aware Asset Mapping

The Go backend evaluates `runtime.GOOS` at boot to select the correct asset envelope:

| Platform | Asset Bundle | Contents |
|----------|-------------|----------|
| Windows | `asset_windows.zip` | `onnxruntime.dll`, `vec0.dll`, `tokenizer.json`, `model_int8.onnx` |
| macOS | `asset_mac.zip` | Matching dylibs/frameworks (future) |
| Linux | `asset_linux.zip` | Matching `.so` binaries (future) |

```go
targetArchive := fmt.Sprintf("asset_%s.zip", runtime.GOOS)
```

---

## 3. Runtime URL Assembly

At boot, when local assets are missing, the app builds a download URL via string interpolation — no external API calls:

```
URL = BaseReleaseURL + "/" + AppVersion + "/" + GetPlatformAssetFilename()
```

**Constants (hardcoded fallbacks in `internal/runtime/boot.go`):**

| Constant | Value |
|----------|-------|
| `BaseReleaseURL` | `https://github.com/Vishnuj-n/ai-tutor/releases/download` |
| `AppVersion` | `v0.0.0-dev` (overridden via ldflags in production) |

Confirmed: account handle is **`Vishnuj-n`** (capital V). GitHub's router handles case mismatches via redirect, but uniform casing avoids issues with strict HTTP clients.

---

## 4. AcquireAssets — Remote Fallback Flow

The current `AcquireAssets()` only copies from local `./asset/`. The resolution is to **retain the local copy as a dev bypass**, then append a structured remote fallback:

```
CheckAssets() fails?
  ├── ./asset/ exists? → copy locally (dev shortcut)
  └── ./asset/ missing? → HTTP GET to version-interpolated GitHub URL
                          → stream download with progress
                          → extract zip
                          → verify against manifest.json inside zip
                          → stage DLLs
```

**Network:** Standard `net/http` client. **Zip extraction:** `archive/zip` — pure Go, zero CGO dependencies, safe for Windows builds.

**Boot order concern:** The download must happen before `EnsureAssetsReady()` returns success, because `boot.go` calls `StageDLLs()` and `db.Init(vec0)` immediately after. The current flow expects assets to exist before DB init. The download needs to be interleaved into `EnsureAssetsReady`'s failure path.

---

## 5. SHA-256 Verification — Decoupled from Source

**Problem:** Hardcoded hashes in Go source (`asset_manager.go:48-53`) brick validation on version bumps.

**Resolution:** Each platform zip bundles a `manifest.json` containing the canonical hashes for that release:

```
asset_windows.zip
  ├── manifest.json       ← { fileHashes: {...}, assetVersion: "..." }
  ├── tokenizer.json
  ├── model_int8.onnx
  ├── onnxruntime.dll
  └── vec0.dll
```

Flow:
1. Download zip
2. Extract and read `manifest.json` (without full extraction)
3. Extract remaining files
4. Verify each file's SHA-256 against the manifest
5. Staged DLLs get validated too

This eliminates source code changes per release. Only the zip contents change.

---

## 6. Environment State — Production Fallback

`boot.go:41` uses `_ = godotenv.Load()` (silent on missing .env). Resolution:

```go
_ = godotenv.Load()
if os.Getenv("APP_ENV") == "" {
    os.Setenv("APP_ENV", "production")
}
```

This ensures predictable routing: empty → production. No crash path. The storage manager already routes to `%LOCALAPPDATA%/ai-tutor/` in production mode.

---

## 7. Frontend Sandbox Concealment

Already using `import.meta.env.DEV` for developer sandbox panels. Verified:
- `npm run dev` → `DEV = true` → sandbox visible
- `wails build` → Vite runs `NODE_ENV=production` → dead-code elimination removes sandbox

No changes needed.

---

## 8. Upgrade Protection

When version `v2.0.0` ships with updated binaries:
1. Upload `asset_windows.zip` to GitHub Release `v2.0.0`
2. Build binary with `-ldflags "-X ai-tutor/internal/runtime.AppVersion=v2.0.0"`
3. Binary automatically resolves to the `v2.0.0` asset stream
4. Manifest inside zip carries v2.0.0 hashes
5. No legacy deprecation logic needed

---

## 9. Build Integration

**MVP approach:** Manual tag coordination between binary build and release artifact upload.

**Future CI/CD:** `.github/workflows/release.yml` will inject the tag into both:
- `-ldflags` for the Go binary
- Release asset generation pipeline

---

## Decision Record: Loose Files vs Platform-Zip

| Factor | Loose Files | Platform-Zip (✓) |
|--------|-------------|-------------------|
| Network hops | N per file | 1 |
| Partial download risk | Fragmented state | All-or-nothing (discard on error) |
| Release clutter | Many raw binaries per platform | 1 archive per platform |
| Platform lookup | Complex switch-case | `fmt.Sprintf("asset_%s.zip", runtime.GOOS)` |
| CGO safety | N/A | `archive/zip` = pure Go |

**Chosen: Platform-Zip.** Single atomic download, clean release surface, minimal Go logic.

---

## Unsolved Concerns

1. **Boot order coupling** — Download must complete before DB init. `EnsureAssetsReady` needs a download-trigger path.
2. **Windows file locking** — If DLLs are loaded during bootstrap, extracted files may be locked. Extraction must precede `db.Init()` and `NewOnnxEmbedder()`.
3. **No CI/CD yet** — Manual ldflags coordination is error-prone for the first few releases.

---

*Based on architectural review — see `internal/runtime/asset_manager.go` and `internal/runtime/boot.go` for current implementation.*
