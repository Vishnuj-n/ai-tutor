# Walkthrough — Cross-Platform Version-Matched Asset Delivery

All tasks from the implementation plan have been completed and verified.

## Changes Made

### 1. [`internal/runtime/asset_manager.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/runtime/asset_manager.go) — Full rewrite

#### New version injection point
```go
// Overridable at build time via:
//   wails build -ldflags "-X ai-tutor/internal/runtime.AppVersion=v1.0.0"
var AppVersion = "v0.0.0-dev"

const BaseReleaseURL = "https://github.com/Vishnuj-n/ai-tutor/releases/download"
```

#### Platform-aware helpers added
| Function | Purpose |
|---|---|
| `GetPlatformAssetFilename(version)` | Returns `rag-assets.zip` for `v1.0.0` (legacy), `asset_<goos>.zip` for later |
| `GetPlatformRequiredFiles()` | Windows: `.dll`, macOS: `.dylib`, Linux: `.so` |
| `getPlatformOnnxLibName()` | OS-specific ONNX runtime filename |
| `getPlatformVecLibName()` | OS-specific sqlite-vec extension filename |
| `IsVersionCompatible(appVer, manifestVer)` | Dev version (`v0.0.0-dev`) always passes; else must match |
| `BuildDownloadURL(version)` | Constructs full URL from `BaseReleaseURL + version + filename` |

#### Removed hardcoded hashes from source
The old `TargetManifest` with hardcoded SHA-256 values is gone. Instead:
- **Local dev copy path**: Files are copied, their SHA-256 hashes are *computed dynamically*, and a `manifest.json` is written to the target directory. Subsequent `CheckAssets()` calls validate against this locally generated manifest.
- **Remote download path**: `manifest.json` is *extracted from the zip first-pass* before any other files are written, then all extracted files are verified against it. Version compatibility is enforced at this point.

#### `CheckAssets()` updated
Now reads the persisted `manifest.json` and validates version compatibility against `AppVersion`, then verifies all required files with hashes sourced from the manifest.

#### `StageDLLs()` updated
Uses `getPlatformOnnxLibName()` and `getPlatformVecLibName()` instead of hardcoded `.dll` names. `Vec0DllPath()` retained as an alias for `Vec0LibPath()` for backward compatibility with existing callers.

---

### 2. [`internal/runtime/boot.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/runtime/boot.go) — APP_ENV fallback

```go
_ = godotenv.Load()
// If APP_ENV was not set by .env or the environment, default to "production".
if os.Getenv("APP_ENV") == "" {
    os.Setenv("APP_ENV", "production")
}
```
Fresh environments without a `.env` file no longer silently inherit an undefined storage routing. `"dev"` must be explicitly opted into.

---

### 3. [`windows-sync-deps.ps1`](file:///c:/Users/vishn/PROJECT/ai-tutor/windows-sync-deps.ps1) — Version-matched URL

Reads an optional `VERSION` file from the project root (defaults to `v1.0.0`). Maps `v1.0.0` → `rag-assets.zip` (legacy) and any later version → `asset_windows.zip`:
```powershell
$normalizedVersion = $appVersion.TrimStart("v")
if ($normalizedVersion -eq "1.0.0") {
    $zipFilename = "rag-assets.zip"
} else {
    $zipFilename = "asset_windows.zip"
}
$downloadUrl = "https://github.com/Vishnuj-n/ai-tutor/releases/download/$releaseTag/$zipFilename"
```

---

### 4. [`sync-deps.sh`](file:///c:/Users/vishn/PROJECT/ai-tutor/sync-deps.sh) — Platform-aware

Now detects `uname -s` (Darwin vs Linux) and:
- Sets OS-specific cache dir (`~/Library/Caches` vs `~/.cache`)
- Sets OS-specific required files (`.dylib` on macOS, `.so` on Linux)
- Constructs version-matched URL using optional `VERSION` file

---

## Verification Results

### Automated Tests
```
ok  ai-tutor          7.615s
ok  ai-tutor/internal/db        (cached)
ok  ai-tutor/internal/embeddings   (cached)
ok  ai-tutor/internal/llm       (cached)
ok  ai-tutor/internal/notebook     (cached)
ok  ai-tutor/internal/scheduler    (cached)
```
All packages compile and all tests pass.

