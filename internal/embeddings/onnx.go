package embeddings

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"ai-tutor/internal/utils"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

// OnnxEmbedder handles embedding generation using ONNX Runtime.
type OnnxEmbedder struct {
	tokenizer     *tokenizer.Tokenizer
	session       *ort.DynamicAdvancedSession
	inputInfo     []ort.InputOutputInfo
	outputInfo    []ort.InputOutputInfo
	dimCount      int32
	maxSeqLen     int
	padID         int64
	modelPath     string
	tokenizerPath string
	runtimeOwned  bool
	mu            sync.Mutex
}

// NewOnnxEmbedder creates a new ONNX embedder from model, tokenizer, and runtime paths.
func NewOnnxEmbedder(modelPath, tokenizerPath, configuredRuntimePath string) (*OnnxEmbedder, error) {
	utils.Infof("Initializing OnnxEmbedder")

	if _, err := os.Stat(tokenizerPath); err != nil {
		return nil, fmt.Errorf("failed to access tokenizer file %s: %w", tokenizerPath, err)
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("failed to access model file %s: %w", modelPath, err)
	}

	tok, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer %s: %w", tokenizerPath, err)
	}

	padID := int64(0)
	for _, candidate := range []string{"[PAD]", "<pad>", "<PAD>"} {
		if id, ok := tok.TokenToId(candidate); ok {
			padID = int64(id)
			break
		}
	}

	runtimePath, err := resolveRuntimeLibraryPath(configuredRuntimePath, modelPath)
	if err != nil {
		return nil, err
	}

	runtimeOwned := false
	if !ort.IsInitialized() {
		ort.SetSharedLibraryPath(runtimePath)
		if err := ort.InitializeEnvironment(ort.WithLogLevelError()); err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
		}
		runtimeOwned = true
	}

	inputInfo, outputInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		if runtimeOwned {
			_ = ort.DestroyEnvironment()
		}
		return nil, fmt.Errorf("failed to inspect model I/O: %w", err)
	}
	if len(inputInfo) == 0 {
		if runtimeOwned {
			_ = ort.DestroyEnvironment()
		}
		return nil, fmt.Errorf("model has no inputs")
	}
	if len(outputInfo) == 0 {
		if runtimeOwned {
			_ = ort.DestroyEnvironment()
		}
		return nil, fmt.Errorf("model has no outputs")
	}

	maxSeqLen := inferMaxSeqLen(inputInfo, 256)

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		extractIONames(inputInfo),
		extractIONames(outputInfo),
		nil,
	)
	if err != nil {
		if runtimeOwned {
			_ = ort.DestroyEnvironment()
		}
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	embedder := &OnnxEmbedder{
		tokenizer:     tok,
		session:       session,
		inputInfo:     inputInfo,
		outputInfo:    outputInfo,
		dimCount:      0,
		maxSeqLen:     maxSeqLen,
		padID:         padID,
		modelPath:     modelPath,
		tokenizerPath: tokenizerPath,
		runtimeOwned:  runtimeOwned,
	}

	warmupVec, err := embedder.embedInternal("warmup")
	if err != nil {
		_ = embedder.Close()
		return nil, fmt.Errorf("failed warmup inference: %w", err)
	}
	if len(warmupVec) == 0 {
		_ = embedder.Close()
		return nil, fmt.Errorf("warmup inference returned an empty vector")
	}
	embedder.dimCount = int32(len(warmupVec))

	utils.Infof("OnnxEmbedder initialized (seq=%d dim=%d runtime=%s)", embedder.maxSeqLen, embedder.dimCount, runtimePath)

	return embedder, nil
}

// Embed generates an embedding vector for the given text.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("input text is empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	vector, err := e.embedInternal(text)
	if err != nil {
		return nil, err
	}
	if e.dimCount == 0 {
		e.dimCount = int32(len(vector))
	}
	if len(vector) != int(e.dimCount) {
		return nil, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(vector), e.dimCount)
	}

	return vector, nil
}

// GetDimension returns the embedding vector dimension.
func (e *OnnxEmbedder) GetDimension() int32 {
	return e.dimCount
}

