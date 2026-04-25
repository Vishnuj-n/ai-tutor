package db

import (
	"database/sql"
	"fmt"
)

// withTx is a transaction helper that handles Begin, Rollback (on error or panic), and Commit.
// It uses recover() for panic guarding to ensure proper rollback if a panic occurs during
// the transaction (e.g., from an LLM call or math error in a background goroutine).
// nolint:unused // Infrastructure helper for future transaction consolidation
func withTx(fn func(*sql.Tx) error) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Track whether commit succeeded
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Panic recovery - ensure rollback even on panic
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r) // Re-panic after rollback
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
