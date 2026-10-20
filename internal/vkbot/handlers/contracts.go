package handlers

import "context"

type Sender interface {
	Send(ctx context.Context, peerID int64, text string, buttons [][]Button) error
}

type Scenario interface {
	Handle(ctx context.Context, userID int64) error
}

type Callback interface {
	Scenario
	Action() string
}
