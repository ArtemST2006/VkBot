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
		{name: "raffle", action: handlers.ActionRaffle, wantMessage: "Заявка на розыгрыш призов", wantButtons: 1},
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
				raffleResult: domain.RaffleResult{Outcome: domain.RaffleJoined, PartnerName: "Демо-партнёр", PrizeName: "Тестовый приз", AccountURL: "https://example.com/partner", Chances: 1}}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewHandler(service, messenger, log, 0)
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
	handler := NewHandler(service, messenger, log, 0)

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
	handler := NewHandler(&scenarioTestService{}, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
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
			handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
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
	handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
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

func TestPrizeClaimConversation(t *testing.T) {
	service := &scenarioTestService{claimStatus: domain.ClaimAwaitingName, pendingName: true}
	messenger := &recordingMessenger{}
	handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	callback := func(action string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"action": action})
		update := Update{Type: "message_event"}
		update.Object.UserID, update.Object.PeerID, update.Object.EventID, update.Object.Payload = 42, 42, "event", payload
		if err := handler.Handle(context.Background(), update); err != nil {
			t.Fatal(err)
		}
	}
	callback("prize:claim:7")
	if !strings.Contains(messenger.sent[0].text, "имя и фамилию") {
		t.Fatalf("claim prompt: %q", messenger.sent[0].text)
	}
	update := Update{Type: "message_new"}
	update.Object.Message = &struct {
		FromID int64  `json:"from_id"`
		PeerID int64  `json:"peer_id"`
		Text   string `json:"text"`
		Out    int    `json:"out"`
	}{FromID: 42, PeerID: 42, Text: "Иван Иванов"}
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if service.savedName != "Иван Иванов" || messenger.sent[1].buttons[0][0].Payload != "prize:consent:7" {
		t.Fatalf("name=%q, consent buttons=%+v", service.savedName, messenger.sent[1].buttons)
	}
	callback("prize:consent:7")
	if !strings.Contains(messenger.sent[2].text, "сохранены") {
		t.Fatalf("consent: %q", messenger.sent[2].text)
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
		{name: "unavailable", result: domain.RaffleResult{Outcome: domain.RaffleUnavailable}, wantText: "розыгрыш пока недоступен", wantRows: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &scenarioTestService{raffleResult: tt.result}
			messenger := &recordingMessenger{}
			handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
			var update Update
			if err := json.Unmarshal([]byte(`{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-1","payload":{"action":"raffle:join"}}}`), &update); err != nil {
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

func TestStatsCommandOnlyForConfiguredAdmin(t *testing.T) {
	for _, tt := range []struct {
		name       string
		adminID    int64
		userID     int64
		peerID     int64
		wantStats  bool
		wantNotice string
	}{
		{name: "admin", adminID: 42, userID: 42, peerID: 42, wantStats: true, wantNotice: "Запросы промокода: 3"},
		{name: "other user", adminID: 42, userID: 43, peerID: 43, wantNotice: "Используй кнопки меню"},
		{name: "disabled", userID: 42, peerID: 42, wantNotice: "Используй кнопки меню"},
		{name: "group chat", adminID: 42, userID: 42, peerID: 100000042, wantNotice: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &scenarioTestService{}
			messenger := &recordingMessenger{}
			handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), tt.adminID)
			update := Update{Type: "message_new"}
			update.Object.Message = &struct {
				FromID int64  `json:"from_id"`
				PeerID int64  `json:"peer_id"`
				Text   string `json:"text"`
				Out    int    `json:"out"`
			}{FromID: tt.userID, PeerID: tt.peerID, Text: "/stats"}
			if err := handler.Handle(context.Background(), update); err != nil {
				t.Fatal(err)
			}
			if (service.statsCalls == 1) != tt.wantStats {
				t.Fatalf("stats calls = %d", service.statsCalls)
			}
			if tt.wantNotice == "" {
				if len(messenger.sent) != 0 {
					t.Fatalf("unexpected messages: %+v", messenger.sent)
				}
				return
			}
			if len(messenger.sent) != 1 || !strings.Contains(messenger.sent[0].text, tt.wantNotice) {
				t.Fatalf("messages: %+v", messenger.sent)
			}
			if tt.wantStats {
				for _, fragment := range []string{"Запустили бота: 2", "Выдано промокодов: 1", "24.10: 2", "Кофейня: промокодов осталось 4, участников розыгрыша 1"} {
					if !strings.Contains(messenger.sent[0].text, fragment) {
						t.Errorf("stats message missing %q: %s", fragment, messenger.sent[0].text)
					}
				}
			}
		})
	}
}

