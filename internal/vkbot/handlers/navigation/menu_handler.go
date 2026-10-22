package navigation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

const welcomeText = "Добро пожаловать в Бот розыгрышей"

func NewMenu(sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		return sender.Send(ctx, userID, welcomeText, handlers.MenuButtons())
	}
}
