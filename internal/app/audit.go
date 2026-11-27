package app

import (
	"go-todo/internal/store"

	"github.com/labstack/echo/v5"
)

// auditedMutation makes the actor available to the audit triggers only for this
// transaction. The data change and the corresponding history commit together.
func (a *App) auditedMutation(c *echo.Context, mutate func(*store.Queries) error) error {
	ctx := c.Request().Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := a.queries.WithTx(tx)
	actor := c.Get(adminContextKey).(*Admin)
	if err := q.SetAuditActor(ctx, actor.ID); err != nil {
		return err
	}
	if err := mutate(q); err != nil {
		return err
	}
	return tx.Commit()
}
