//go:build !cgo

package db

import (
	"ai-tutor/internal/utils"
)

func setExtensionPath(path string) {
	utils.RagLogger.Warn("sqlite3_tutor (nocgo): setExtensionPath is a no-op since CGO is disabled", "path", path)
}


