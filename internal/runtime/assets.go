package runtime

import (
	"fmt"
	"os"
	"path/filepath"
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
		{"onnxruntime.dll", true, false},
		{"vec0.dll", true, false},
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
	return av.GetAssetPath("onnxruntime.dll")
}

// Vec0DllPath returns the full path to vec0.dll.
func (av *AssetValidator) Vec0DllPath() string {
	return av.GetAssetPath("vec0.dll")
}
