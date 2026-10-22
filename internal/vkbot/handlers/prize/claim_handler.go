package prize

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type Service interface {
	StartPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error)
	PendingPrizeName(context.Context, int64) (bool, error)
	SavePrizeName(context.Context, int64, string) (int64, domain.ClaimStatus, error)
	ConsentPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error)
}

func NewClaim(service Service, sender handlers.Sender, drawID int64) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		status, err := service.StartPrizeClaim(ctx, drawID, userID)
		if err != nil {
			return err
		}
		switch status {
		case domain.ClaimAwaitingName:
			return sender.Send(ctx, userID, "Отправь имя и фамилию одним сообщением.", nil)
		case domain.ClaimAwaitingConsent:
			return askConsent(ctx, sender, userID, drawID)
		default:
			return sender.Send(ctx, userID, claimStatusText(status), handlers.BackButton())
		}
	}
}

func NewName(service Service, sender handlers.Sender, name string) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		drawID, status, err := service.SavePrizeName(ctx, userID, name)
		if errors.Is(err, domain.ErrInvalidFullName) {
			return sender.Send(ctx, userID, "Напиши имя и фамилию двумя словами, например «Иван Иванов».", nil)
		}
		if err != nil {
			return err
		}
		if status != domain.ClaimAwaitingConsent {
			return sender.Send(ctx, userID, claimStatusText(status), handlers.BackButton())
		}
		return askConsent(ctx, sender, userID, drawID)
	}
}

func NewConsent(service Service, sender handlers.Sender, drawID int64) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		status, err := service.ConsentPrizeClaim(ctx, drawID, userID)
		if err != nil {
			return err
		}
		if status == domain.ClaimCompleted {
			return sender.Send(ctx, userID, "Согласие и контактные данные сохранены ✅ Организатор свяжется с тобой по поводу приза.", handlers.BackButton())
		}
		return sender.Send(ctx, userID, claimStatusText(status), handlers.BackButton())
	}
}

func askConsent(ctx context.Context, sender handlers.Sender, userID, drawID int64) error {
	message := "Имя сохранено. Согласен на передачу имени и фамилии партнёру или билетному оператору для выдачи приза?"
	buttons := [][]handlers.Button{{{Text: "Согласен", Payload: handlers.ActionPrizeConsentPrefix + strconv.FormatInt(drawID, 10)}}}
	return sender.Send(ctx, userID, message, buttons)
}

func claimStatusText(status domain.ClaimStatus) string {
	switch status {
	case domain.ClaimCompleted:
		return "Данные для получения приза уже сохранены ✅"
	case domain.ClaimExpired:
		return "Срок ответа на выигрыш истёк. Приз передан резервному участнику."
	case domain.ClaimOtherOpen:
		return "Сначала заверши оформление другого приза."
	case domain.ClaimAwaitingName:
		return "Сначала отправь имя и фамилию одним сообщением."
	default:
		return fmt.Sprintf("Не удалось найти доступный приз (%s).", status)
	}
}