func TestExportStatsOnlyForConfiguredAdmin(t *testing.T) {
	service := &scenarioTestService{}
	messenger := &recordingMessenger{}
	handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 42)
	for _, tc := range []struct {
		userID int64
		peerID int64
	}{
		{userID: 43, peerID: 43},
		{userID: 42, peerID: 2000000042},
	} {
		update := Update{Type: "message_new"}
		update.Object.Message = &struct {
			FromID int64  `json:"from_id"`
			PeerID int64  `json:"peer_id"`
			Text   string `json:"text"`
			Out    int    `json:"out"`
		}{FromID: tc.userID, PeerID: tc.peerID, Text: "/export"}
		if err := handler.Handle(context.Background(), update); err != nil {
			t.Fatal(err)
		}
	}
	if service.statsCalls != 0 || len(messenger.documents) != 0 {
		t.Fatalf("unauthorized export: stats calls=%d, files=%d", service.statsCalls, len(messenger.documents))
	}
	update := Update{Type: "message_new"}
	update.Object.Message = &struct {
		FromID int64  `json:"from_id"`
		PeerID int64  `json:"peer_id"`
		Text   string `json:"text"`
		Out    int    `json:"out"`
	}{FromID: 42, PeerID: 42, Text: "/export"}
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if service.statsCalls != 1 || len(messenger.documents) != 2 || messenger.documents[0].peerID != 42 ||
		!strings.HasPrefix(messenger.documents[0].filename, "stats_") ||
		!strings.Contains(string(messenger.documents[0].contents), "Кофейня") ||
		!strings.HasPrefix(messenger.documents[1].filename, "draws_") ||
		!strings.Contains(string(messenger.documents[1].contents), "Кофейня") {
		t.Fatalf("admin export: stats calls=%d, files=%+v", service.statsCalls, messenger.documents)
	}
}

func TestDrawApprovalOnlyForConfiguredAdmin(t *testing.T) {
	service := &scenarioTestService{}
	messenger := &recordingMessenger{}
	handler := NewHandler(service, messenger, slog.New(slog.NewTextHandler(io.Discard, nil)), 42)

	message := func(userID int64) Update {
		update := Update{Type: "message_new"}
		update.Object.Message = &struct {
			FromID int64  `json:"from_id"`
			PeerID int64  `json:"peer_id"`
			Text   string `json:"text"`
			Out    int    `json:"out"`
		}{FromID: userID, PeerID: userID, Text: "/draws"}
		return update
	}
	callback := func(userID int64) Update {
		payload, err := json.Marshal(map[string]string{"action": handlers.ActionDrawApprovePrefix + "7"})
		if err != nil {
			t.Fatal(err)
		}
		update := Update{Type: "message_event"}
		update.Object.UserID, update.Object.PeerID, update.Object.EventID, update.Object.Payload = userID, userID, "event", payload
		return update
	}

	if err := handler.Handle(context.Background(), message(43)); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), callback(43)); err != nil {
		t.Fatal(err)
	}
	if service.pendingCalls != 0 || service.approveCalls != 0 {
		t.Fatalf("other user accessed draws: pending=%d, approve=%d", service.pendingCalls, service.approveCalls)
	}
	if err := handler.Handle(context.Background(), message(42)); err != nil {
		t.Fatal(err)
	}
	if service.pendingCalls != 1 || len(messenger.sent) != 3 ||
		!strings.Contains(messenger.sent[2].text, "Победители (VK ID): 42") ||
		messenger.sent[2].buttons[0][0].Payload != handlers.ActionDrawApprovePrefix+"7" {
		t.Fatalf("pending draw messages=%+v", messenger.sent)
	}
	if err := handler.Handle(context.Background(), callback(42)); err != nil {
		t.Fatal(err)
	}
	if service.approveCalls != 1 || service.approveID != 7 || service.approveAdmin != 42 ||
		!strings.Contains(messenger.sent[3].text, "подтверждён") {
		t.Fatalf("approval=%d/%d/%d, messages=%+v", service.approveCalls, service.approveID, service.approveAdmin, messenger.sent)
	}
}