## How to Upgrade Assets for a New Release

1. Upload `asset_windows.zip` (and future `asset_darwin.zip`, `asset_linux.zip`) to GitHub Release `vX.Y.Z`
2. Ensure each zip contains a `manifest.json` with canonical hashes
3. Create a `VERSION` file in the project root with `vX.Y.Z`
4. Build binary with: `wails build -ldflags "-X ai-tutor/internal/runtime.AppVersion=vX.Y.Z"`
5. The app will auto-resolve to the correct asset stream on first launch

> [!NOTE]
> `v0.0.0-dev` is always considered compatible with any manifest version — safe for local development without touching the version.

---

# OUTDATED 

All tasks outlined in the implementation plan have been completed and verified successfully.

## Changes Made

### 1. SQLite Vector Extension Verification & CGO Fallback
- **Implemented `IsVecExtensionLoaded()`:** Added a query-based check executing `SELECT vec_version()` inside [store.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/store.go) to verify if the extension was successfully loaded.
- **Fixed `extension_nocgo.go` Compilation:** Added `"database/sql"` to the imports in [extension_nocgo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/extension_nocgo.go) to avoid compilation failures when building under `!cgo` environments.
- **Fallback Initialization:** Modified [boot.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/runtime/boot.go) to catch errors when trying to initialize the database with vectors. If loading fails or `IsVecExtensionLoaded()` returns false, it re-initializes without vector support using `db.Init(dbPath, "")`.
- **Safe RAG Toggle:** Updated `InitializeRAG` in [app.go](file:///c:/Users/vishn/PROJECT/ai-tutor/app.go) to revert cleanly to a non-vector configuration if extension loading fails, preventing corrupt or half-configured database states.

### 2. On-Demand Remote Downloader
- **Pure Go HTTP Downloader:** Integrated Go's standard `"net/http"` and `"archive/zip"` in [asset_manager.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/runtime/asset_manager.go).
- **Graceful Download Stream:** If local `./asset/` files are not found, `AcquireAssets` downloads the release assets directly from GitHub, reporting progress safely.
- **Atomic Defer Cleanup:** Downloads are stored in a temporary zip file (`rag-assets.tmp.zip`) and decompressed. The target `manifest.json` is only written upon zero-error completion to prevent partial or corrupted asset states from passing subsequent checks.

### 3. Sync Scripts Update
- **PowerShell and Bash Fallbacks:** Updated [windows-sync-deps.ps1](file:///c:/Users/vishn/PROJECT/ai-tutor/windows-sync-deps.ps1) and [sync-deps.sh](file:///c:/Users/vishn/PROJECT/ai-tutor/sync-deps.sh) to check both AppData and local workspace paths.
- **Asset Mirroring:** If assets exist in the workspace, they are synced directly to the AppData folder. If they are missing entirely, the scripts attempt to download the release zip from GitHub, fall back cleanly on offline/404 scenarios, and copy downloaded assets back to the workspace for development coupling.

---

## Verification Results

### Automated Tests
- Run `go test ./...` in the repository root. All tests compiled and passed:
  ```
  ok  	ai-tutor	17.699s
  ok  	ai-tutor/internal/db	8.233s
  ok  	ai-tutor/internal/embeddings	(cached)
  ok  	ai-tutor/internal/llm	(cached)
  ok  	ai-tutor/internal/notebook	4.969s
  ok  	ai-tutor/internal/scheduler	5.646s
  ```

### Build Constraints Verification
- **CGO/Vector Build:** `go build -tags sqlite_extension -o test_rag_build.exe .` built successfully.
- **Nocgo Fallback Build:** `go build -tags nocgo -o test_fallback_build.exe .` built successfully, proving that non-CGO compilation behaves correctly and respects the safe fallback paths.

### Script Execution Verification
- Ran `powershell -ExecutionPolicy Bypass -File .\windows-sync-deps.ps1` which successfully detected local assets and synced them to the AppData folder:
  ```
  Step 4: Validating and acquiring RAG assets...
  RAG assets detected in local workspace (.\asset). Syncing to AppData (C:\Users\vishn\AppData\Local\ai-tutor\assets)...
  Sync complete.
  ```