// Close cleans up the embedder resources.
func (e *OnnxEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var firstErr error
	if e.session != nil {
		if err := e.session.Destroy(); err != nil {
			firstErr = err
		}
		e.session = nil
	}
	if e.runtimeOwned && ort.IsInitialized() {
		if err := ort.DestroyEnvironment(); err != nil && firstErr == nil {
			firstErr = err
		}
		e.runtimeOwned = false
	}

	if firstErr != nil {
		return fmt.Errorf("failed to close embedder resources: %w", firstErr)
	}

	return nil
}

func (e *OnnxEmbedder) embedInternal(text string) ([]float32, error) {
	if e.tokenizer == nil {
		return nil, fmt.Errorf("tokenizer not initialized")
	}
	if e.session == nil {
		return nil, fmt.Errorf("onnx session not initialized")
	}

	inputIDs, attentionMask, tokenTypeIDs, err := e.tokenize(text)
	if err != nil {
		return nil, err
	}

	shape := ort.NewShape(1, int64(len(inputIDs)))
	inputs, err := e.buildInputValues(shape, inputIDs, attentionMask, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer destroyValues(inputs)

	outputs := make([]ort.Value, len(e.outputInfo))
	if err := e.session.Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("onnx inference failed: %w", err)
	}
	defer destroyValues(outputs)

	vector, err := extractEmbedding(outputs, e.outputInfo, attentionMask)
	if err != nil {
		return nil, err
	}

	normalizeL2(vector)
	return vector, nil
}

func (e *OnnxEmbedder) tokenize(text string) ([]int64, []int64, []int64, error) {
	enc, err := e.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tokenizer encode failed: %w", err)
	}
	return buildTokenArrays(enc.Ids, enc.AttentionMask, enc.TypeIds, e.maxSeqLen)
}

func buildTokenArrays(sourceIDs []int, sourceMask []int, sourceTypeIDs []int, maxSeqLen int) ([]int64, []int64, []int64, error) {
	if len(sourceIDs) == 0 {
		return nil, nil, nil, fmt.Errorf("tokenizer returned no token ids")
	}

	// Compute target length upfront to avoid over-allocating for long inputs
	targetLen := len(sourceIDs)
	if maxSeqLen > 0 && targetLen > maxSeqLen {
		targetLen = maxSeqLen
	}

	ids := make([]int64, targetLen)
	for i := 0; i < targetLen; i++ {
		ids[i] = int64(sourceIDs[i])
	}

	mask := make([]int64, targetLen)
	if len(sourceMask) >= targetLen {
		for i := 0; i < targetLen; i++ {
			if sourceMask[i] > 0 {
				mask[i] = 1
			}
		}
	} else if len(sourceMask) > 0 {
		// Partial mask available
		for i := 0; i < len(sourceMask) && i < targetLen; i++ {
			if sourceMask[i] > 0 {
				mask[i] = 1
			}
		}
		// Fill remaining with 1 (attention enabled)
		for i := len(sourceMask); i < targetLen; i++ {
			mask[i] = 1
		}
	} else {
		// No mask provided, enable attention for all
		for i := 0; i < targetLen; i++ {
			mask[i] = 1
		}
	}

	typeIDs := make([]int64, targetLen)
	if len(sourceTypeIDs) >= targetLen {
		for i := 0; i < targetLen; i++ {
			typeIDs[i] = int64(sourceTypeIDs[i])
		}
	}

	return ids, mask, typeIDs, nil
}

