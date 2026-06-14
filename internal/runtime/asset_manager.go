package runtime

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ai-tutor/internal/utils"
)

// AppVersion is the application version string, injected at build time via ldflags:
//
//	wails build -ldflags "-X ai-tutor/internal/runtime.AppVersion=v1.0.0"
//
// CRITICAL: the module name is "ai-tutor" (see go.mod), NOT the full GitHub path.
// Using the full GitHub path as the ldflags key silently fails — AppVersion stays "v0.0.0-dev".
var AppVersion = "v0.0.0-dev"

// BaseReleaseURL is the GitHub Releases download base URL for this repository.
const BaseReleaseURL = "https://github.com/Vishnuj-n/ai-tutor/releases/download"

// AssetManifest represents the metadata schema of the asset bundle.
type AssetManifest struct {
	AssetVersion  string            `json:"assetVersion"`
	DownloadURL   string            `json:"downloadUrl"`
	SHA256        string            `json:"sha256"` // Hash of the zip file itself (informational)
	SizeBytes     int64             `json:"sizeBytes"`
	RequiredFiles []string          `json:"requiredFiles"`
	FileHashes    map[string]string `json:"fileHashes"` // File name -> SHA-256 hash (uppercase)
}

// AssetManager coordinates checking, copying, and validating the local ONNX/sqlite-vec assets.
type AssetManager struct {
	targetDir      string
	sourceAssetDir string // where the project ships assets locally (e.g., "./asset")
	manifest       AssetManifest
	ctx            context.Context
}

// GetPlatformAssetFilename returns the zip filename for the current OS and version.
// For v1.0.0 (first release), the archive was published as "rag-assets.zip".
// Subsequent releases follow the naming convention "asset_<goos>.zip".
func GetPlatformAssetFilename(version string) string {
	// Normalize version: strip leading "v"
	normalized := strings.TrimPrefix(version, "v")
	if normalized == "1.0.0" || version == "v0.0.0-dev" {
		// Legacy release name (also used for dev/local fallback)
		return "rag-assets.zip"
	}
	return fmt.Sprintf("asset_%s.zip", runtime.GOOS)
}

// GetPlatformRequiredFiles returns the list of required asset files for the current OS.
// On Windows: .dll files. On macOS: .dylib files. On Linux: .so files.
func GetPlatformRequiredFiles() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"tokenizer.json",
			"model_int8.onnx",
			"onnxruntime.dll",
			"vec0.dll",
		}
	case "darwin":
		return []string{
			"tokenizer.json",
			"model_int8.onnx",
			"libonnxruntime.dylib",
			"vec0.dylib",
		}
	default: // linux and others
		return []string{
			"tokenizer.json",
			"model_int8.onnx",
			"libonnxruntime.so",
			"vec0.so",
		}
	}
}

// getPlatformOnnxLibName returns the OS-specific ONNX runtime library filename.
func getPlatformOnnxLibName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	default:
		return "libonnxruntime.so"
	}
}

// getPlatformVecLibName returns the OS-specific sqlite-vec extension filename.
func getPlatformVecLibName() string {
	switch runtime.GOOS {
	case "windows":
		return "vec0.dll"
	case "darwin":
		return "vec0.dylib"
	default:
		return "vec0.so"
	}
}

// IsVersionCompatible returns true when assetVersion (from manifest) is compatible with appVersion.
// A dev version ("v0.0.0-dev") is always compatible (local dev bypass).
func IsVersionCompatible(appVer, manifestVer string) bool {
	if appVer == "v0.0.0-dev" || appVer == "0.0.0-dev" {
		return true
	}
	// Normalize both: strip leading "v"
	return strings.TrimPrefix(appVer, "v") == strings.TrimPrefix(manifestVer, "v")
}

// BuildDownloadURL constructs the GitHub Release asset download URL.
func BuildDownloadURL(version string) string {
	filename := GetPlatformAssetFilename(version)
	// For dev version, point to v1.0.0 so we always have a valid URL.
	releaseTag := version
	if releaseTag == "v0.0.0-dev" {
		releaseTag = "v1.0.0"
	}
	return fmt.Sprintf("%s/%s/%s", BaseReleaseURL, releaseTag, filename)
}

