package db

import (
	"path/filepath"
	"runtime"
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
