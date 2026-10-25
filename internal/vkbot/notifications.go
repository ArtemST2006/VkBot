package vkbot

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/lib/logger"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type NotificationStore interface {
	NextNotification(context.Context) (*domain.Notification, error)
	MarkNotificationSent(context.Context, domain.Notification) error
	MarkNotificationFailed(context.Context, domain.Notification) error
	ExpirePrizes(context.Context) error
}

// RunNotifications delivers winner messages.
func RunNotifications(ctx context.Context, store NotificationStore, client *Client, log *slog.Logger) {
	for ctx.Err() == nil {
		if err := store.ExpirePrizes(ctx); err != nil && ctx.Err() == nil {
			log.WarnContext(ctx, "expire prizes", logger.Err(err))
		}
		item, err := store.NextNotification(ctx)
		if err != nil && ctx.Err() == nil {
			log.WarnContext(ctx, "read notification queue", logger.Err(err))
		}
		if item != nil {
			var buttons [][]handlers.Button
			if item.Action != "" {
				buttons = [][]handlers.Button{{{Text: "Получить приз", Payload: item.Action}}}
			}
			if err := client.SendQueued(ctx, item.UserID, item.Message, buttons, item.ID); err != nil {
				if ctx.Err() == nil {
					log.WarnContext(ctx, "send notification", "id", item.ID, logger.Err(err))
					if markErr := store.MarkNotificationFailed(ctx, *item); markErr != nil {
						log.WarnContext(ctx, "save notification failure", "id", item.ID, logger.Err(markErr))
					}
				}
			} else if err := store.MarkNotificationSent(ctx, *item); err != nil && ctx.Err() == nil {
				log.WarnContext(ctx, "mark notification sent", "id", item.ID, logger.Err(err))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
