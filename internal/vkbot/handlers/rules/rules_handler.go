package rules

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type rulesHandler struct {
	sender handlers.Sender
}

func NewRules(sender handlers.Sender) handlers.Callback { return rulesHandler{sender: sender} }

func (rulesHandler) Action() string { return handlers.ActionRules }

func (h rulesHandler) Handle(ctx context.Context, userID int64) error {
	text := fmt.Sprintf(`Правила акции 🎟

• Каждый день акции посвящён одному партнёру. Его приз и аккаунт показаны в разделе «Участвовать в розыгрыше».
• Для участия в розыгрыше дня нужно подписаться на партнёра и проверить подписку в боте. На одного человека — одна заявка в день.
• Промокод можно получить, пока коды есть в наличии. Повторное нажатие в тот же день покажет тот же код. Промокод не обязателен для участия в розыгрыше дня.
• Получение промокода или регистрация в розыгрыше дают один шанс на главный приз за этот день. Оба действия в один день всё равно дают один шанс. Максимум — %d шансов.
• По условиям ТЗ главный приз — %s; его получат %d победителей. Розыгрыши дня и финальный выбор победителей должны проводиться случайным образом среди участников.

Сейчас это демонстрационная версия: партнёры и промокоды тестовые, проверка подписки условная, розыгрыши призов и уведомления победителей ещё не запущены.`,
		domain.MaxChances, domain.GrandPrize, domain.FinalWinnerCount)
	return h.sender.Send(ctx, userID, text, handlers.BackButton())
}