func (e *OnnxEmbedder) buildInputValues(shape ort.Shape, ids, attentionMask, tokenTypeIDs []int64) ([]ort.Value, error) {
	values := make([]ort.Value, 0, len(e.inputInfo))
	for i, info := range e.inputInfo {
		source := pickInputSource(info.Name, i)
		var data []int64
		switch source {
		case "attention_mask":
			data = attentionMask
		case "token_type_ids":
			data = tokenTypeIDs
		default:
			data = ids
		}

		value, err := tensorFromInputData(info, shape, data)
		if err != nil {
			destroyValues(values)
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func tensorFromInputData(info ort.InputOutputInfo, shape ort.Shape, data []int64) (ort.Value, error) {
	switch info.DataType {
	case ort.TensorElementDataTypeInt64:
		t, err := ort.NewTensor(shape, data)
		if err != nil {
			return nil, fmt.Errorf("failed to create int64 tensor for %s: %w", info.Name, err)
		}
		return t, nil
	case ort.TensorElementDataTypeInt32:
		casted := make([]int32, len(data))
		for i, v := range data {
			casted[i] = int32(v)
		}
		t, err := ort.NewTensor(shape, casted)
		if err != nil {
			return nil, fmt.Errorf("failed to create int32 tensor for %s: %w", info.Name, err)
		}
		return t, nil
	default:
		return nil, fmt.Errorf("unsupported input dtype for %s: %s", info.Name, info.DataType)
	}
}

func extractEmbedding(outputs []ort.Value, outputInfo []ort.InputOutputInfo, attentionMask []int64) ([]float32, error) {
	for i, output := range outputs {
		if output == nil {
			continue
		}

		if tensor, ok := output.(*ort.Tensor[float32]); ok {
			vector, err := poolFloat32Tensor(tensor, attentionMask)
			if err == nil && len(vector) > 0 {
				return vector, nil
			}
		}

		if tensor, ok := output.(*ort.Tensor[float64]); ok {
			vector, err := poolFloat64Tensor(tensor, attentionMask)
			if err == nil && len(vector) > 0 {
				return vector, nil
			}
		}

		if i < len(outputInfo) {
			log.Printf("Skipping unsupported output %s dtype=%s", outputInfo[i].Name, outputInfo[i].DataType)
		}
	}

	return nil, fmt.Errorf("model did not return a supported float embedding tensor")
}

func poolFloat32Tensor(t *ort.Tensor[float32], attentionMask []int64) ([]float32, error) {
	shape := t.GetShape()
	data := t.GetData()
	if len(data) == 0 {
		return nil, fmt.Errorf("output tensor is empty")
	}

	switch len(shape) {
	case 1:
		vector := make([]float32, len(data))
		copy(vector, data)
		return vector, nil
	case 2:
		rows := int(shape[0])
		cols := int(shape[1])
		if rows <= 0 || cols <= 0 {
			return nil, fmt.Errorf("invalid 2D output shape: %v", shape)
		}
		if rows == 1 {
			vector := make([]float32, cols)
			copy(vector, data[:cols])
			return vector, nil
		}
		return meanPool2D(data, rows, cols, attentionMask), nil
	case 3:
		batch := int(shape[0])
		seqLen := int(shape[1])
		hidden := int(shape[2])
		if batch <= 0 || seqLen <= 0 || hidden <= 0 {
			return nil, fmt.Errorf("invalid 3D output shape: %v", shape)
		}
		return meanPool3D(data, seqLen, hidden, attentionMask), nil
	default:
		return nil, fmt.Errorf("unsupported output tensor rank: %d", len(shape))
	}
}

func poolFloat64Tensor(t *ort.Tensor[float64], attentionMask []int64) ([]float32, error) {
	shape := t.GetShape()
	data := t.GetData()
	if len(data) == 0 {
		return nil, fmt.Errorf("output tensor is empty")
	}

	toFloat32 := func(in []float64) []float32 {
		out := make([]float32, len(in))
		for i, v := range in {
			out[i] = float32(v)
		}
		return out
	}

	switch len(shape) {
	case 1:
		return toFloat32(data), nil
	case 2:
		rows := int(shape[0])
		cols := int(shape[1])
		if rows <= 0 || cols <= 0 {
			return nil, fmt.Errorf("invalid 2D output shape: %v", shape)
		}
		if rows == 1 {
			return toFloat32(data[:cols]), nil
		}
		pooled := meanPool2DFloat64(data, rows, cols, attentionMask)
		return toFloat32(pooled), nil
	case 3:
		seqLen := int(shape[1])
		hidden := int(shape[2])
		if seqLen <= 0 || hidden <= 0 {
			return nil, fmt.Errorf("invalid 3D output shape: %v", shape)
		}
		pooled := meanPool3DFloat64(data, seqLen, hidden, attentionMask)
		return toFloat32(pooled), nil
	default:
		return nil, fmt.Errorf("unsupported output tensor rank: %d", len(shape))
	}
}

func meanPool3D(data []float32, seqLen, hidden int, attentionMask []int64) []float32 {
	vector := make([]float32, hidden)
	count := float32(0)
	for tokenIdx := 0; tokenIdx < seqLen; tokenIdx++ {
		if tokenIdx < len(attentionMask) && attentionMask[tokenIdx] == 0 {
			continue
		}
		offset := tokenIdx * hidden
		for d := 0; d < hidden; d++ {
			vector[d] += data[offset+d]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	for i := range vector {
		vector[i] /= count
	}
	return vector
}

func meanPool2D(data []float32, rows, cols int, attentionMask []int64) []float32 {
	vector := make([]float32, cols)
	count := float32(0)
	for row := 0; row < rows; row++ {
		if row < len(attentionMask) && attentionMask[row] == 0 {
			continue
		}
		offset := row * cols
		for col := 0; col < cols; col++ {
			vector[col] += data[offset+col]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	for i := range vector {
		vector[i] /= count
	}
	return vector
}

func meanPool3DFloat64(data []float64, seqLen, hidden int, attentionMask []int64) []float64 {
	vector := make([]float64, hidden)
	count := float64(0)
	for tokenIdx := 0; tokenIdx < seqLen; tokenIdx++ {
		if tokenIdx < len(attentionMask) && attentionMask[tokenIdx] == 0 {
			continue
		}
		offset := tokenIdx * hidden
		for d := 0; d < hidden; d++ {
			vector[d] += data[offset+d]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	for i := range vector {
		vector[i] /= count
	}
	return vector
}

func meanPool2DFloat64(data []float64, rows, cols int, attentionMask []int64) []float64 {
	vector := make([]float64, cols)
	count := float64(0)
	for row := 0; row < rows; row++ {
		if row < len(attentionMask) && attentionMask[row] == 0 {
			continue
		}
		offset := row * cols
		for col := 0; col < cols; col++ {
			vector[col] += data[offset+col]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	for i := range vector {
		vector[i] /= count
	}
	return vector
}

func normalizeL2(vector []float32) {
	if len(vector) == 0 {
		return
	}

	var sum float64
	for _, value := range vector {
		sum += float64(value * value)
	}
	if sum == 0 {
		return
	}

	norm := float32(math.Sqrt(sum))
	for i := range vector {
		vector[i] /= norm
	}
}

func extractIONames(info []ort.InputOutputInfo) []string {
	names := make([]string, len(info))
	for i, item := range info {
		names[i] = item.Name
	}
	return names
}

func pickInputSource(name string, index int) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "attention") && strings.Contains(lower, "mask"):
		return "attention_mask"
	case strings.Contains(lower, "token") && strings.Contains(lower, "type"):
		return "token_type_ids"
	case strings.Contains(lower, "segment"):
		return "token_type_ids"
	case strings.Contains(lower, "input") && strings.Contains(lower, "id"):
		return "input_ids"
	case index == 1:
		return "attention_mask"
	case index >= 2:
		return "token_type_ids"
	default:
		return "input_ids"
	}
}

func inferMaxSeqLen(inputInfo []ort.InputOutputInfo, fallback int) int {
	for _, info := range inputInfo {
		if len(info.Dimensions) >= 2 {
			dim := int(info.Dimensions[1])
			if dim > 0 && dim <= 4096 {
				return dim
			}
		}
	}
	return fallback
}

func destroyValues(values []ort.Value) {
	for _, value := range values {
		if value != nil {
			_ = value.Destroy()
		}
	}
}

func resolveRuntimeLibraryPath(configuredPath, modelPath string) (string, error) {
	if configuredPath != "" {
		if _, err := os.Stat(configuredPath); err == nil {
			abs, absErr := filepath.Abs(configuredPath)
			if absErr == nil {
				return abs, nil
			}
			return configuredPath, nil
		}
	}

	modelDir := filepath.Dir(modelPath)
	var candidates []string

	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			filepath.Join(modelDir, "onnxruntime.dll"),
			"onnxruntime.dll",
		}
	case "darwin":
		candidates = []string{
			filepath.Join(modelDir, "libonnxruntime.dylib"),
			"libonnxruntime.dylib",
		}
	default:
		candidates = []string{
			filepath.Join(modelDir, "libonnxruntime.so"),
			filepath.Join(modelDir, "onnxruntime.so"),
			"libonnxruntime.so",
			"onnxruntime.so",
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, absErr := filepath.Abs(candidate)
			if absErr == nil {
				return abs, nil
			}
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to locate ONNX runtime shared library near %s", modelPath)
}
