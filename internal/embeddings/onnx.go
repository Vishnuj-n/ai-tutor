package embeddings

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"
)

// OnnxEmbedder handles embedding generation using ONNX Runtime.
// Tokenization is implemented in pure Go and loaded from tokenizer.json.
type OnnxEmbedder struct {
	// session   *ort.Session  // TODO: uncomment when onnxruntime_go is available
	tokenizer     *wordPieceTokenizer
	dimCount      int32
	modelPath     string
	tokenizerPath string
	maxSeqLen     int
}

type tokenizerConfig struct {
	Normalizer struct {
		Lowercase bool `json:"lowercase"`
	} `json:"normalizer"`
	Model struct {
		Type                    string         `json:"type"`
		UnkToken                string         `json:"unk_token"`
		ContinuingSubwordPrefix string         `json:"continuing_subword_prefix"`
		MaxInputCharsPerWord    int            `json:"max_input_chars_per_word"`
		Vocab                   map[string]int `json:"vocab"`
	} `json:"model"`
}

type wordPieceTokenizer struct {
	vocab                   map[string]int
	unkToken                string
	continuingSubwordPrefix string
	maxInputCharsPerWord    int
	doLowercase             bool
	clsID                   int
	sepID                   int
	padID                   int
	unkID                   int
}

// NewOnnxEmbedder creates a new ONNX embedder from model and tokenizer paths.
// It loads tokenizer.json in pure Go and prepares token IDs/masks for ONNX input tensors.
func NewOnnxEmbedder(modelPath, tokenizerPath string) (*OnnxEmbedder, error) {
	log.Printf("Initializing OnnxEmbedder (pure Go tokenizer)")

	if _, err := os.Stat(tokenizerPath); err != nil {
		return nil, fmt.Errorf("failed to access tokenizer file %s: %w", tokenizerPath, err)
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("failed to access model file %s: %w", modelPath, err)
	}

	tok, err := loadWordPieceTokenizer(tokenizerPath)
	if err != nil {
		return nil, err
	}

	// TODO: In Phase 10, initialize ONNX Runtime environment and load model session
	// For now, assume standard Nomic v1.5 embedding dimension (384)
	dimCount := int32(384)

	return &OnnxEmbedder{
		tokenizer:     tok,
		dimCount:      dimCount,
		modelPath:     modelPath,
		tokenizerPath: tokenizerPath,
		maxSeqLen:     256,
	}, nil
}

// Embed generates an embedding vector for the given text.
// Returns a placeholder vector of the correct dimension.
// TODO: In Phase 10, replace with actual ONNX inference.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("input text is empty")
	}
	if e.tokenizer == nil {
		return nil, fmt.Errorf("tokenizer not initialized")
	}

	inputIDs, attentionMask, err := e.tokenizer.Encode(text, e.maxSeqLen)
	if err != nil {
		return nil, err
	}

	log.Printf("Tokenized text into %d ids with %d active mask entries", len(inputIDs), sumMask(attentionMask))

	// TODO: Replace this with actual ONNX inference in Phase 10
	// Planned ONNX inputs:
	// - input_ids:      int64[maxSeqLen]
	// - attention_mask: int64[maxSeqLen]
	_ = inputIDs
	_ = attentionMask

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

func loadWordPieceTokenizer(tokenizerPath string) (*wordPieceTokenizer, error) {
	raw, err := os.ReadFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokenizer file %s: %w", tokenizerPath, err)
	}

	var cfg tokenizerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse tokenizer file %s: %w", tokenizerPath, err)
	}

	if cfg.Model.Type != "WordPiece" {
		return nil, fmt.Errorf("unsupported tokenizer model type: %s", cfg.Model.Type)
	}
	if len(cfg.Model.Vocab) == 0 {
		return nil, fmt.Errorf("tokenizer vocab is empty")
	}

	clsID, ok := cfg.Model.Vocab["[CLS]"]
	if !ok {
		return nil, fmt.Errorf("tokenizer vocab missing [CLS]")
	}
	sepID, ok := cfg.Model.Vocab["[SEP]"]
	if !ok {
		return nil, fmt.Errorf("tokenizer vocab missing [SEP]")
	}
	padID, ok := cfg.Model.Vocab["[PAD]"]
	if !ok {
		return nil, fmt.Errorf("tokenizer vocab missing [PAD]")
	}

	unkToken := cfg.Model.UnkToken
	if strings.TrimSpace(unkToken) == "" {
		unkToken = "[UNK]"
	}
	unkID, ok := cfg.Model.Vocab[unkToken]
	if !ok {
		return nil, fmt.Errorf("tokenizer vocab missing unknown token %s", unkToken)
	}

	maxChars := cfg.Model.MaxInputCharsPerWord
	if maxChars <= 0 {
		maxChars = 100
	}

	prefix := cfg.Model.ContinuingSubwordPrefix
	if prefix == "" {
		prefix = "##"
	}

	return &wordPieceTokenizer{
		vocab:                   cfg.Model.Vocab,
		unkToken:                unkToken,
		continuingSubwordPrefix: prefix,
		maxInputCharsPerWord:    maxChars,
		doLowercase:             cfg.Normalizer.Lowercase,
		clsID:                   clsID,
		sepID:                   sepID,
		padID:                   padID,
		unkID:                   unkID,
	}, nil
}

func (t *wordPieceTokenizer) Encode(text string, maxLen int) ([]int64, []int64, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil, fmt.Errorf("input text is empty")
	}
	if maxLen < 2 {
		return nil, nil, fmt.Errorf("max sequence length must be at least 2")
	}

	normalized := text
	if t.doLowercase {
		normalized = strings.ToLower(normalized)
	}

	words := splitBertTokens(normalized)
	ids := make([]int64, 0, maxLen)
	ids = append(ids, int64(t.clsID))

	for _, word := range words {
		wordIDs := t.encodeWord(word)
		for _, id := range wordIDs {
			ids = append(ids, int64(id))
			if len(ids) == maxLen-1 {
				break
			}
		}
		if len(ids) == maxLen-1 {
			break
		}
	}

	ids = append(ids, int64(t.sepID))

	if len(ids) > maxLen {
		ids = ids[:maxLen]
		ids[maxLen-1] = int64(t.sepID)
	}

	mask := make([]int64, len(ids))
	for i := range mask {
		mask[i] = 1
	}

	for len(ids) < maxLen {
		ids = append(ids, int64(t.padID))
		mask = append(mask, 0)
	}

	return ids, mask, nil
}

func (t *wordPieceTokenizer) encodeWord(word string) []int {
	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) > t.maxInputCharsPerWord {
		return []int{t.unkID}
	}

	pieces := make([]int, 0, len(runes))
	start := 0
	for start < len(runes) {
		end := len(runes)
		found := false
		for start < end {
			sub := string(runes[start:end])
			if start > 0 {
				sub = t.continuingSubwordPrefix + sub
			}

			id, ok := t.vocab[sub]
			if ok {
				pieces = append(pieces, id)
				start = end
				found = true
				break
			}
			end--
		}

		if !found {
			return []int{t.unkID}
		}
	}

	return pieces
}

func splitBertTokens(text string) []string {
	parts := make([]string, 0)
	var current []rune

	flush := func() {
		if len(current) > 0 {
			parts = append(parts, string(current))
			current = current[:0]
		}
	}

	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			flush()
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current = append(current, r)
		default:
			flush()
			parts = append(parts, string(r))
		}
	}

	flush()
	return parts
}

func sumMask(mask []int64) int {
	total := 0
	for _, v := range mask {
		if v > 0 {
			total++
		}
	}
	return total
}
