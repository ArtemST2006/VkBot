package promo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type PromoIssuer interface {
	IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error)
}

type promoHandler struct {
	issuer PromoIssuer
	sender handlers.Sender
}

func NewPromo(issuer PromoIssuer, sender handlers.Sender) handlers.Callback {
	return promoHandler{issuer: issuer, sender: sender}
}

func (promoHandler) Action() string { return handlers.ActionPromo }

func (h promoHandler) Handle(ctx context.Context, userID int64) error {
	result, err := h.issuer.IssuePromo(ctx, userID)
	if errors.Is(err, domain.ErrUserNotStarted) {
		return h.sender.Send(ctx, userID, "Сначала отправь «Начать», чтобы запустить бота.", nil)
	}
	if err != nil {
		return err
	}
	buttons := [][]handlers.Button{
		{{Text: "Участвовать в розыгрыше", Payload: handlers.ActionRaffle}},
		{{Text: "В меню", Payload: handlers.ActionMenu}},
	}
	switch result.Outcome {
	case domain.PromoIssued, domain.PromoAlreadyIssued:
		moscow, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			return fmt.Errorf("load Moscow time zone: %w", err)
		}
		intro := "Твой фестивальный бонус от " + result.PartnerName + " готов 🎁"
		if result.Outcome == domain.PromoAlreadyIssued {
			intro = "Твой промокод от " + result.PartnerName + " уже закреплён за тобой 🎁"
		}
		message := fmt.Sprintf("%s\n\nПромокод: %s\n\nИспользуй его до %s. %s\n\nТеперь можешь принять участие в розыгрыше призов дня.",
			intro, result.Code, result.ValidUntil.In(moscow).Format("02.01.2006 15:04 МСК"), result.Instructions)
		return h.sender.Send(ctx, userID, message, buttons)
	case domain.PromoExhausted:
		message := fmt.Sprintf("Промокоды от %s уже закончились — их разобрали быстрее всех 🔥\n\nНо ты всё ещё можешь принять участие в розыгрыше призов дня.", result.PartnerName)
		return h.sender.Send(ctx, userID, message, buttons)
	case domain.PromoUnavailable:
		return h.sender.Send(ctx, userID, "Сегодня промокоды пока недоступны. Попробуй позже.", handlers.BackButton())
	default:
		return fmt.Errorf("unknown promo outcome: %q", result.Outcome)
	}
}
