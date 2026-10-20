package navigation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

const welcomeText = "Добро пожаловать в Бот розыгрышей"

type menuHandler struct {
	sender handlers.Sender
}

func NewMenu(sender handlers.Sender) handlers.Callback { return menuHandler{sender: sender} }

func (menuHandler) Action() string { return handlers.ActionMenu }

func (h menuHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, welcomeText, handlers.MenuButtons())
}
