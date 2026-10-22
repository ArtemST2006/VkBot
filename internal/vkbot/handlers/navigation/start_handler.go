package navigation

import (
	"context"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type startService interface {
	Start(ctx context.Context, userID int64) error
}

type welcomeSender interface {
	SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]handlers.Button) error
}

func IsStart(message string) bool {
	return strings.EqualFold(message, "Начать") || strings.EqualFold(message, "Старт") || strings.HasPrefix(message, "/start")
}

func NewStart(service startService, sender welcomeSender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		if err := service.Start(ctx, userID); err != nil {
			return err
		}
		return sender.SendWelcome(ctx, userID, welcomeText, handlers.MenuButtons())
	}
}
