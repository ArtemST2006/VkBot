package participation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type subscriptionService interface {
	CheckSubscription(ctx context.Context, userID int64, partnerID string) (bool, error)
}

type subscriptionHandler struct {
	checker subscriptionService
	sender  handlers.Sender
}

func NewSubscription(checker subscriptionService, sender handlers.Sender) handlers.Callback {
	return subscriptionHandler{checker: checker, sender: sender}
}

func (subscriptionHandler) Action() string { return handlers.ActionSubscriptionCheck }

func (h subscriptionHandler) Handle(ctx context.Context, userID int64) error {
	ok, err := h.checker.CheckSubscription(ctx, userID, "demo")
	if err != nil {
		return err
	}
	if ok {
		return h.sender.Send(ctx, userID, "Демо-проверка пройдена. Заявка пока не создаётся.", handlers.BackButton())
	}
	return h.sender.Send(ctx, userID, "Подписка не найдена.", handlers.BackButton())
}
