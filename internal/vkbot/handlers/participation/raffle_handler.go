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

func NewRaffle(partners partnerService, sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		partner, err := partners.TodayPartner(ctx)
		if err != nil {
			return err
		}
		if partner == nil {
			return sender.Send(ctx, userID, "Сегодня партнёр для розыгрыша пока не назначен. Попробуй позже.", handlers.BackButton())
		}
		buttons := [][]handlers.Button{
			{{Text: "Проверить подписку (демо)", Payload: handlers.ActionSubscriptionCheck}},
			{{Text: "В меню", Payload: handlers.ActionMenu}},
		}
		message := fmt.Sprintf("Розыгрыш призов от %s 🎁\nПриз: %s\nАккаунт партнёра: %s\n\nСейчас партнёр тестовый, поэтому проверка подписки демонстрационная. Нажми «Проверить подписку», чтобы зарегистрироваться.",
			partner.Name, partner.PrizeName, partner.AccountURL)
		return sender.Send(ctx, userID, message, buttons)
	}
}
