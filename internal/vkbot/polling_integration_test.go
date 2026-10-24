package vkbot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/service"
)

func TestPollingRoutesMessagesAndButtons(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var messages []string
	var acked int
	var polls []string
	var uploads int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/longpoll" {
			polls = append(polls, r.URL.Query().Get("ts"))
			if r.URL.Query().Get("key") != "test-key" || r.URL.Query().Get("act") != "a_check" {
				t.Errorf("Long Poll query = %s", r.URL.RawQuery)
			}
			if len(polls) == 2 {
				cancel()
				return nil, context.Canceled
			}
			return jsonResponse(`{"ts":"11","updates":[{"type":"message_new","object":{"message":{"from_id":42,"peer_id":42,"text":"Начать","out":0}}},{"type":"message_new","object":{"message":{"from_id":43,"peer_id":43,"text":"/start","out":0}}},{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-1","payload":{"action":"chances:show"}}},{"type":"message_event","object":{"user_id":42,"peer_id":42,"event_id":"event-2","payload":{"action":"how:join"}}}]}`), nil
		}
		if r.URL.Path == "/upload" {
			uploads++
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Fatal(err)
			}
			photo, _, err := r.FormFile("photo")
			if err != nil {
				t.Fatal(err)
			}
			defer photo.Close()
			var header [8]byte
			if _, err := io.ReadFull(photo, header[:]); err != nil || string(header[:]) != "\x89PNG\r\n\x1a\n" {
				t.Errorf("welcome photo is not PNG: header=%q, err=%v", header, err)
			}
			return jsonResponse(`{"server":123,"photo":"uploaded-photo","hash":"hash"}`), nil
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Method != http.MethodPost || r.Form.Get("access_token") != "test-token" || r.Form.Get("v") != apiVersion {
			t.Errorf("VK API request %s %s: form = %v", r.Method, r.URL.Path, r.Form)
		}
		switch r.URL.Path {
		case "/method/groups.getLongPollServer":
			if r.Form.Get("group_id") != "123" {
				t.Errorf("group_id = %q", r.Form.Get("group_id"))
			}
			return jsonResponse(`{"response":{"key":"test-key","server":"https://vk.test/longpoll","ts":"10"}}`), nil
		case "/method/photos.getMessagesUploadServer":
			if r.Form.Get("peer_id") != "42" && r.Form.Get("peer_id") != "43" {
				t.Errorf("photo upload peer = %q", r.Form.Get("peer_id"))
			}
			return jsonResponse(`{"response":{"upload_url":"https://vk.test/upload"}}`), nil
		case "/method/photos.saveMessagesPhoto":
			if r.Form.Get("server") != "123" || r.Form.Get("photo") != "uploaded-photo" || r.Form.Get("hash") != "hash" {
				t.Errorf("save photo form = %v", r.Form)
			}
			return jsonResponse(`{"response":[{"owner_id":-123,"id":456,"access_key":"secret"}]}`), nil
		case "/method/messages.send":
			if r.Form.Get("random_id") == "" {
				t.Error("random_id is missing")
			}
			messages = append(messages, r.Form.Get("message"))
			if len(messages) <= 2 && r.Form.Get("attachment") != "photo-123_456_secret" {
				t.Errorf("welcome attachment = %q", r.Form.Get("attachment"))
			}
			if len(messages) == 1 {
				var keyboard vkKeyboard
				if err := json.Unmarshal([]byte(r.Form.Get("keyboard")), &keyboard); err != nil {
					t.Fatal(err)
				}
				if !keyboard.Inline || len(keyboard.Buttons) != 5 || keyboard.Buttons[3][0].Action.Type != "callback" {
					t.Errorf("menu keyboard = %+v", keyboard)
				}
				var payload struct {
					Action string `json:"action"`
				}
				if err := json.Unmarshal([]byte(keyboard.Buttons[3][0].Action.Payload), &payload); err != nil || payload.Action != "chances:show" {
					t.Errorf("chances payload = %+v, err = %v", payload, err)
				}
			}
			if len(messages) == 4 {
				var keyboard vkKeyboard
				if err := json.Unmarshal([]byte(r.Form.Get("keyboard")), &keyboard); err != nil {
					t.Fatal(err)
				}
				want := []string{"promo:get", "raffle:join", "chances:show", "menu"}
				if len(keyboard.Buttons) != len(want) {
					t.Fatalf("how-to keyboard = %+v", keyboard)
				}
				for i, action := range want {
					if len(keyboard.Buttons[i]) != 1 {
						t.Fatalf("how-to keyboard row %d = %+v", i, keyboard.Buttons[i])
					}
					var payload struct {
						Action string `json:"action"`
					}
					if err := json.Unmarshal([]byte(keyboard.Buttons[i][0].Action.Payload), &payload); err != nil || payload.Action != action {
						t.Errorf("how-to button %d payload = %+v, err = %v", i, payload, err)
					}
				}
			}
			return jsonResponse(`{"response":100}`), nil
		case "/method/messages.sendMessageEventAnswer":
			acked++
			if (r.Form.Get("event_id") != "event-1" && r.Form.Get("event_id") != "event-2") || r.Form.Get("peer_id") != "42" || r.Form.Get("user_id") != "42" {
				t.Errorf("event answer = %v", r.Form)
			}
			return jsonResponse(`{"response":1}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	client := NewClient("test-token", 123, "https://vk.test/method", &http.Client{Transport: transport})
	services := service.New(integrationUsers{})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(services, client, log, 0)
	if err := RunPolling(ctx, client, handler); !errors.Is(err, context.Canceled) {
		t.Fatalf("RunPolling() error = %v", err)
	}
	if acked != 2 || uploads != 2 || len(messages) != 4 || !strings.Contains(messages[0], "Добро пожаловать") || !strings.Contains(messages[1], "Добро пожаловать") || !strings.Contains(messages[2], "0 из 10") || !strings.Contains(messages[3], "Промокод не обязателен") || !strings.Contains(messages[3], "до 10 шансов") {
		t.Errorf("responses: acked=%d, uploads=%d, messages=%q", acked, uploads, messages)
	}
	if len(polls) != 2 || polls[0] != "10" || polls[1] != "11" {
		t.Errorf("poll timestamps = %v", polls)
	}
}

func TestPollingRefreshesExpiredState(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var servers int
	var polls []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/method/groups.getLongPollServer" {
			servers++
			if servers == 1 {
				return jsonResponse(`{"response":{"key":"first","server":"https://vk.test/longpoll","ts":"10"}}`), nil
			}
			return jsonResponse(`{"response":{"key":"second","server":"https://vk.test/longpoll","ts":"20"}}`), nil
		}
		polls = append(polls, r.URL.Query().Get("ts"))
		switch len(polls) {
		case 1:
			return jsonResponse(`{"failed":1,"ts":"11"}`), nil
		case 2:
			return jsonResponse(`{"failed":2}`), nil
		default:
			cancel()
			return nil, context.Canceled
		}
	})
	client := NewClient("test-token", 123, "https://vk.test/method", &http.Client{Transport: transport})
	handler := NewHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	if err := RunPolling(ctx, client, handler); !errors.Is(err, context.Canceled) {
		t.Fatalf("RunPolling() error = %v", err)
	}
	if servers != 2 || len(polls) != 3 || polls[0] != "10" || polls[1] != "11" || polls[2] != "20" {
		t.Errorf("servers=%d, polls=%v", servers, polls)
	}
}

func TestWelcomeFallsBackToTextWhenPhotoUploadFails(t *testing.T) {
	var sent bool
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch r.URL.Path {
		case "/method/photos.getMessagesUploadServer":
			return jsonResponse(`{"error":{"error_code":27,"error_msg":"photo upload unavailable"}}`), nil
		case "/method/messages.send":
			sent = true
			if r.Form.Get("message") != "Добро пожаловать" || r.Form.Get("attachment") != "" || r.Form.Get("keyboard") == "" {
				t.Errorf("fallback message = %v", r.Form)
			}
			return jsonResponse(`{"response":100}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})
	client := NewClient("test-token", 123, "https://vk.test/method", &http.Client{Transport: transport})
	if err := client.SendWelcome(context.Background(), 42, "Добро пожаловать", [][]Button{{{Text: "В меню", Payload: "menu"}}}); err != nil {
		t.Fatal(err)
	}
	if !sent {
		t.Fatal("text menu was not sent after failed photo upload")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

type integrationUsers struct{}

func (integrationUsers) Stats(context.Context) (domain.Stats, error) { return domain.Stats{}, nil }

func (integrationUsers) StartPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error) {
	return domain.ClaimNotWinner, nil
}
func (integrationUsers) PendingPrizeName(context.Context, int64) (bool, error) { return false, nil }
func (integrationUsers) SavePrizeName(context.Context, int64, string) (int64, domain.ClaimStatus, error) {
	return 0, domain.ClaimNotWinner, nil
}
func (integrationUsers) ConsentPrizeClaim(context.Context, int64, int64) (domain.ClaimStatus, error) {
	return domain.ClaimNotWinner, nil
}

func (integrationUsers) EnsureUser(context.Context, int64) error { return nil }

func (integrationUsers) Chances(context.Context, int64) (domain.Chances, error) {
	return domain.Chances{Maximum: domain.MaxChances, RemainingDays: 10}, nil
}

func (integrationUsers) IssuePromo(context.Context, int64) (domain.PromoResult, error) {
	return domain.PromoResult{Outcome: domain.PromoUnavailable}, nil
}

func (integrationUsers) TodayPartner(context.Context) (*domain.PartnerDay, error) {
	return &domain.PartnerDay{ID: 1, Name: "Демо-партнёр"}, nil
}

func (integrationUsers) ExistingRaffle(context.Context, int64, int64) (*domain.RaffleResult, error) {
	return nil, nil
}

func (integrationUsers) RegisterRaffle(context.Context, int64, int64) (domain.RaffleResult, error) {
	return domain.RaffleResult{Outcome: domain.RaffleJoined, PartnerName: "Демо-партнёр", Chances: 1}, nil
}
