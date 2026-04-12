package embeddings

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sugarme/tokenizer/pretrained"
)

func TestTokenizeTruncatesToMaxSeqLen(t *testing.T) {
	tokenizerPath := tokenizerAssetPath(t)
	tok, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		t.Fatalf("failed to load tokenizer: %v", err)
	}

	embedder := &OnnxEmbedder{
		tokenizer: tok,
		maxSeqLen: 12,
	}

	longText := strings.Repeat("token ", 300)
	ids, mask, typeIDs, err := embedder.tokenize(longText)
	if err != nil {
		t.Fatalf("tokenize returned error: %v", err)
	}

	if len(ids) != 12 || len(mask) != 12 || len(typeIDs) != 12 {
		t.Fatalf("expected all token arrays to be truncated to 12, got ids=%d mask=%d typeIDs=%d",
			len(ids), len(mask), len(typeIDs))
	}
}

func TestTokenizeShortInputRemainsVariableLength(t *testing.T) {
	tokenizerPath := tokenizerAssetPath(t)
	tok, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		t.Fatalf("failed to load tokenizer: %v", err)
	}

	embedder := &OnnxEmbedder{
		tokenizer: tok,
		maxSeqLen: 128,
	}

	ids, mask, typeIDs, err := embedder.tokenize("short prompt")
	if err != nil {
		t.Fatalf("tokenize returned error: %v", err)
	}

	if len(ids) == 0 {
		t.Fatalf("expected at least one token id")
	}
	if len(ids) >= 128 {
		t.Fatalf("expected short input to remain variable-length (<128), got %d", len(ids))
	}
	if len(mask) != len(ids) || len(typeIDs) != len(ids) {
		t.Fatalf("expected equal lengths for ids/mask/typeIDs, got ids=%d mask=%d typeIDs=%d",
			len(ids), len(mask), len(typeIDs))
	}
}

func TestBuildTokenArraysReturnsErrorForEmptyIDs(t *testing.T) {
	_, _, _, err := buildTokenArrays(nil, nil, nil, 16)
	if err == nil {
		t.Fatalf("expected error for empty token ids")
	}
}

func TestMeanPool3DRespectsAttentionMask(t *testing.T) {
	data := []float32{
		1, 2, // token 0
		3, 4, // token 1 (masked out)
		5, 6, // token 2
	}
	mask := []int64{1, 0, 1}

	got := meanPool3D(data, 3, 2, mask)
	want := []float32{3, 4}
	assertFloat32SliceClose(t, got, want)
}

func TestMeanPool2DRespectsAttentionMask(t *testing.T) {
	data := []float32{
		1, 3, // row 0
		5, 7, // row 1 (masked out)
		9, 11, // row 2
	}
	mask := []int64{1, 0, 1}

	got := meanPool2D(data, 3, 2, mask)
	want := []float32{5, 7}
	assertFloat32SliceClose(t, got, want)
}

func TestMeanPoolFloat64RespectsAttentionMask(t *testing.T) {
	data3D := []float64{
		2, 4,
		6, 8,
		10, 12,
	}
	mask := []int64{1, 0, 1}
	got3D := meanPool3DFloat64(data3D, 3, 2, mask)
	want3D := []float64{6, 8}
	assertFloat64SliceClose(t, got3D, want3D)

	data2D := []float64{
		2, 6,
		8, 10,
		12, 14,
	}
	got2D := meanPool2DFloat64(data2D, 3, 2, mask)
	want2D := []float64{7, 10}
	assertFloat64SliceClose(t, got2D, want2D)
}

func TestMeanPoolWithZeroMaskFallsBackToZeroVector(t *testing.T) {
	data := []float32{
		2, 4,
		6, 8,
	}
	mask := []int64{0, 0}

	got := meanPool2D(data, 2, 2, mask)
	want := []float32{0, 0}
	assertFloat32SliceClose(t, got, want)
}

func TestNormalizeL2(t *testing.T) {
	zero := []float32{0, 0, 0}
	normalizeL2(zero)
	assertFloat32SliceClose(t, zero, []float32{0, 0, 0})

	vector := []float32{3, 4}
	normalizeL2(vector)
	assertFloat32SliceClose(t, vector, []float32{0.6, 0.8})
}

func tokenizerAssetPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve caller path")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "tokenizer.json")
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("failed to resolve tokenizer path: %v", err)
	}
	return absPath
}

func assertFloat32SliceClose(t *testing.T, got, want []float32) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		delta := got[i] - want[i]
		if delta < -1e-6 || delta > 1e-6 {
			t.Fatalf("value mismatch at %d: got=%f want=%f", i, got[i], want[i])
		}
	}
}

func assertFloat64SliceClose(t *testing.T, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		delta := got[i] - want[i]
		if delta < -1e-9 || delta > 1e-9 {
			t.Fatalf("value mismatch at %d: got=%f want=%f", i, got[i], want[i])
		}
	}
}
