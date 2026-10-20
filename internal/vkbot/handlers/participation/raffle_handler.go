package participation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type raffleHandler struct {
	sender handlers.Sender
}

func NewRaffle(sender handlers.Sender) handlers.Callback { return raffleHandler{sender: sender} }

func (raffleHandler) Action() string { return handlers.ActionRaffle }

func (h raffleHandler) Handle(ctx context.Context, userID int64) error {
	buttons := [][]handlers.Button{
		{{Text: "Проверить подписку (демо)", Payload: handlers.ActionSubscriptionCheck}},
		{{Text: "В меню", Payload: handlers.ActionMenu}},
	}
	return h.sender.Send(ctx, userID, "Проверка подписки пока работает в демо-режиме. Регистрация в розыгрыше ещё не подключена.", buttons)
}
