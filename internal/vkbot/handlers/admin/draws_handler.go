package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type DrawsService interface {
	PendingDraws(ctx context.Context) ([]domain.PendingDraw, error)
	ApproveDraw(ctx context.Context, drawID, adminVKID int64) (domain.ApprovalOutcome, error)
}

func NewPendingDraws(service DrawsService, sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, adminVKID int64) error {
		draws, err := service.PendingDraws(ctx)
		if err != nil {
			return err
		}
		if len(draws) == 0 {
			return sender.Send(ctx, adminVKID, "Нет розыгрышей, ожидающих подтверждения.", handlers.BackButton())
		}
		for _, draw := range draws {
			label := "Финальный розыгрыш"
			if draw.Kind == "daily" {
				label = fmt.Sprintf("Розыгрыш %s — %s", draw.Date, draw.PartnerName)
			}
			winners := draw.WinnerIDs
			if winners == "" {
				winners = "нет"
			}
			reserves := draw.ReserveIDs
			if reserves == "" {
				reserves = "нет"
			}
			message := fmt.Sprintf("%s (#%d)\nБилетов в розыгрыше: %d\nПобедители (VK ID): %s\nРезерв (VK ID): %s\n\nПодтвердить результаты и отправить уведомления победителям?",
				label, draw.ID, draw.Tickets, winners, reserves)
			buttons := [][]handlers.Button{{{Text: "Подтвердить результаты", Payload: handlers.ActionDrawApprovePrefix + strconv.FormatInt(draw.ID, 10)}}}
			if err := sender.Send(ctx, adminVKID, message, buttons); err != nil {
				return err
			}
		}
		return nil
	}
}

func NewApproveDraw(service DrawsService, sender handlers.Sender, drawID int64) handlers.Func {
	return func(ctx context.Context, adminVKID int64) error {
		outcome, err := service.ApproveDraw(ctx, drawID, adminVKID)
		if err != nil {
			return err
		}
		switch outcome {
		case domain.DrawApproved:
			return sender.Send(ctx, adminVKID, fmt.Sprintf("Розыгрыш #%d подтверждён. Уведомления победителям поставлены в очередь.", drawID), handlers.BackButton())
		case domain.DrawAlreadyApproved:
			return sender.Send(ctx, adminVKID, fmt.Sprintf("Розыгрыш #%d уже был подтверждён.", drawID), handlers.BackButton())
		case domain.DrawNotFound:
			return sender.Send(ctx, adminVKID, "Розыгрыш не найден.", handlers.BackButton())
		default:
			return fmt.Errorf("unknown draw approval outcome: %q", outcome)
		}
	}
}
