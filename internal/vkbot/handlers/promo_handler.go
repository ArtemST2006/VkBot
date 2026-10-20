package handlers

import "context"

type promoHandler struct {
	sender Sender
}

func NewPromo(sender Sender) Callback { return promoHandler{sender: sender} }

func (promoHandler) Action() string { return ActionPromo }

func (h promoHandler) Handle(ctx context.Context, userID int64) error {
	return h.sender.Send(ctx, userID, "Раздел пока не реализован.", BackButton())
}
