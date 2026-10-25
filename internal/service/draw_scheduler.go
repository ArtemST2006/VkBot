package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/lib/logger"
)

type DrawRepository interface {
	DueDraws(context.Context) ([]time.Time, bool, error)
	DrawDaily(context.Context, time.Time) (domain.DrawSummary, error)
	DrawFinal(context.Context) (domain.DrawSummary, error)
}

// RunDrawScheduler catches up after restarts and checks for new draws every minute.
func RunDrawScheduler(ctx context.Context, repo DrawRepository, log *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := runDueDraws(ctx, repo, log); err != nil && ctx.Err() == nil {
			log.WarnContext(ctx, "automatic draw failed", logger.Err(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runDueDraws(ctx context.Context, repo DrawRepository, log *slog.Logger) error {
	days, final, err := repo.DueDraws(ctx)
	if err != nil {
		return err
	}
	for _, day := range days {
		draw, err := repo.DrawDaily(ctx, day)
		if err != nil {
			return err
		}
		log.InfoContext(ctx, "daily draw created", "id", draw.ID, "day", day.Format("2006-01-02"))
	}
	if final {
		draw, err := repo.DrawFinal(ctx)
		if err != nil {
			return err
		}
		log.InfoContext(ctx, "final draw created", "id", draw.ID)
	}
	return nil
}
