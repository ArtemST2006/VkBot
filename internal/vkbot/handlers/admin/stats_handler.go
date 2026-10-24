package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type StatsService interface {
	Stats(ctx context.Context) (domain.Stats, error)
}

func NewStats(service StatsService, sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		stats, err := service.Stats(ctx)
		if err != nil {
			return err
		}
		return sender.Send(ctx, userID, FormatStats(stats), handlers.BackButton())
	}
}

func FormatStats(stats domain.Stats) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Статистика акции\nЗапустили бота: %d\nЗапросы промокода: %d\nВыдано промокодов: %d\n", stats.UsersStarted, stats.PromoRequests, stats.PromosIssued)
	out.WriteString("\nНовые пользователи по дням акции (МСК):\n")
	for _, day := range stats.NewUsers {
		fmt.Fprintf(&out, "%s: %d\n", day.Date.Format("02.01"), day.Count)
	}
	out.WriteString("\nПартнёры по дням:\n")
	for _, partner := range stats.Partners {
		fmt.Fprintf(&out, "%s — %s: промокодов осталось %d, участников розыгрыша %d\n",
			partner.Date.Format("02.01"), partner.PartnerName, partner.PromosLeft, partner.RaffleEntrants)
	}
	return strings.TrimSpace(out.String())
}
