package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go-todo/internal/app"

	"github.com/spf13/cobra"
)

func newAdminCommand(load configLoader) *cobra.Command {
	admin := &cobra.Command{Use: "admin", Short: "管理者の初期設定と復旧"}
	admin.AddCommand(newBootstrapCommand(load), newAdminRecoveryCommand(load))
	return admin
}

func newBootstrapCommand(load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "空のDBに特権管理者を作成",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			cfg, err := load()
			if err != nil {
				return err
			}
			if cfg.BootstrapEmail == "" {
				return errors.New("BOOTSTRAP_EMAIL と BOOTSTRAP_PASSWORD を指定してください")
			}
			application, err := app.Open(cmd.Context(), cfg.DatabaseURL, cfg.CookieSecure)
			if err != nil {
				return err
			}
			defer func() { runErr = errors.Join(runErr, application.Close()) }()
			return application.Bootstrap(cmd.Context(), cfg.BootstrapEmail, cfg.BootstrapPassword)
		},
	}
}

func newAdminRecoveryCommand(load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "recover EMAIL OPERATOR_REF",
		Short: "唯一の有効な特権管理者のパスワードを復旧",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			password := os.Getenv("RECOVERY_PASSWORD")
			if password == "" {
				return errors.New("RECOVERY_PASSWORD を指定してください")
			}
			cfg, err := load()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			application, err := app.Open(ctx, cfg.DatabaseURL, cfg.CookieSecure)
			if err != nil {
				return err
			}
			defer func() { runErr = errors.Join(runErr, application.Close()) }()
			if err := application.RecoverSoleSuperAdmin(ctx, args[0], password, args[1]); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "パスワードを更新し既存セッションを失効しました")
			return err
		},
	}
}
