package handlers

type Button struct {
	Text    string
	Payload string
}

const (
	ActionMenu               = "menu"
	ActionPromo              = "promo:get"
	ActionRaffle             = "raffle:join"
	ActionHowToParticipate   = "how:join"
	ActionChances            = "chances:show"
	ActionRules              = "rules:show"
	ActionPrizeClaimPrefix   = "prize:claim:"
	ActionPrizeConsentPrefix = "prize:consent:"
	ActionDrawApprovePrefix  = "draw:approve:"
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
