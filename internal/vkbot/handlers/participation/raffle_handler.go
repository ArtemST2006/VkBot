package participation

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type partnerService interface {
	TodayPartner(ctx context.Context) (*domain.PartnerDay, error)
}

type raffleHandler struct {
	partners partnerService
	sender   handlers.Sender
}

func NewRaffle(partners partnerService, sender handlers.Sender) handlers.Callback {
	return raffleHandler{partners: partners, sender: sender}
}

func (raffleHandler) Action() string { return handlers.ActionRaffle }

func (h raffleHandler) Handle(ctx context.Context, userID int64) error {
	partner, err := h.partners.TodayPartner(ctx)
	if err != nil {
		return err
	}
	if partner == nil {
		return h.sender.Send(ctx, userID, "Сегодня партнёр для розыгрыша пока не назначен. Попробуй позже.", handlers.BackButton())
	}
	buttons := [][]handlers.Button{
		{{Text: "Проверить подписку (демо)", Payload: handlers.ActionSubscriptionCheck}},
		{{Text: "В меню", Payload: handlers.ActionMenu}},
	}
	message := fmt.Sprintf("Розыгрыш призов от %s 🎁\nПриз: %s\nАккаунт партнёра: %s\n\nСейчас партнёр тестовый, поэтому проверка подписки демонстрационная. Нажми «Проверить подписку», чтобы зарегистрироваться.",
		partner.Name, partner.PrizeName, partner.AccountURL)
	return h.sender.Send(ctx, userID, message, buttons)
}
