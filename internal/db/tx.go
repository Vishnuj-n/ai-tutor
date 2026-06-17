package db

import (
	"database/sql"
	"fmt"
)

// withTx is a transaction helper that handles Begin, Rollback (on error or panic), and Commit.
// It uses recover() for panic guarding to ensure proper rollback if a panic occurs during
// the transaction in the same goroutine (e.g., from a nil pointer dereference or index out of bounds).
// Panics from spawned/background goroutines must recover themselves; this defer will not catch them.
func (r *Repository) withTx(fn func(*sql.Tx) error) error {
	tx, err := r.db.Begin()

	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Track whether commit succeeded
	committed := false
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r) // Re-panic after rollback
		} else if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	committed = true
	return nil
}
