package handlers

import "context"

type Sender interface {
	Send(ctx context.Context, peerID int64, text string, buttons [][]Button) error
}

type Func func(ctx context.Context, userID int64) error
