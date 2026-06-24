//go:build cgo

package db

import (
	"database/sql"
	"fmt"
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



