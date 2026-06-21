//go:build !cgo

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ai-tutor/internal/utils"
)

func isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		utils.RagLogger.Warn("sqlite3_tutor (nocgo): path allowed check failed to resolve absolute path", "path", path, "error", err)
		return false
	}

	// Base validation
	baseName := filepath.Base(absPath)
	switch runtime.GOOS {
	case "windows":
		if baseName != "vec0.dll" {
			utils.RagLogger.Warn("sqlite3_tutor (nocgo): path allowed check failed baseName validation for windows", "baseName", baseName)
			return false
		}
	case "darwin":
		if baseName != "vec0.dylib" {
			utils.RagLogger.Warn("sqlite3_tutor (nocgo): path allowed check failed baseName validation for darwin", "baseName", baseName)
			return false
		}
	default:
		if baseName != "vec0.so" {
			utils.RagLogger.Warn("sqlite3_tutor (nocgo): path allowed check failed baseName validation for linux/other", "baseName", baseName)
			return false
		}
	}

	// Build list of allowed directories
	allowedDirs := []string{}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		allowedDirs = append(allowedDirs, filepath.Join(localAppData, "ai-tutor", "assets", "runtime"))
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		allowedDirs = append(allowedDirs, filepath.Join(cacheDir, "ai-tutor", "assets", "runtime"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		allowedDirs = append(allowedDirs, filepath.Join(homeDir, ".ai-tutor", "assets", "runtime"))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		allowedDirs = append(allowedDirs, filepath.Join(wd, "assets", "runtime"))
		allowedDirs = append(allowedDirs, filepath.Join(wd, "dev_data", "assets", "runtime"))
		allowedDirs = append(allowedDirs, filepath.Join(wd, "dev_data", "runtime"))
	}

	// Check if absPath resides strictly within any of the allowed directories
	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absDir, absPath)
		if err != nil {
			continue
		}
		// If Rel does not start with ".." and is not ".", it's inside the allowed directory
		if !strings.HasPrefix(rel, "..") && rel != "." {
			utils.RagLogger.Info("sqlite3_tutor (nocgo): path allowed check passed", "path", path, "absPath", absPath, "matchedDir", absDir)
			return true
		}
	}

	utils.RagLogger.Warn("sqlite3_tutor (nocgo): path allowed check failed, absPath does not reside inside any allowed directory", "absPath", absPath, "allowedDirs", allowedDirs)
	return false
}

func setExtensionPath(path string) {
	utils.RagLogger.Warn("sqlite3_tutor (nocgo): setExtensionPath is a no-op since CGO is disabled", "path", path)
}

func loadExtension(db *sql.DB, extensionPath string) error {
	cleanedPath := filepath.Clean(extensionPath)
	utils.RagLogger.Warn("sqlite3_tutor (nocgo): loadExtension triggered in non-CGO build", "path", extensionPath, "cleanedPath", cleanedPath)
	if !isPathAllowed(cleanedPath) {
		utils.RagLogger.Error("sqlite3_tutor (nocgo): loading extension from unauthorized path blocked", "path", cleanedPath)
		return fmt.Errorf("loading extension from unauthorized path is blocked: %s", cleanedPath)
	}
	utils.RagLogger.Error("sqlite3_tutor (nocgo): loading extension failed: CGO_ENABLED=1 is required")
	return fmt.Errorf("sqlite-vec extension loading requires CGO_ENABLED=1")
}


