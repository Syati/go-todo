package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go-todo/internal/app"
	"go-todo/internal/config"

	"github.com/spf13/cobra"
)

func newServeCommand(load configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Web サーバーを起動",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return serve(cmd.Context(), cfg)
		},
	}
}

func serve(ctx context.Context, cfg config.Config) (runErr error) {
	application, err := app.Open(ctx, cfg.DatabaseURL, cfg.CookieSecure)
	if err != nil {
		return err
	}
	defer func() { runErr = errors.Join(runErr, application.Close()) }()
	address := cfg.Addr
	server := &http.Server{Addr: address, Handler: application.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() { slog.Info("listening", "address", address); errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		return server.Shutdown(shutdown)
	}
}
