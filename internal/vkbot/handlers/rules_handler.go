package handlers

import "context"

type rulesHandler struct {
	sender Sender
}

func NewRules(sender Sender) Callback { return rulesHandler{sender: sender} }

func (rulesHandler) Action() string { return ActionRules }

func (h rulesHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, "Раздел пока не реализован.", BackButton())
}
