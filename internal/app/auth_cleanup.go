package app

import (
	"context"
	"log/slog"
	"time"
)

// RunAuthCleanup removes expired sessions and login attempts at startup and hourly.
// It returns when the server context is canceled.
func (a *App) RunAuthCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for ctx.Err() == nil {
		a.cleanExpiredAuthRecords(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) cleanExpiredAuthRecords(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := a.queries.DeleteExpiredSessions(cleanupCtx); err != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "expired session cleanup failed", "error", err)
	}
	if err := a.queries.DeleteExpiredLoginAttempts(cleanupCtx); err != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "expired login attempt cleanup failed", "error", err)
	}
}
