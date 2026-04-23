//go:build cgo

package db

import (
	"context"
	"database/sql"
	"fmt"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func loadExtension(db *sql.DB, extensionPath string) error {
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
			if loadErr := sqliteConn.LoadExtension(extensionPath, entry); loadErr == nil {
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