// NewAssetManager creates a new AssetManager.
func NewAssetManager(ctx context.Context) (*AssetManager, error) {
	targetDir, err := ResolveAssetDir()
	if err != nil {
		return nil, err
	}

	return &AssetManager{
		targetDir:      targetDir,
		sourceAssetDir: "asset",
		ctx:            ctx,
	}, nil
}

// ResolveAssetDir resolves the user cache directory where assets should reside.
func ResolveAssetDir() (string, error) {
	// 1. Try LOCALAPPDATA environment variable on Windows
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		return filepath.Join(localAppData, "ai-tutor", "assets"), nil
	}

	// 2. Fallback to standard UserCacheDir
	cacheDir, err := os.UserCacheDir()
	if err == nil {
		return filepath.Join(cacheDir, "ai-tutor", "assets"), nil
	}

	// 3. Fallback to UserHomeDir
	homeDir, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(homeDir, ".ai-tutor", "assets"), nil
	}

	// 4. Default fallback to current working directory
	return "./assets", nil
}

// CheckAssets checks if the manifest exists and verifies all files and hashes.
// Hashes are read from the persisted manifest.json (not hardcoded in source).
func (am *AssetManager) CheckAssets() error {
	manifestPath := filepath.Join(am.targetDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest file not found")
	}

	// Read persisted manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var m AssetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Version compatibility check
	if !IsVersionCompatible(AppVersion, m.AssetVersion) {
		return fmt.Errorf("asset version mismatch: binary is %s but installed assets are %s",
			AppVersion, m.AssetVersion)
	}

	// Use the required files from the manifest (platform-specific)
	requiredFiles := m.RequiredFiles
	if len(requiredFiles) == 0 {
		// Fallback to platform defaults if manifest lacks this field
		requiredFiles = GetPlatformRequiredFiles()
	}

	// Verify all required files are present and match hashes
	for _, file := range requiredFiles {
		filePath := filepath.Join(am.targetDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("required asset file %s is missing", file)
		}

		expectedHash, ok := m.FileHashes[file]
		if !ok {
			return fmt.Errorf("manifest has no hash for required file %s", file)
		}
		expectedHash = strings.ToUpper(expectedHash)

		actualHash, err := computeSHA256(filePath)
		if err != nil {
			return fmt.Errorf("failed to compute hash for %s: %w", file, err)
		}

		if actualHash != expectedHash {
			return fmt.Errorf("integrity check failed for %s: expected %s, got %s", file, expectedHash, actualHash)
		}
	}

	return nil
}

// EnsureAssetsReady checks if assets are ready, returns nil if yes.
func (am *AssetManager) EnsureAssetsReady() error {
	return am.CheckAssets()
}

// ModelPath returns the path to model_int8.onnx in the target asset directory.
func (am *AssetManager) ModelPath() string {
	return filepath.Join(am.targetDir, "model_int8.onnx")
}

// TokenizerPath returns the path to tokenizer.json in the target asset directory.
func (am *AssetManager) TokenizerPath() string {
	return filepath.Join(am.targetDir, "tokenizer.json")
}

// OnnxRuntimePath returns the path to the staged ONNX runtime library in target assets.
// The filename is OS-specific: .dll on Windows, .dylib on macOS, .so on Linux.
func (am *AssetManager) OnnxRuntimePath() string {
	return filepath.Join(am.targetDir, "runtime", getPlatformOnnxLibName())
}

// Vec0LibPath returns the path to the staged sqlite-vec extension in target assets.
// The filename is OS-specific: .dll on Windows, .dylib on macOS, .so on Linux.
func (am *AssetManager) Vec0LibPath() string {
	return filepath.Join(am.targetDir, "runtime", getPlatformVecLibName())
}

// Vec0DllPath is an alias for Vec0LibPath for backward compatibility.
func (am *AssetManager) Vec0DllPath() string {
	return am.Vec0LibPath()
}

