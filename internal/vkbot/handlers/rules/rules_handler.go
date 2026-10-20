package rules

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type rulesHandler struct {
	sender handlers.Sender
}

func NewRules(sender handlers.Sender) handlers.Callback { return rulesHandler{sender: sender} }

func (rulesHandler) Action() string { return handlers.ActionRules }

func (h rulesHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, "Раздел пока не реализован.", handlers.BackButton())
}
