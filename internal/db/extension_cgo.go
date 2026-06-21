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

	"ai-tutor/internal/utils"
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

			utils.RagLogger.Info("sqlite3_tutor: ConnectHook triggered", "extensionPath", path)

			if path == "" {
				utils.RagLogger.Warn("sqlite3_tutor: ConnectHook bypassing extension loading since extensionPath is empty")
				return nil
			}

			entryPoints := []string{"sqlite3_vec_init", "sqlite3_extension_init", ""}
			var lastErr error
			for _, entry := range entryPoints {
				utils.RagLogger.Info("sqlite3_tutor: attempting connection load of extension", "path", path, "entryPoint", entry)
				if err := conn.LoadExtension(path, entry); err == nil {
					utils.RagLogger.Info("sqlite3_tutor: extension loaded successfully on connection", "path", path, "entryPoint", entry)
					return nil
				} else {
					utils.RagLogger.Warn("sqlite3_tutor: conn.LoadExtension failed", "path", path, "entryPoint", entry, "error", err)
					lastErr = err
				}
			}
			utils.RagLogger.Error("sqlite3_tutor: all known entry points failed to load extension", "path", path, "error", lastErr)
			return fmt.Errorf("could not load extension from %s with known entry points: %w", path, lastErr)
		},
	})
}

func setExtensionPath(path string) {
	extensionMu.Lock()
	utils.RagLogger.Info("sqlite3_tutor: setting active extension path", "path", path)
	extensionPath = path
	extensionMu.Unlock()
}

func isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		utils.RagLogger.Warn("sqlite3_tutor: path allowed check failed to resolve absolute path", "path", path, "error", err)
		return false
	}

	// Base validation
	baseName := filepath.Base(absPath)
	switch runtime.GOOS {
	case "windows":
		if baseName != "vec0.dll" {
			utils.RagLogger.Warn("sqlite3_tutor: path allowed check failed baseName validation for windows", "baseName", baseName)
			return false
		}
	case "darwin":
		if baseName != "vec0.dylib" {
			utils.RagLogger.Warn("sqlite3_tutor: path allowed check failed baseName validation for darwin", "baseName", baseName)
			return false
		}
	default:
		if baseName != "vec0.so" {
			utils.RagLogger.Warn("sqlite3_tutor: path allowed check failed baseName validation for linux/other", "baseName", baseName)
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
			utils.RagLogger.Info("sqlite3_tutor: path allowed check passed", "path", path, "absPath", absPath, "matchedDir", absDir)
			return true
		}
	}

	utils.RagLogger.Warn("sqlite3_tutor: path allowed check failed, absPath does not reside inside any allowed directory", "absPath", absPath, "allowedDirs", allowedDirs)
	return false
}

func loadExtension(db *sql.DB, path string) error {
	cleanedPath := filepath.Clean(path)
	utils.RagLogger.Info("sqlite3_tutor: loadExtension triggered", "path", path, "cleanedPath", cleanedPath)
	if !isPathAllowed(cleanedPath) {
		utils.RagLogger.Error("sqlite3_tutor: loading extension from unauthorized path blocked", "path", cleanedPath)
		return fmt.Errorf("loading extension from unauthorized path is blocked: %s", cleanedPath)
	}

	setExtensionPath(cleanedPath)

	// Trigger connection creation and extension load by querying vec_version()
	var version string
	utils.RagLogger.Info("sqlite3_tutor: verifying loaded extension using SELECT vec_version()")
	if err := db.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		utils.RagLogger.Error("sqlite3_tutor: extension verification failed", "error", err)
		setExtensionPath("")
		return fmt.Errorf("extension load verification failed: %w", err)
	}

	utils.RagLogger.Info("sqlite3_tutor: extension verification succeeded", "version", version)
	return nil
}

