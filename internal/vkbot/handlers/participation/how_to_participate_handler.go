package participation

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

func NewHowToParticipate(sender handlers.Sender) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		buttons := [][]handlers.Button{
			{{Text: "Получить промокод", Payload: handlers.ActionPromo}},
			{{Text: "Участвовать в розыгрыше", Payload: handlers.ActionRaffle}},
			{{Text: "Мои шансы", Payload: handlers.ActionChances}},
			{{Text: "В меню", Payload: handlers.ActionMenu}},
		}
		return sender.Send(ctx, userID, howToParticipateText, buttons)
	}
}

const howToParticipateText = `Как участвовать 🎟

1. Каждый день появляется партнёр дня. Нажми «Участвовать в розыгрыше», подпишись на его аккаунт и вернись в бот, чтобы проверить подписку.

2. Если у партнёра остались промокоды, можешь получить один. Промокод не обязателен для участия в ежедневном розыгрыше.

3. За день, когда ты получил промокод или зарегистрировался в розыгрыше, начисляется один шанс на главный приз. Оба действия в один день всё равно дают один шанс. Всего можно накопить до 10 шансов.

Свой результат смотри в разделе «Мои шансы».

Сейчас партнёры и промокоды тестовые, а проверка подписки демонстрационная.`
