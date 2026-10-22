package participation

import (
	"context"
	"errors"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type subscriptionService interface {
	JoinRaffle(ctx context.Context, userID int64) (domain.RaffleResult, error)
}

func NewSubscription(service subscriptionService, sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		result, err := service.JoinRaffle(ctx, userID)
		if errors.Is(err, domain.ErrUserNotStarted) {
			return sender.Send(ctx, userID, "Сначала отправь «Начать», чтобы запустить бота.", nil)
		}
		if err != nil {
			return err
		}
		switch result.Outcome {
		case domain.RaffleJoined:
			message := fmt.Sprintf("Ты в лайнапе ✅\n\nЗаявка на розыгрыш призов от %s зарегистрирована.\nНакоплено шансов: %d из %d.\n\nВозвращайся завтра — на сцену выйдет новый партнёр.",
				result.PartnerName, result.Chances, domain.MaxChances)
			return sender.Send(ctx, userID, message, handlers.BackButton())
		case domain.RaffleAlreadyJoined:
			message := fmt.Sprintf("Ты уже участвуешь в сегодняшнем розыгрыше ✅\nЗаявка уже зарегистрирована.\nНакоплено шансов: %d из %d.", result.Chances, domain.MaxChances)
			return sender.Send(ctx, userID, message, handlers.BackButton())
		case domain.RaffleUnsubscribed:
			message := fmt.Sprintf("Пока не вижу подписку 👀\n\nПодпишись на аккаунт %s: %s\nВернись в бот и попробуй ещё раз.", result.PartnerName, result.AccountURL)
			buttons := [][]handlers.Button{
				{{Text: "Проверить ещё раз", Payload: handlers.ActionSubscriptionCheck}},
				{{Text: "В меню", Payload: handlers.ActionMenu}},
			}
			return sender.Send(ctx, userID, message, buttons)
		case domain.RaffleUnavailable:
			return sender.Send(ctx, userID, "Сегодня розыгрыш пока недоступен. Попробуй позже.", handlers.BackButton())
		default:
			return fmt.Errorf("unknown raffle outcome: %q", result.Outcome)
		}
	}
}
