package navigation

import (
	"context"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type startService interface {
	Start(ctx context.Context, userID int64) error
}

type StartHandler struct {
	service startService
	sender  welcomeSender
}

type welcomeSender interface {
	SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]handlers.Button) error
}

func NewStart(service startService, sender welcomeSender) StartHandler {
	return StartHandler{service: service, sender: sender}
}

func (StartHandler) Matches(message string) bool {
	return strings.EqualFold(message, "Начать") || strings.EqualFold(message, "Старт") || strings.HasPrefix(message, "/start")
}

func (h StartHandler) Handle(ctx context.Context, userID int64) error {
	if err := h.service.Start(ctx, userID); err != nil {
		return err
	}
	return h.sender.SendWelcome(ctx, userID, welcomeText, handlers.MenuButtons())
}
