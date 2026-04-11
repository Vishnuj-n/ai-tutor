//go:build tokenizers

package embeddings

import (
	"fmt"
	"log"
	"os"

	"github.com/daulet/tokenizers"
)

// OnnxEmbedder handles embedding generation using ONNX Runtime and HuggingFace tokenizer.
// This variant uses github.com/daulet/tokenizers and requires native tokenizer libraries.
type OnnxEmbedder struct {
	tokenizer *tokenizers.Tokenizer
	// session   *ort.Session  // TODO: uncomment when onnxruntime_go is available
	dimCount      int32
	modelPath     string
	tokenizerPath string
}

// NewOnnxEmbedder creates a new ONNX embedder from model and tokenizer paths.
func NewOnnxEmbedder(modelPath, tokenizerPath string) (*OnnxEmbedder, error) {
	log.Printf("Initializing OnnxEmbedder (tokenizers build tag enabled)")

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("failed to access model file %s: %w", modelPath, err)
	}

	tok, err := tokenizers.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer from %s: %w", tokenizerPath, err)
	}

	dimCount := int32(384)

	return &OnnxEmbedder{
		tokenizer:     tok,
		dimCount:      dimCount,
		modelPath:     modelPath,
		tokenizerPath: tokenizerPath,
	}, nil
}

// Embed tokenizes text and returns a placeholder embedding vector.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	if e.tokenizer == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}

	tokenIDs := e.tokenizer.Encode(text, false)
	if tokenIDs == nil {
		return nil, fmt.Errorf("tokenization returned nil")
	}

	log.Printf("Tokenized text into %d tokens", len(tokenIDs))

	vector := make([]float32, e.dimCount)
	for i := range vector {
		vector[i] = 0.0
	}

	return vector, nil
}

// GetDimension returns the embedding vector dimension.
func (e *OnnxEmbedder) GetDimension() int32 {
	return e.dimCount
}

// Close cleans up the embedder resources.
func (e *OnnxEmbedder) Close() error {
	if e.tokenizer != nil {
		e.tokenizer = nil
	}
	return nil
}
