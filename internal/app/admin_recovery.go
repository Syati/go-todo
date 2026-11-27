package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"go-todo/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// RecoverSoleSuperAdmin replaces the sole active super admin's password,
// revokes their sessions, and records the operator reference in one transaction.
func (a *App) RecoverSoleSuperAdmin(ctx context.Context, email, password, operatorRef string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	operatorRef = strings.TrimSpace(operatorRef)
	if err := validateEmail(email); err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if n := utf8.RuneCountInString(operatorRef); n < 1 || n > 200 {
		return errors.New("復旧担当者の記録を1〜200文字で指定してください")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := a.queries.WithTx(tx)
	if err := q.LockAdmins(ctx); err != nil {
		return err
	}
	id, err := q.RecoverSoleSuperAdmin(ctx, store.RecoverSoleSuperAdminParams{Email: email, PasswordHash: string(hash)})
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("指定先が唯一の有効な特権管理者である場合に復旧できます")
	}
	if err != nil {
		return err
	}
	if err := q.DeleteAdminSessions(ctx, id); err != nil {
		return err
	}
	if err := q.ResetRecoveredAdminAttempts(ctx, tokenHash(email)); err != nil {
		return err
	}
	if err := q.RecordAdminRecovery(ctx, store.RecordAdminRecoveryParams{AdminID: id, OperatorRef: operatorRef}); err != nil {
		return err
	}
	return tx.Commit()
}
