//go:build !tokenizers

package embeddings

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// OnnxEmbedder handles embedding generation using ONNX Runtime.
// This default build does not require CGO tokenizers to keep local tests/builds portable.
type OnnxEmbedder struct {
	// session   *ort.Session  // TODO: uncomment when onnxruntime_go is available
	dimCount      int32
	modelPath     string
	tokenizerPath string
}

// NewOnnxEmbedder creates a new ONNX embedder from model and tokenizer paths.
// For now, validates files exist and sets up dimension info without full ONNX loading.
func NewOnnxEmbedder(modelPath, tokenizerPath string) (*OnnxEmbedder, error) {
	log.Printf("Initializing OnnxEmbedder (fallback mode without CGO tokenizers)")

	if _, err := os.Stat(tokenizerPath); err != nil {
		return nil, fmt.Errorf("failed to access tokenizer file %s: %w", tokenizerPath, err)
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("failed to access model file %s: %w", modelPath, err)
	}

	// TODO: In Phase 10, initialize ONNX Runtime environment and load model session
	// For now, assume standard Nomic v1.5 embedding dimension (384)
	dimCount := int32(384)

	return &OnnxEmbedder{
		dimCount:      dimCount,
		modelPath:     modelPath,
		tokenizerPath: tokenizerPath,
	}, nil
}

// Embed generates an embedding vector for the given text.
// Returns a placeholder vector of the correct dimension.
// TODO: In Phase 10, replace with actual ONNX inference.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("input text is empty")
	}

	// Default fallback tokenization for portability in environments without tokenizers native libs.
	tokens := strings.Fields(strings.ToLower(text))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("tokenization returned empty token set")
	}

	// Log tokenization for debugging
	log.Printf("Tokenized text into %d tokens (fallback mode)", len(tokens))

	// TODO: Replace this with actual ONNX inference in Phase 10
	// For now, return a placeholder vector (all zeros) of correct dimension
	// This allows the rest of the RAG pipeline to compile and test
	vector := make([]float32, e.dimCount)
	for i := range vector {
		vector[i] = 0.0 // Placeholder: would be filled by ONNX inference
	}

	return vector, nil
}

// GetDimension returns the embedding vector dimension.
func (e *OnnxEmbedder) GetDimension() int32 {
	return e.dimCount
}

// Close cleans up the embedder resources.
func (e *OnnxEmbedder) Close() error {
	// TODO: In Phase 10, call session.Destroy() and ort.Shutdown()
	return nil
}
