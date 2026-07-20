//go:build !cgo

package embeddings

import "errors"

// OnnxEmbedder stub for builds with CGO disabled.
type OnnxEmbedder struct{}

// NewOnnxEmbedder returns an error indicating that ONNX runtime is not supported when CGO is disabled.
func NewOnnxEmbedder(modelPath, tokenizerPath, configuredRuntimePath string) (*OnnxEmbedder, error) {
	return nil, errors.New("onnx embedder is not supported in builds with CGO disabled")
}

// Embed returns an error indicating that ONNX runtime is not supported when CGO is disabled.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	return nil, errors.New("onnx embedder is not supported in builds with CGO disabled")
}

// GetDimension returns 0 when CGO is disabled.
func (e *OnnxEmbedder) GetDimension() int32 {
	return 0
}

// Close is a no-op when CGO is disabled.
func (e *OnnxEmbedder) Close() error {
	return nil
}
