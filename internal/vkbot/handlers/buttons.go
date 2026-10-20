package handlers

type Button struct {
	Text    string
	Payload string
}

const (
	ActionMenu              = "menu"
	ActionPromo             = "promo:get"
	ActionRaffle            = "raffle:join"
	ActionSubscriptionCheck = "subscription:check"
	ActionHowToParticipate  = "how:join"
	ActionChances           = "chances:show"
	ActionRules             = "rules:show"
)

func MenuButtons() [][]Button {
	return [][]Button{
		{{Text: "Получить промокод", Payload: ActionPromo}},
		{{Text: "Участвовать в розыгрыше", Payload: ActionRaffle}},
		{{Text: "Как участвовать", Payload: ActionHowToParticipate}},
		{{Text: "Мои шансы", Payload: ActionChances}},
		{{Text: "Правила акции", Payload: ActionRules}},
	}
}

func BackButton() [][]Button {
	return [][]Button{{{Text: "В меню", Payload: ActionMenu}}}
}
