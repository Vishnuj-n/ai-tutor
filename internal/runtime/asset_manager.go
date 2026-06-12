package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ai-tutor/internal/utils"
)

// AssetManifest represents the metadata schema of the asset bundle.
type AssetManifest struct {
	AssetVersion  string            `json:"assetVersion"`
	DownloadURL   string            `json:"downloadUrl"`
	SHA256        string            `json:"sha256"` // Hash of hypothetical zip file
	SizeBytes     int64             `json:"sizeBytes"`
	RequiredFiles []string          `json:"requiredFiles"`
	FileHashes    map[string]string `json:"fileHashes"` // File name -> SHA-256 hash
}

// AssetManager coordinates checking, copying, and validating the local ONNX/sqlite-vec assets.
type AssetManager struct {
	targetDir      string
	sourceAssetDir string // where the project ships assets locally (e.g., "./asset")
	manifest       AssetManifest
	ctx            context.Context
}

// TargetManifest is the canonical v1.0.0 manifest for RAG assets.
var TargetManifest = AssetManifest{
	AssetVersion: "1.0.0",
	DownloadURL:  "https://github.com/Vishnuj-n/ai-tutor/releases/download/v1.0.0/rag-assets.zip",
	SizeBytes:    152458471,
	RequiredFiles: []string{
		"tokenizer.json",
		"model_int8.onnx",
		"onnxruntime.dll",
		"vec0.dll",
	},
	FileHashes: map[string]string{
		"model_int8.onnx": "B4342336DEBAEA79DE872370664B0AAEB67DEA4605513D00EE236EA871A81F27",
		"onnxruntime.dll": "8A1AAD8D59D02A5337D4E3F5BBD1158C3F7BF84FE3B3F0052F957DD3E75A91CB",
		"tokenizer.json":  "FFB28886478B9B17A8C06F4FE6741B970D5DD3DE13330CCC3B9F686DF8A0545A",
		"vec0.dll":        "FCF98662A7AD9DCE394B96A88F91032047823831B951C76636787C312A6476E6",
	},
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
		manifest:       TargetManifest,
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

	// Verify all required files are present and match hashes
	for _, file := range am.manifest.RequiredFiles {
		filePath := filepath.Join(am.targetDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("required asset file %s is missing", file)
		}

		expectedHash := strings.ToUpper(am.manifest.FileHashes[file])
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

// OnnxRuntimePath returns the path to staged onnxruntime.dll in target assets.
func (am *AssetManager) OnnxRuntimePath() string {
	return filepath.Join(am.targetDir, "runtime", "onnxruntime.dll")
}

// Vec0DllPath returns the path to staged vec0.dll in target assets.
func (am *AssetManager) Vec0DllPath() string {
	return filepath.Join(am.targetDir, "runtime", "vec0.dll")
}

// StageDLLs copies the DLL files to a subfolder "runtime/" so that Wails/sqlite-vec can load them cleanly.
func (am *AssetManager) StageDLLs() (map[string]string, error) {
	runtimeDir := filepath.Join(am.targetDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runtime DLL directory: %w", err)
	}

	dlls := []string{"onnxruntime.dll", "vec0.dll"}
	out := make(map[string]string, len(dlls))

	for _, name := range dlls {
		src := filepath.Join(am.targetDir, name)
		dst := filepath.Join(runtimeDir, name)

		if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("failed to stage DLL %s: %w", name, err)
		}

		absDst, err := filepath.Abs(dst)
		if err != nil {
			absDst = dst
		}
		out[name] = absDst
	}

	return out, nil
}

// AcquireAssets simulates the downloading process by copy-streaming from am.sourceAssetDir to am.targetDir.
// It reports progress via the callback.
func (am *AssetManager) AcquireAssets(progressCallback func(status string, percent int, msg, detail string)) error {
	// Create target directories
	if err := os.MkdirAll(am.targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target asset directory: %w", err)
	}

	progressCallback("checking", 5, "Checking system compatibility...", "Checking OS and CPU specs")
	time.Sleep(300 * time.Millisecond)

	// Verify CPU and OS specs for DLL loading
	if runtime.GOOS != "windows" {
		utils.Warnf("[AssetManager] Compatibility warning: RAG is designed for Windows, running on %s", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" {
		utils.Warnf("[AssetManager] Compatibility warning: vec0.dll is compiled for x64, running on %s", runtime.GOARCH)
	}

	// Setup download simulation ranges
	fileRanges := []struct {
		name       string
		startPct   int
		endPct     int
		approxSize string
	}{
		{"vec0.dll", 10, 15, "289 KB"},
		{"tokenizer.json", 15, 20, "742 KB"},
		{"onnxruntime.dll", 20, 35, "14.1 MB"},
		{"model_int8.onnx", 35, 85, "137.3 MB"},
	}

	// Copy files with streaming blocks to show progress
	for _, fileInfo := range fileRanges {
		srcPath := filepath.Join(am.sourceAssetDir, fileInfo.name)
		// Fallback check: look beside executable if not in source project structure
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			srcPath = filepath.Join(".", fileInfo.name)
		}

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			return fmt.Errorf("source asset %s not found in local workspace. Please place it in ./asset/", fileInfo.name)
		}

		dstPath := filepath.Join(am.targetDir, fileInfo.name)
		msg := fmt.Sprintf("Acquiring %s...", fileInfo.name)
		detail := fmt.Sprintf("Downloading %s", fileInfo.approxSize)

		if err := copyFileWithProgress(srcPath, dstPath, fileInfo.startPct, fileInfo.endPct, msg, detail, progressCallback); err != nil {
			return fmt.Errorf("failed to acquire %s: %w", fileInfo.name, err)
		}
	}

	// Verify SHA-256 hashes
	progressCallback("verifying", 90, "Verifying asset integrity...", "Checking SHA-256 checksums")
	time.Sleep(400 * time.Millisecond)

	for _, name := range am.manifest.RequiredFiles {
		filePath := filepath.Join(am.targetDir, name)
		expectedHash := strings.ToUpper(am.manifest.FileHashes[name])
		actualHash, err := computeSHA256(filePath)
		if err != nil {
			return fmt.Errorf("failed to verify hash for %s: %w", name, err)
		}

		if actualHash != expectedHash {
			return fmt.Errorf("SHA-256 hash mismatch for file %s. Expected %s, got %s", name, expectedHash, actualHash)
		}
	}

	// Stage DLLs
	progressCallback("extracting", 95, "Extracting and staging DLLs...", "Setting up local runtime cache")
	time.Sleep(300 * time.Millisecond)
	if _, err := am.StageDLLs(); err != nil {
		return fmt.Errorf("failed to stage DLLs during acquisition: %w", err)
	}

	// Save manifest file
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
	defer f.Close()

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
	defer in.Close()

	stat, err := in.Stat()
	if err != nil {
		return err
	}
	totalSize := stat.Size()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

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
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	_ = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	return out.Sync()
}