// StageDLLs copies the native libraries to a subfolder "runtime/" so that Wails/sqlite-vec can load them cleanly.
// On Windows these are .dll, on macOS .dylib, on Linux .so.
func (am *AssetManager) StageDLLs() (map[string]string, error) {
	runtimeDir := filepath.Join(am.targetDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runtime library directory: %w", err)
	}

	libs := []string{getPlatformOnnxLibName(), getPlatformVecLibName()}
	out := make(map[string]string, len(libs))

	for _, name := range libs {
		src := filepath.Join(am.targetDir, name)
		dst := filepath.Join(runtimeDir, name)

		if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("failed to stage library %s: %w", name, err)
		}

		absDst, err := filepath.Abs(dst)
		if err != nil {
			absDst = dst
		}
		out[name] = absDst
	}

	return out, nil
}

// hasLocalSourceAssets checks if all required assets for the current platform exist locally
// in the source directory or project root.
func (am *AssetManager) hasLocalSourceAssets() bool {
	for _, file := range GetPlatformRequiredFiles() {
		src := filepath.Join(am.sourceAssetDir, file)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			alt := filepath.Join(".", file)
			if _, err := os.Stat(alt); os.IsNotExist(err) {
				return false
			}
		}
	}
	return true
}

// AcquireAssets checks for local source assets and copies them, or falls back to downloading from GitHub.
// Progress is reported via the callback (may be nil to disable).
//
// For the local dev path: files are copied, hashes computed, and a manifest.json is written
// so subsequent CheckAssets() calls succeed without hardcoded source hashes.
//
// For the remote path: the download URL is constructed dynamically using AppVersion and the
// platform-specific zip filename. manifest.json inside the zip carries the canonical hashes.
func (am *AssetManager) AcquireAssets(progressCallback func(status string, percent int, msg, detail string)) error {
	if progressCallback == nil {
		progressCallback = func(status string, percent int, msg, detail string) {}
	}

	// Create target directories
	if err := os.MkdirAll(am.targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target asset directory: %w", err)
	}

	progressCallback("checking", 5, "Checking system compatibility...", "Checking OS and CPU specs")
	time.Sleep(300 * time.Millisecond)

	// Warn on non-primary platforms
	if runtime.GOOS != "windows" {
		utils.Warnf("[AssetManager] Non-Windows platform detected (%s). RAG assets for macOS/Linux are planned but not yet released.", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" {
		utils.Warnf("[AssetManager] Compatibility warning: native libraries are compiled for amd64, running on %s", runtime.GOARCH)
	}

	requiredFiles := GetPlatformRequiredFiles()

	if am.hasLocalSourceAssets() {
		utils.Infof("[AssetManager] Local source assets detected. Using fast dev copy bypass.")
		// Setup copy ranges (sizes informational — used for UI only)
		type fileInfo struct {
			name       string
			startPct   int
			endPct     int
			approxSize string
		}
		fileRanges := []fileInfo{
			{getPlatformVecLibName(), 10, 15, "289 KB"},
			{"tokenizer.json", 15, 20, "742 KB"},
			{getPlatformOnnxLibName(), 20, 35, "14.1 MB"},
			{"model_int8.onnx", 35, 85, "137.3 MB"},
		}

		fileHashes := make(map[string]string, len(requiredFiles))

		// Copy files with streaming blocks to show progress
		for _, fi := range fileRanges {
			srcPath := filepath.Join(am.sourceAssetDir, fi.name)
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				srcPath = filepath.Join(".", fi.name)
			}

			dstPath := filepath.Join(am.targetDir, fi.name)
			msg := fmt.Sprintf("Acquiring %s...", fi.name)
			detail := fmt.Sprintf("Copying %s", fi.approxSize)

			if err := copyFileWithProgress(srcPath, dstPath, fi.startPct, fi.endPct, msg, detail, progressCallback); err != nil {
				return fmt.Errorf("failed to acquire %s: %w", fi.name, err)
			}
		}

		// Compute hashes for all copied files to populate the manifest
		progressCallback("verifying", 88, "Computing file hashes...", "Building local manifest")
		for _, file := range requiredFiles {
			filePath := filepath.Join(am.targetDir, file)
			hash, err := computeSHA256(filePath)
			if err != nil {
				return fmt.Errorf("failed to hash %s: %w", file, err)
			}
			fileHashes[file] = hash
		}

		// Normalize AppVersion for the manifest
		manifestVersion := strings.TrimPrefix(AppVersion, "v")
		if manifestVersion == "0.0.0-dev" {
			manifestVersion = "0.0.0-dev"
		}

		localManifest := AssetManifest{
			AssetVersion:  manifestVersion,
			DownloadURL:   BuildDownloadURL(AppVersion),
			RequiredFiles: requiredFiles,
			FileHashes:    fileHashes,
		}
		am.manifest = localManifest

	} else {
		// Remote download fallback
		downloadURL := BuildDownloadURL(AppVersion)
		utils.Infof("[AssetManager] Local source assets not found. Initiating remote download from %s", downloadURL)
		progressCallback("acquiring", 10, "Connecting to asset repository...", fmt.Sprintf("Downloading from GitHub (%s)", AppVersion))

		req, err := http.NewRequestWithContext(am.ctx, "GET", downloadURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create download request: %w", err)
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download assets: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed with HTTP status: %s (URL: %s)", resp.Status, downloadURL)
		}

		totalSize := resp.ContentLength
		if totalSize <= 0 {
			// Fallback estimated size for progress display
			totalSize = 152_458_471
		}

		// Download to a temporary zip file inside target directory to avoid incomplete state
		tempZipPath := filepath.Join(am.targetDir, "rag-assets.tmp.zip")
		out, err := os.Create(tempZipPath)
		if err != nil {
			return fmt.Errorf("failed to create temporary zip file: %w", err)
		}

		// Clean up the temporary zip file when done
		defer func() {
			_ = out.Close()
			_ = os.Remove(tempZipPath)
		}()

		// Stream body into temp file
		buf := make([]byte, 64*1024)
		var written int64
		for {
			nr, er := resp.Body.Read(buf)
			if nr > 0 {
				nw, ew := out.Write(buf[0:nr])
				if nw < 0 || ew != nil {
					return ew
				}
				written += int64(nw)
				if totalSize > 0 {
					pct := 10 + int((float64(written)/float64(totalSize))*70) // 10% to 80%
					progressCallback("acquiring", pct, "Downloading RAG assets...", fmt.Sprintf("%.1f MB / %.1f MB", float64(written)/(1024*1024), float64(totalSize)/(1024*1024)))
				}
			}
			if er != nil {
				if er == io.EOF {
					break
				}
				return er
			}
		}
		_ = out.Close() // Close file handle before extracting

		// Extract using Go's archive/zip (pure Go, no CGO, safe on Windows)
		progressCallback("extracting", 80, "Extracting RAG assets...", "Decompressing zip archive")
		zipReader, err := zip.OpenReader(tempZipPath)
		if err != nil {
			return fmt.Errorf("failed to open zip file: %w", err)
		}
		defer func() { _ = zipReader.Close() }()

		// First pass: extract manifest.json from zip to get canonical hashes
		var remoteManifest *AssetManifest
		for _, f := range zipReader.File {
			if filepath.Base(f.Name) == "manifest.json" {
				src, openErr := f.Open()
				if openErr != nil {
					return fmt.Errorf("failed to open manifest.json in zip: %w", openErr)
				}
				data, readErr := io.ReadAll(src)
				_ = src.Close()
				if readErr != nil {
					return fmt.Errorf("failed to read manifest.json from zip: %w", readErr)
				}
				var m AssetManifest
				if parseErr := json.Unmarshal(data, &m); parseErr != nil {
					return fmt.Errorf("failed to parse manifest.json from zip: %w", parseErr)
				}
				remoteManifest = &m
				break
			}
		}

		if remoteManifest != nil {
			// Validate version compatibility
			if !IsVersionCompatible(AppVersion, remoteManifest.AssetVersion) {
				return fmt.Errorf("version mismatch: binary is %s but downloaded assets are version %s",
					AppVersion, remoteManifest.AssetVersion)
			}
			am.manifest = *remoteManifest
			utils.Infof("[AssetManager] Remote manifest loaded. Asset version: %s", remoteManifest.AssetVersion)
		} else {
			return fmt.Errorf("no manifest.json found in remote asset archive; verification cannot be performed")
		}

		// Second pass: extract all files
		for _, f := range zipReader.File {
			cleanPath := filepath.Clean(f.Name)
			// Zip-slip protection
			if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.Contains(cleanPath, `\..`) {
				return fmt.Errorf("invalid path in zip archive: %s", f.Name)
			}

			dstPath := filepath.Join(am.targetDir, f.Name)
			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(dstPath, 0o755)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", f.Name, err)
			}

			dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fmt.Errorf("failed to create target file %s: %w", f.Name, err)
			}

			srcFile, err := f.Open()
			if err != nil {
				_ = dstFile.Close()
				return fmt.Errorf("failed to open zip content for %s: %w", f.Name, err)
			}

			_, err = io.Copy(dstFile, srcFile)
			_ = dstFile.Close()
			_ = srcFile.Close()
			if err != nil {
				return fmt.Errorf("failed to write extracted file %s: %w", f.Name, err)
			}
		}
	}

	// Verify SHA-256 hashes against manifest
	progressCallback("verifying", 90, "Verifying asset integrity...", "Checking SHA-256 checksums")
	time.Sleep(400 * time.Millisecond)

	verifyFiles := am.manifest.RequiredFiles
	if len(verifyFiles) == 0 {
		verifyFiles = requiredFiles
	}

	for _, name := range verifyFiles {
		expectedHash, ok := am.manifest.FileHashes[name]
		if !ok {
			return fmt.Errorf("no hash in manifest for required file %s; verification cannot be performed", name)
		}
		expectedHash = strings.ToUpper(expectedHash)

		filePath := filepath.Join(am.targetDir, name)
		actualHash, err := computeSHA256(filePath)
		if err != nil {
			return fmt.Errorf("failed to verify hash for %s: %w", name, err)
		}

		if actualHash != expectedHash {
			return fmt.Errorf("SHA-256 hash mismatch for file %s. Expected %s, got %s", name, expectedHash, actualHash)
		}
	}

	// Stage native libraries
	progressCallback("extracting", 95, "Staging native libraries...", "Setting up local runtime cache")
	time.Sleep(300 * time.Millisecond)
	if _, err := am.StageDLLs(); err != nil {
		return fmt.Errorf("failed to stage native libraries during acquisition: %w", err)
	}

	// Save manifest file only on successful completion (atomic completion guard)
	manifestPath := filepath.Join(am.targetDir, "manifest.json")
	manifestData, err := json.MarshalIndent(am.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("failed to save manifest.json: %w", err)
	}

	progressCallback("initializing", 98, "Initializing local AI engine...", "Preloading ONNX embedder")
	time.Sleep(300 * time.Millisecond)

	return nil
}

// Helper to compute SHA-256 hash of a file.
func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))), nil
}

// Copy file with progress updates.
func copyFileWithProgress(src, dst string, startPct, endPct int, msg, detail string, cb func(string, int, string, string)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	stat, err := in.Stat()
	if err != nil {
		return err
	}
	totalSize := stat.Size()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	buf := make([]byte, 1024*1024) // 1MB buffer
	var written int64

	for {
		nr, er := in.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw < 0 || ew != nil {
				return ew
			}
			written += int64(nw)

			// Calculate progress percent
			pctRange := endPct - startPct
			currentPct := startPct + int((float64(written)/float64(totalSize))*float64(pctRange))
			cb("acquiring", currentPct, msg, fmt.Sprintf("%s (%d%%)", detail, int((float64(written)/float64(totalSize))*100)))

			// Sleep slightly to simulate network throttling/download appearance
			time.Sleep(15 * time.Millisecond)
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	return out.Sync()
}

// Copy file helper.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	_ = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	return out.Sync()
}
