package embeddings

import (
	"fmt"
	"log"

	"github.com/daulet/tokenizers"
)

// OnnxEmbedder handles embedding generation using ONNX Runtime and HuggingFace tokenizer.
// NOTE: This is a temporary stub. Phase 10 will wire in actual yalue/onnxruntime_go binding.
type OnnxEmbedder struct {
	tokenizer *tokenizers.Tokenizer
	// session   *ort.Session  // TODO: uncomment when onnxruntime_go is available
	dimCount  int32
	modelPath string
}

// NewOnnxEmbedder creates a new ONNX embedder from model and tokenizer paths.
// For now, validates files exist and sets up dimension info without full ONNX loading.
func NewOnnxEmbedder(modelPath, tokenizerPath string) (*OnnxEmbedder, error) {
	log.Printf("Initializing OnnxEmbedder (stub mode - full ONNX binding in Phase 10)")

	// Load tokenizer from HuggingFace tokenizer.json
	tok, err := tokenizers.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer from %s: %w", tokenizerPath, err)
	}

	// TODO: In Phase 10, initialize ONNX Runtime environment and load model session
	// For now, assume standard Nomic v1.5 embedding dimension (384)
	dimCount := int32(384)

	return &OnnxEmbedder{
		tokenizer: tok,
		dimCount:  dimCount,
		modelPath: modelPath,
	}, nil
}

// Embed generates an embedding vector for the given text.
// Returns a placeholder vector of the correct dimension.
// TODO: In Phase 10, replace with actual ONNX inference.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	if e.tokenizer == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}

	// Tokenize the input text using the tokenizer
	// Encode returns []uint32 (token IDs) directly
	tokenIDs := e.tokenizer.Encode(text, false)
	if tokenIDs == nil {
		return nil, fmt.Errorf("tokenization returned nil")
	}

	// Log tokenization for debugging
	log.Printf("Tokenized text into %d tokens", len(tokenIDs))

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
	if e.tokenizer != nil {
		// Tokenizer doesn't need explicit cleanup in this binding
		e.tokenizer = nil
	}
	// TODO: In Phase 10, call session.Destroy() and ort.Shutdown()
	return nil
}
