package db

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func initDBForTest(t *testing.T, withVec bool, dim int32) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ai-tutor-db-test.sqlite")
	vecPath := ""
	if withVec {
		vecPath = vecAssetPath(t)
	}

	if err := Init(dbPath, vecPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	if withVec && dim > 0 {
		if err := InitWithVectorDimension(dim); err != nil {
			t.Skipf("skipping vec0-backed test, InitWithVectorDimension failed: %v", err)
		}
	}
}

func vecAssetPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve caller path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "asset", "vec0.dll")
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("failed to resolve vec0.dll path: %v", err)
	}
	return absPath
}

func assertCountEquals(t *testing.T, query string, arg interface{}, want int) {
	t.Helper()

	var got int
	if err := conn.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("query failed (%s): %v", sanitizeWhitespace(query), err)
	}
	if got != want {
		t.Fatalf("unexpected count for query (%s): got=%d want=%d", sanitizeWhitespace(query), got, want)
	}
}

func sanitizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
