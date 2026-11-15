package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"go-todo/db/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

func newDBCommand(load configLoader) *cobra.Command {
	db := &cobra.Command{Use: "db", Short: "goose による migration 管理"}
	db.AddCommand(
		newMigrationCommand("up", "未適用の migration を適用", load),
		newMigrationCommand("down", "直前の migration を1件戻す（データ削除を伴う場合があります）", load),
		newMigrationCommand("status", "migration の適用状況を表示", load),
		newMigrationCreateCommand(),
		newSeedCommand(load),
	)
	return db
}

func newMigrationCommand(operation, description string, load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   operation,
		Short: description,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			cfg, err := load()
			if err != nil {
				return err
			}
			conn, err := sql.Open("pgx", cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer func() { runErr = errors.Join(runErr, conn.Close()) }()
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.Files)
			if err != nil {
				return err
			}
			switch operation {
			case "up":
				results, err := provider.Up(ctx)
				if err != nil {
					return err
				}
				for _, result := range results {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), result); err != nil {
						return err
					}
				}
			case "down":
				result, err := provider.Down(ctx)
				if err != nil {
					return err
				}
				if result != nil {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), result); err != nil {
						return err
					}
				}
			case "status":
				statuses, err := provider.Status(ctx)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Version\tState\tMigration"); err != nil {
					return err
				}
				for _, status := range statuses {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", status.Source.Version, status.State, path.Base(status.Source.Path)); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}

func newMigrationCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new NAME",
		Short: "SQL migration を作成",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goose.SetSequential(true)
			return goose.Create(nil, "db/migrations", args[0], "sql")
		},
	}
}
