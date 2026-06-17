//go:build !cgo

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Base validation
	baseName := filepath.Base(absPath)
	switch runtime.GOOS {
	case "windows":
		if baseName != "vec0.dll" {
			return false
		}
	case "darwin":
		if baseName != "vec0.dylib" {
			return false
		}
	default:
		if baseName != "vec0.so" {
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
			return true
		}
	}

	return false
}

func loadExtension(db *sql.DB, extensionPath string) error {
	cleanedPath := filepath.Clean(extensionPath)
	if !isPathAllowed(cleanedPath) {
		return fmt.Errorf("loading extension from unauthorized path is blocked: %s", cleanedPath)
	}
	return fmt.Errorf("sqlite-vec extension loading requires CGO_ENABLED=1")
}


