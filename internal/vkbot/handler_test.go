package vkbot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

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
		{name: "promo", action: handlers.ActionPromo, wantMessage: "Промокод: DEMO-01", wantButtons: 2},
		{name: "raffle", action: handlers.ActionRaffle, wantMessage: "Розыгрыш призов от Демо-партнёр", wantButtons: 2},
		{name: "subscription", action: handlers.ActionSubscriptionCheck, wantMessage: "Заявка на розыгрыш призов", wantButtons: 1},
		{name: "how to participate", action: handlers.ActionHowToParticipate, wantMessage: "Как участвовать", wantButtons: 4},
		{name: "chances", action: handlers.ActionChances, wantMessage: "3 из 10", wantButtons: 1},
		{name: "rules", action: handlers.ActionRules, wantMessage: "Правила акции", wantButtons: 1},
		{name: "unknown", action: "missing:action", wantMessage: "Неизвестное действие", wantButtons: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			messenger := &recordingMessenger{}
			service := &scenarioTestService{promoResult: domain.PromoResult{
				Outcome: domain.PromoIssued, PartnerName: "Демо-партнёр", Code: "DEMO-01",
				ValidUntil: time.Date(2026, time.October, 20, 20, 59, 0, 0, time.UTC),
			}, partner: &domain.PartnerDay{Name: "Демо-партнёр", AccountURL: "https://example.com/partner", PrizeName: "Тестовый приз"},
				raffleResult: domain.RaffleResult{Outcome: domain.RaffleJoined, PartnerName: "Демо-партнёр", Chances: 1}}
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

func TestStartCreatesUserOnlyAfterStartMessage(t *testing.T) {
	service := &scenarioTestService{}
	messenger := &recordingMessenger{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(service, messenger, log)

	var update Update
	if err := json.Unmarshal([]byte(`{"type":"message_new","object":{"message":{"from_id":42,"peer_id":42,"text":"Привет"}}}`), &update); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if service.startCalls != 0 {
		t.Fatalf("Start called before the start message: %d", service.startCalls)
	}

	update.Object.Message.Text = "Начать"
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if service.startCalls != 1 {
		t.Fatalf("Start calls=%d, want 1", service.startCalls)
	}
	if messenger.welcomeCalls != 1 {
		t.Fatalf("welcome calls=%d, want 1", messenger.welcomeCalls)
	}
}

func TestInformationSectionsShowCampaignDetails(t *testing.T) {
	messenger := &recordingMessenger{}
	handler := NewHandler(&scenarioTestService{}, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, action := range []string{handlers.ActionChances, handlers.ActionRules} {
		payload, err := json.Marshal(map[string]string{"action": action})
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
	}
	if len(messenger.sent) != 2 {
		t.Fatalf("messages=%+v", messenger.sent)
	}
	for _, fragment := range []string{"Пройдено дней акции: 4 из 10", "Осталось дней (включая сегодня): 6", "20.10.2026 — Демо-партнёр", domain.GrandPrize} {
		if !strings.Contains(messenger.sent[0].text, fragment) {
			t.Errorf("chances message does not contain %q: %s", fragment, messenger.sent[0].text)
		}
	}
	for _, fragment := range []string{"Правила акции", "Максимум — 10 шансов", "демонстрационная версия"} {
		if !strings.Contains(messenger.sent[1].text, fragment) {
			t.Errorf("rules message does not contain %q: %s", fragment, messenger.sent[1].text)
		}
	}
}

func TestPromoCallbackOtherOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name     string
		outcome  domain.PromoOutcome
		wantText string
		wantRows int
	}{
		{name: "repeat", outcome: domain.PromoAlreadyIssued, wantText: "уже закреплён", wantRows: 2},
		{name: "exhausted", outcome: domain.PromoExhausted, wantText: "уже закончились", wantRows: 2},
		{name: "unavailable", outcome: domain.PromoUnavailable, wantText: "пока недоступны", wantRows: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &scenarioTestService{promoResult: domain.PromoResult{
				Outcome: tt.outcome, PartnerName: "Демо-партнёр", Code: "DEMO-01",
				ValidUntil: time.Date(2026, time.October, 20, 20, 59, 0, 0, time.UTC),
			}}
			messenger := &recordingMessenger{}
			handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)))
			var update Update
			if err := json.Unmarshal([]byte(`{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-1","payload":{"action":"promo:get"}}}`), &update); err != nil {
				t.Fatal(err)
			}
			if err := handler.Handle(context.Background(), update); err != nil {
				t.Fatal(err)
			}
			if messenger.acked != 1 || len(messenger.sent) != 1 ||
				!strings.Contains(messenger.sent[0].text, tt.wantText) || len(messenger.sent[0].buttons) != tt.wantRows {
				t.Fatalf("ack=%d, sent=%+v", messenger.acked, messenger.sent)
			}
		})
	}
}

