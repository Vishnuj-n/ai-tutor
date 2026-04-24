//go:build !cgo

package db

import (
	"fmt"
)

func loadExtension(db *sql.DB, extensionPath string) error {
	return fmt.Errorf("sqlite-vec extension loading requires CGO_ENABLED=1")
}
