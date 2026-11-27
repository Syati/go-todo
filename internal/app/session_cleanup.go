package app

import (
	"context"
	"log/slog"
	"time"
)

// RunSessionCleanup removes expired sessions on startup and at a fixed interval.
// It returns when the server context is canceled.
func (a *App) RunSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for ctx.Err() == nil {
		a.cleanExpiredSessions(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) cleanExpiredSessions(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := a.queries.DeleteExpiredSessions(cleanupCtx); err != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "expired session cleanup failed", "error", err)
	}
}
