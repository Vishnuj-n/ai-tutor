//go:build !cgo

package db

import (
	"database/sql"
	"fmt"
)

func loadExtension(db *sql.DB, extensionPath string) error {
	return fmt.Errorf("sqlite extensions cannot be loaded because CGO is disabled")
}
