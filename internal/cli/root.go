package cli

import (
	"go-todo/internal/config"

	"github.com/spf13/cobra"
)

type configLoader func() (config.Config, error)

func NewRootCommand() *cobra.Command {
	var environment, configDirectory string
	root := &cobra.Command{Use: "go-todo", Short: "Go Todo 管理アプリ", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&environment, "env", "", "実行環境（APP_ENV、未指定時 development）")
	root.PersistentFlags().StringVar(&configDirectory, "config-dir", "", "外部設定ディレクトリ（未指定時 config/settings、なければ埋め込み設定）")
	load := func() (config.Config, error) {
		return config.Load(config.Options{Environment: environment, Directory: configDirectory})
	}
	root.AddCommand(
		newServeCommand(load),
		newDBCommand(load),
		newAdminCommand(load),
	)
	return root
}
