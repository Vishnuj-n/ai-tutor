//go:build cgo

package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
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

	sqlConn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		_ = sqlConn.Close()
	}()

	return sqlConn.Raw(func(driverConn interface{}) error {
		sqliteConn, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver connection type %T", driverConn)
		}

		entryPoints := []string{"sqlite3_vec_init", "sqlite3_extension_init", ""}
		var lastErr error
		for _, entry := range entryPoints {
			if loadErr := sqliteConn.LoadExtension(cleanedPath, entry); loadErr == nil {
				return nil
			} else {
				lastErr = loadErr
			}
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("unknown extension load failure")
		}
		return fmt.Errorf("could not load extension with known entry points: %w", lastErr)
	})
}

