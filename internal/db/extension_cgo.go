//go:build cgo

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var (
	extensionMu   sync.RWMutex
	extensionPath string
)

func init() {
	sql.Register("sqlite3_tutor", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			extensionMu.RLock()
			path := extensionPath
			extensionMu.RUnlock()

			if path == "" {
				return nil
			}

			entryPoints := []string{"sqlite3_vec_init", "sqlite3_extension_init", ""}
			var lastErr error
			for _, entry := range entryPoints {
				if err := conn.LoadExtension(path, entry); err == nil {
					return nil
				} else {
					lastErr = err
				}
			}
			return fmt.Errorf("could not load extension from %s with known entry points: %w", path, lastErr)
		},
	})
}

func setExtensionPath(path string) {
	extensionMu.Lock()
	extensionPath = path
	extensionMu.Unlock()
}

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

func loadExtension(db *sql.DB, path string) error {
	cleanedPath := filepath.Clean(path)
	if !isPathAllowed(cleanedPath) {
		return fmt.Errorf("loading extension from unauthorized path is blocked: %s", cleanedPath)
	}

	setExtensionPath(cleanedPath)

	// Trigger connection creation and extension load by querying vec_version()
	var version string
	if err := db.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		setExtensionPath("")
		return fmt.Errorf("extension load verification failed: %w", err)
	}

	return nil
}

