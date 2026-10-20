package vkbot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

func TestHandlerRoutesCallbackScenarios(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		action      string
		wantMessage string
		wantButtons int
	}{
		{name: "menu", action: handlers.ActionMenu, wantMessage: "Добро пожаловать", wantButtons: 5},
		{name: "promo", action: handlers.ActionPromo, wantMessage: "Раздел пока не реализован", wantButtons: 1},
		{name: "raffle", action: handlers.ActionRaffle, wantMessage: "демо-режиме", wantButtons: 2},
		{name: "subscription", action: handlers.ActionSubscriptionCheck, wantMessage: "Демо-проверка пройдена", wantButtons: 1},
		{name: "how to participate", action: handlers.ActionHowToParticipate, wantMessage: "Как участвовать", wantButtons: 4},
		{name: "chances", action: handlers.ActionChances, wantMessage: "3 из 10", wantButtons: 1},
		{name: "rules", action: handlers.ActionRules, wantMessage: "Раздел пока не реализован", wantButtons: 1},
		{name: "unknown", action: "missing:action", wantMessage: "Неизвестное действие", wantButtons: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			messenger := &recordingMessenger{}
			service := &scenarioTestService{}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewHandler(service, messenger, log)
			payload, err := json.Marshal(struct {
				Action string `json:"action"`
			}{Action: tt.action})
			if err != nil {
				t.Fatal(err)
			}
			update := Update{Type: "message_event"}
			update.Object.UserID = 42
			update.Object.PeerID = 42
			update.Object.EventID = "event-1"
			update.Object.Payload = payload
			if err := handler.Handle(context.Background(), update); err != nil {
				t.Fatal(err)
			}
			if messenger.acked != 1 || len(messenger.sent) != 1 {
				t.Fatalf("acked=%d, messages=%+v", messenger.acked, messenger.sent)
			}
			message := messenger.sent[0]
			if message.peerID != 42 || !strings.Contains(message.text, tt.wantMessage) || len(message.buttons) != tt.wantButtons {
				t.Errorf("message=%+v, want text containing %q with %d button rows", message, tt.wantMessage, tt.wantButtons)
			}
		})
	}
}

type scenarioTestService struct{}

func (*scenarioTestService) Start(context.Context, int64) error { return nil }

func (*scenarioTestService) MyChances(context.Context, int64) (domain.Chances, error) {
	return domain.Chances{Earned: 3, Maximum: domain.MaxChances}, nil
}

func (*scenarioTestService) CheckSubscription(context.Context, int64, string) (bool, error) {
	return true, nil
}

type recordedMessage struct {
	peerID  int64
	text    string
	buttons [][]Button
}

type recordingMessenger struct {
	acked int
	sent  []recordedMessage
}

func (m *recordingMessenger) Send(_ context.Context, peerID int64, text string, buttons [][]Button) error {
	m.sent = append(m.sent, recordedMessage{peerID: peerID, text: text, buttons: buttons})
	return nil
}

func (m *recordingMessenger) Ack(context.Context, string, int64, int64) error {
	m.acked++
	return nil
}
