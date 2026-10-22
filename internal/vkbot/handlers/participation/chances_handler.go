package participation

import (
	"context"
	"fmt"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type chancesService interface {
	MyChances(ctx context.Context, userID int64) (domain.Chances, error)
}

func NewChances(service chancesService, sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		chances, err := service.MyChances(ctx, userID)
		if err != nil {
			return err
		}
		var text strings.Builder
		fmt.Fprintf(&text, "Твой фестивальный пропуск 🎟\nНакоплено шансов: %d из %d\nПройдено дней акции: %d из %d\nОсталось дней (включая сегодня): %d\n\n",
			chances.Earned, chances.Maximum, chances.PassedDays, chances.PassedDays+chances.RemainingDays, chances.RemainingDays)
		if len(chances.ActiveDays) == 0 {
			text.WriteString("Активных дней пока нет.\n")
		} else {
			text.WriteString("Твои активные дни и партнёры:\n")
			for _, day := range chances.ActiveDays {
				fmt.Fprintf(&text, "• %s — %s\n", day.Date.Format("02.01.2006"), day.PartnerName)
			}
		}
		fmt.Fprintf(&text, "\nГлавный приз: %s.\nЧем больше активных дней, тем выше шанс выиграть.", domain.GrandPrize)
		return sender.Send(ctx, userID, text.String(), handlers.BackButton())
	}
}
