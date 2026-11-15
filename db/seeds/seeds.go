package seeds

import (
	"context"
	"database/sql"

	"go-todo/db/seeds/development"
)

// Run dispatches environment-specific seeds, matching rails8-todo/db/seeds.rb.
// An environment without seeds leaves the database unchanged.
func Run(ctx context.Context, db *sql.DB, environment string) (bool, error) {
	switch environment {
	case "development":
		return true, development.Run(ctx, db)
	default:
		return false, nil
	}
}
