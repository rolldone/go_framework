package db

import (
	"context"

	"gorm.io/gorm"
)

// WithTransaction runs the supplied function inside a database transaction.
// It begins a transaction, injects the provided context into the tx via
// tx = tx.WithContext(ctx), and ensures rollback on error or panic.
// On success it commits the transaction.
func WithTransaction(ctx context.Context, gdb *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := gdb.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// ensure we rollback on panic
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	tx = tx.WithContext(ctx)

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}