func TestPromoCallbackBeforeStart(t *testing.T) {
	service := &scenarioTestService{promoErr: domain.ErrUserNotStarted}
	messenger := &recordingMessenger{}
	handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var update Update
	if err := json.Unmarshal([]byte(`{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-1","payload":{"action":"promo:get"}}}`), &update); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if messenger.acked != 1 || len(messenger.sent) != 1 || !strings.Contains(messenger.sent[0].text, "Начать") {
		t.Fatalf("ack=%d, sent=%+v", messenger.acked, messenger.sent)
	}
}

func TestRaffleCallbackOtherOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name     string
		result   domain.RaffleResult
		wantText string
		wantRows int
	}{
		{name: "repeat", result: domain.RaffleResult{Outcome: domain.RaffleAlreadyJoined, Chances: 1}, wantText: "уже участвуешь", wantRows: 1},
		{name: "unsubscribed", result: domain.RaffleResult{Outcome: domain.RaffleUnsubscribed, PartnerName: "Демо-партнёр", AccountURL: "https://example.com/partner"}, wantText: "Пока не вижу подписку", wantRows: 2},
		{name: "unavailable", result: domain.RaffleResult{Outcome: domain.RaffleUnavailable}, wantText: "розыгрыш пока недоступен", wantRows: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &scenarioTestService{raffleResult: tt.result}
			messenger := &recordingMessenger{}
			handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)))
			var update Update
			if err := json.Unmarshal([]byte(`{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-1","payload":{"action":"subscription:check"}}}`), &update); err != nil {
				t.Fatal(err)
			}
			if err := handler.Handle(context.Background(), update); err != nil {
				t.Fatal(err)
			}
			if messenger.acked != 1 || len(messenger.sent) != 1 ||
				!strings.Contains(messenger.sent[0].text, tt.wantText) || len(messenger.sent[0].buttons) != tt.wantRows {
				t.Fatalf("ack=%d, sent=%+v", messenger.acked, messenger.sent)
			}
		})
	}
}

type scenarioTestService struct {
	startCalls   int
	promoResult  domain.PromoResult
	promoErr     error
	partner      *domain.PartnerDay
	raffleResult domain.RaffleResult
}

func (s *scenarioTestService) Start(context.Context, int64) error {
	s.startCalls++
	return nil
}

func (*scenarioTestService) MyChances(context.Context, int64) (domain.Chances, error) {
	return domain.Chances{
		Earned: 3, Maximum: domain.MaxChances, PassedDays: 4, RemainingDays: 6,
		ActiveDays: []domain.ActiveDay{{Date: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC), PartnerName: "Демо-партнёр"}},
	}, nil
}

func (s *scenarioTestService) IssuePromo(context.Context, int64) (domain.PromoResult, error) {
	return s.promoResult, s.promoErr
}

func (s *scenarioTestService) TodayPartner(context.Context) (*domain.PartnerDay, error) {
	return s.partner, nil
}

func (s *scenarioTestService) JoinRaffle(context.Context, int64) (domain.RaffleResult, error) {
	return s.raffleResult, nil
}

type recordedMessage struct {
	peerID  int64
	text    string
	buttons [][]Button
}

type recordingMessenger struct {
	acked        int
	welcomeCalls int
	sent         []recordedMessage
}

func (m *recordingMessenger) Send(_ context.Context, peerID int64, text string, buttons [][]Button) error {
	m.sent = append(m.sent, recordedMessage{peerID: peerID, text: text, buttons: buttons})
	return nil
}

func (m *recordingMessenger) SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]Button) error {
	m.welcomeCalls++
	return m.Send(ctx, peerID, text, buttons)
}

func (m *recordingMessenger) Ack(context.Context, string, int64, int64) error {
	m.acked++
	return nil
}
