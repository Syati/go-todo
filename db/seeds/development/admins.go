package development

import (
	"context"
	"database/sql"
	"errors"

	"go-todo/internal/store"

	"golang.org/x/crypto/bcrypt"
)

func seedAdmins(ctx context.Context, queries *store.Queries, data adminData) error {
	for _, admin := range data.Admins {
		if err := seedAdmin(ctx, queries, admin, data.Password); err != nil {
			return err
		}
	}
	return nil
}

func seedAdmin(ctx context.Context, queries *store.Queries, admin adminSeed, password string) error {
	_, err := queries.FindSeedAdmin(ctx, admin.Email)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return queries.CreateSeedAdmin(ctx, store.CreateSeedAdminParams{
		Email: admin.Email, PasswordHash: string(hash), Role: admin.Role, Deleted: admin.Deleted,
	})
}
