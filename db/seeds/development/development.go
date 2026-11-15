package development

import (
	"context"
	"database/sql"

	"go-todo/internal/store"
)

// Run loads seeds in dependency order, as in rails8-todo's development entry.
func Run(ctx context.Context, db *sql.DB) error {
	data, err := loadData(files)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	queries := store.New(db).WithTx(tx)
	// Products have no unique name constraint; serialize concurrent seed runs.
	if err := queries.LockSeeds(ctx); err != nil {
		return err
	}
	if err := seedAdmins(ctx, queries, data.Admins); err != nil {
		return err
	}
	if err := seedProducts(ctx, queries, data.Products); err != nil {
		return err
	}
	return tx.Commit()
}
