package participation

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type chancesService interface {
	MyChances(ctx context.Context, userID int64) (domain.Chances, error)
}

type chancesHandler struct {
	service chancesService
	sender  handlers.Sender
}

func NewChances(service chancesService, sender handlers.Sender) handlers.Callback {
	return chancesHandler{service: service, sender: sender}
}

func (chancesHandler) Action() string { return handlers.ActionChances }

func (h chancesHandler) Handle(ctx context.Context, userID int64) error {
	chances, err := h.service.MyChances(ctx, userID)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Твой фестивальный пропуск\nНакоплено шансов: %d из %d", chances.Earned, chances.Maximum)
	return h.sender.Send(ctx, userID, text, handlers.BackButton())
}
