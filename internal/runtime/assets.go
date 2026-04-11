package runtime

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	onnxRuntimeDLL = "onnxruntime.dll"
	vec0DLL        = "vec0.dll"
)

// AssetValidator checks for required runtime assets (models, tokenizers, extensions).
type AssetValidator struct {
	assetDir string
}

// NewAssetValidator creates a new asset validator.
func NewAssetValidator(assetDir string) *AssetValidator {
	return &AssetValidator{
		assetDir: assetDir,
	}
}

// ValidateAll checks that all required assets exist and are readable.
func (av *AssetValidator) ValidateAll() error {
	required := []struct {
		name     string
		isFile   bool
		optional bool
	}{
		{"tokenizer.json", true, false},
		{"model_int8.onnx", true, false},
		{onnxRuntimeDLL, true, false},
		{vec0DLL, true, false},
	}

	missing := []string{}
	for _, asset := range required {
		path := filepath.Join(av.assetDir, asset.name)
		if _, err := os.Stat(path); err != nil {
			if !asset.optional {
				missing = append(missing, fmt.Sprintf("%s (%s)", asset.name, path))
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required assets: %v", missing)
	}

	return nil
}

// GetAssetPath returns the full path to an asset by name.
func (av *AssetValidator) GetAssetPath(name string) string {
	return filepath.Join(av.assetDir, name)
}

// TokenizerPath returns the full path to tokenizer.json.
func (av *AssetValidator) TokenizerPath() string {
	return av.GetAssetPath("tokenizer.json")
}

// ModelPath returns the full path to model_int8.onnx.
func (av *AssetValidator) ModelPath() string {
	return av.GetAssetPath("model_int8.onnx")
}

// OnnxRuntimePath returns the full path to onnxruntime.dll.
func (av *AssetValidator) OnnxRuntimePath() string {
	return av.GetAssetPath(onnxRuntimeDLL)
}

// Vec0DllPath returns the full path to vec0.dll.
func (av *AssetValidator) Vec0DllPath() string {
	return av.GetAssetPath(vec0DLL)
}

// PrepareRuntimeAssets copies runtime DLLs to an app-data subdirectory and returns absolute paths.
// This avoids reliance on the process working directory when loading native dependencies.
func (av *AssetValidator) PrepareRuntimeAssets(appDir string) (map[string]string, error) {
	runtimeDir := filepath.Join(appDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runtime directory: %w", err)
	}

	assets := []string{onnxRuntimeDLL, vec0DLL}
	out := make(map[string]string, len(assets))

	for _, name := range assets {
		src := av.GetAssetPath(name)
		dst := filepath.Join(runtimeDir, name)
		if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("failed to stage %s: %w", name, err)
		}

		absDst, err := filepath.Abs(dst)
		if err != nil {
			absDst = dst
		}
		out[name] = absDst
	}

	return out, nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if dstInfo, dstErr := os.Stat(dst); dstErr == nil {
		if srcInfo.Size() == dstInfo.Size() {
			srcHash, srcHashErr := fileSHA256(src)
			if srcHashErr != nil {
				return srcHashErr
			}

			dstHash, dstHashErr := fileSHA256(dst)
			if dstHashErr != nil {
				return dstHashErr
			}

			if srcHash == dstHash {
				return nil
			}
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return err
	}

	return out.Sync()
}

func fileSHA256(path string) ([32]byte, error) {
	var digest [32]byte

	file, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return digest, err
	}

	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}
