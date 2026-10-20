package vkbot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/navigation"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/participation"
)

type BotService interface {
	Start(ctx context.Context, userID int64) error
	MyChances(ctx context.Context, userID int64) (domain.Chances, error)
	CheckSubscription(ctx context.Context, userID int64, partnerID string) (bool, error)
}

type Messenger interface {
	handlers.Sender
	Ack(ctx context.Context, eventID string, userID, peerID int64) error
}

// Handler validates VK events and routes them to individual scenarios.
type Handler struct {
	start          navigation.StartHandler
	messageUnknown handlers.Scenario
	actionUnknown  handlers.Scenario
	actions        map[string]handlers.Callback
	messenger      Messenger
	log            *slog.Logger
}

func NewHandler(service BotService, messenger Messenger, log *slog.Logger) *Handler {
	actions := []handlers.Callback{
		navigation.NewMenu(messenger),
		handlers.NewPromo(messenger),
		participation.NewRaffle(messenger),
		participation.NewSubscription(service, messenger),
		participation.NewHowToParticipate(messenger),
		participation.NewChances(service, messenger),
		handlers.NewRules(messenger),
	}
	routes := make(map[string]handlers.Callback, len(actions))
	for _, handler := range actions {
		if _, exists := routes[handler.Action()]; exists {
			panic("duplicate VK callback action: " + handler.Action())
		}
		routes[handler.Action()] = handler
	}
	return &Handler{
		start:          navigation.NewStart(service, messenger),
		messageUnknown: navigation.NewUnknownMessage(messenger),
		actionUnknown:  navigation.NewUnknownAction(messenger),
		actions:        routes,
		messenger:      messenger,
		log:            log,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	userID := update.UserID()
	peerID := update.PeerID()
	h.log.DebugContext(ctx, "VK update received", slog.String("type", update.Type), slog.Int64("user_id", userID))
	switch update.Type {
	case "message_new":
		if update.Object.Message == nil || userID <= 0 || peerID <= 0 {
			return fmt.Errorf("message_new without sender or peer")
		}
		if update.Object.Message.Out != 0 || peerID != userID {
			return nil
		}
		message := strings.TrimSpace(update.Object.Message.Text)
		if h.start.Matches(message) {
			return h.start.Handle(ctx, userID)
		}
		return h.messageUnknown.Handle(ctx, userID)
	case "message_event":
		if update.Object.EventID == "" || userID <= 0 || peerID <= 0 {
			return fmt.Errorf("message_event without event, user or peer ID")
		}
		if peerID != userID {
			return nil
		}
		if err := h.messenger.Ack(ctx, update.Object.EventID, userID, peerID); err != nil {
			return err
		}
		if handler, ok := h.actions[update.Action()]; ok {
			return handler.Handle(ctx, userID)
		}
		return h.actionUnknown.Handle(ctx, userID)
	default:
		return nil
	}
}
