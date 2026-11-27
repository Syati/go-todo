package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go-todo/internal/app"
	"go-todo/internal/config"

	"github.com/labstack/echo/v5"
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
	cleanupCtx, stopCleanup := context.WithCancel(ctx)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		application.RunSessionCleanup(cleanupCtx)
	}()
	defer func() {
		stopCleanup()
		<-cleanupDone
	}()
	address := cfg.Addr
	startConfig := echo.StartConfig{
		Address:         address,
		HideBanner:      true,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(server *http.Server) error {
			server.ReadHeaderTimeout = 5 * time.Second
			server.ReadTimeout = 15 * time.Second
			server.WriteTimeout = 30 * time.Second
			server.IdleTimeout = 60 * time.Second
			return nil
		},
	}
	slog.Info("listening", "address", address)
	err = startConfig.Start(ctx, application.Echo())
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
