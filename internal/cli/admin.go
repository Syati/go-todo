package cli

import (
	"errors"

	"go-todo/internal/app"

	"github.com/spf13/cobra"
)

func newAdminCommand(load configLoader) *cobra.Command {
	admin := &cobra.Command{Use: "admin", Short: "管理者の初期設定"}
	admin.AddCommand(newBootstrapCommand(load))
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