type scenarioTestService struct {
	startCalls   int
	statsCalls   int
	pendingCalls int
	approveCalls int
	approveID    int64
	approveAdmin int64
	promoResult  domain.PromoResult
	promoErr     error
	partner      *domain.PartnerDay
	raffleResult domain.RaffleResult
	claimStatus  domain.ClaimStatus
	pendingName  bool
	savedName    string
}

func (s *scenarioTestService) PendingDraws(context.Context) ([]domain.PendingDraw, error) {
	s.pendingCalls++
	return []domain.PendingDraw{{ID: 7, Kind: "daily", Date: "24.10.2026", PartnerName: "Кофейня", Tickets: 3, WinnerIDs: "42", ReserveIDs: "43"}}, nil
}

func (s *scenarioTestService) ApproveDraw(_ context.Context, drawID, adminID int64) (domain.ApprovalOutcome, error) {
	s.approveCalls++
	s.approveID = drawID
	s.approveAdmin = adminID
	return domain.DrawApproved, nil
}

func (s *scenarioTestService) Stats(context.Context) (domain.Stats, error) {
	s.statsCalls++
	return domain.Stats{
		UsersStarted: 2, PromoRequests: 3, PromosIssued: 1,
		NewUsers: []domain.DailyUsers{{Date: time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC), Count: 2}},
		Partners: []domain.PartnerStats{{Date: time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC), PartnerName: "Кофейня", PromosLeft: 4, RaffleEntrants: 1}},
		Draws: []domain.DrawStats{{ID: 7, Kind: "daily", Date: time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC),
			DrawnAt: time.Date(2026, time.October, 25, 0, 0, 0, 0, time.UTC), PartnerName: "Кофейня",
			Tickets: 3, Participants: 2, Winners: 1, Reserves: 1, Approved: true}},
	}, nil
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

func (s *scenarioTestService) StartPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error) {
	if s.claimStatus != "" {
		return s.claimStatus, nil
	}
	return domain.ClaimNotWinner, nil
}

func (s *scenarioTestService) PendingPrizeName(context.Context, int64) (bool, error) {
	return s.pendingName, nil
}

func (s *scenarioTestService) SavePrizeName(_ context.Context, _ int64, name string) (int64, domain.ClaimStatus, error) {
	s.savedName = name
	return 7, domain.ClaimAwaitingConsent, nil
}

func (*scenarioTestService) ConsentPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error) {
	return domain.ClaimCompleted, nil
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
	documents    []recordedDocument
}

type recordedDocument struct {
	peerID   int64
	filename string
	contents []byte
}

func (m *recordingMessenger) Send(_ context.Context, peerID int64, text string, buttons [][]Button) error {
	m.sent = append(m.sent, recordedMessage{peerID: peerID, text: text, buttons: buttons})
	return nil
}

func (m *recordingMessenger) SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]Button) error {
	m.welcomeCalls++
	return m.Send(ctx, peerID, text, buttons)
}

func (m *recordingMessenger) SendDocument(_ context.Context, peerID int64, filename string, contents []byte) error {
	m.documents = append(m.documents, recordedDocument{peerID: peerID, filename: filename, contents: contents})
	return nil
}

func (m *recordingMessenger) Ack(context.Context, string, int64, int64) error {
	m.acked++
	return nil
}
