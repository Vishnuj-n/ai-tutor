package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	onnxRuntimeName, vec0Name := runtimeLibraryNames()
	required := []struct {
		name     string
		isFile   bool
		optional bool
	}{
		{"tokenizer.json", true, false},
		{"model_int8.onnx", true, false},
		{onnxRuntimeName, true, false},
		{vec0Name, true, false},
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
	onnxRuntimeName, _ := runtimeLibraryNames()
	return av.GetAssetPath(onnxRuntimeName)
}

// Vec0DllPath returns the full path to vec0.dll.
func (av *AssetValidator) Vec0DllPath() string {
	_, vec0Name := runtimeLibraryNames()
	return av.GetAssetPath(vec0Name)
}

// PrepareRuntimeAssets copies runtime DLLs to an app-data subdirectory and returns absolute paths.
// This avoids reliance on the process working directory when loading native dependencies.
func (av *AssetValidator) PrepareRuntimeAssets(appDir string) (map[string]string, error) {
	runtimeDir := filepath.Join(appDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runtime directory: %w", err)
	}

	onnxRuntimeName, vec0Name := runtimeLibraryNames()
	assets := []string{onnxRuntimeName, vec0Name}
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
		if srcInfo.Size() == dstInfo.Size() && srcInfo.ModTime().Equal(dstInfo.ModTime()) {
			return nil
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

func runtimeLibraryNames() (onnxRuntimeName string, vec0Name string) {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll", "vec0.dll"
	case "darwin":
		return "libonnxruntime.dylib", "vec0.dylib"
	default:
		return "libonnxruntime.so", "vec0.so"
	}
}
