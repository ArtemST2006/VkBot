package navigation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type unknownMessageHandler struct {
	sender handlers.Sender
}

func NewUnknownMessage(sender handlers.Sender) handlers.Scenario {
	return unknownMessageHandler{sender: sender}
}

func (h unknownMessageHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, "Используй кнопки меню ниже.", handlers.MenuButtons())
}

type unknownActionHandler struct {
	sender handlers.Sender
}

func NewUnknownAction(sender handlers.Sender) handlers.Scenario {
	return unknownActionHandler{sender: sender}
}

func (h unknownActionHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, "Неизвестное действие. Открой меню.", handlers.BackButton())
}
