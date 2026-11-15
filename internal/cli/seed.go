package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go-todo/db/seeds"

	"github.com/spf13/cobra"
)

func newSeedCommand(load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "環境別の初期データを登録",
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
			applied, err := seeds.Run(ctx, conn, cfg.Environment)
			if err != nil {
				return err
			}
			if !applied {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s の seed は未定義です（変更なし）。\n", cfg.Environment)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s seed 完了（既存データは変更しません）。\n", cfg.Environment)
			return err
		},
	}
}
