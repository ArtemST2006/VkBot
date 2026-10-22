package navigation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

func NewUnknownMessage(sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		return sender.Send(ctx, userID, "Используй кнопки меню ниже.", handlers.MenuButtons())
	}
}

func NewUnknownAction(sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		return sender.Send(ctx, userID, "Неизвестное действие. Открой меню.", handlers.BackButton())
	}
}
