package vkbot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/navigation"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/participation"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/prize"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/promo"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/rules"
)

type BotService interface {
	prize.Service
	Start(ctx context.Context, userID int64) error
	MyChances(ctx context.Context, userID int64) (domain.Chances, error)
	IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error)
	TodayPartner(ctx context.Context) (*domain.PartnerDay, error)
	JoinRaffle(ctx context.Context, userID int64) (domain.RaffleResult, error)
}

type Messenger interface {
	handlers.Sender
	SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]handlers.Button) error
	Ack(ctx context.Context, eventID string, userID, peerID int64) error
}

// Handler validates VK events and routes them to individual scenarios.
type Handler struct {
	start          handlers.Func
	messageUnknown handlers.Func
	actionUnknown  handlers.Func
	actions        map[string]handlers.Func
	service        BotService
	messenger      Messenger
	log            *slog.Logger
}

func NewHandler(service BotService, messenger Messenger, log *slog.Logger) *Handler {
	return &Handler{
		start:          navigation.NewStart(service, messenger),
		messageUnknown: navigation.NewUnknownMessage(messenger),
		actionUnknown:  navigation.NewUnknownAction(messenger),
		actions: map[string]handlers.Func{
			handlers.ActionMenu:              navigation.NewMenu(messenger),
			handlers.ActionPromo:             promo.NewPromo(service, messenger),
			handlers.ActionRaffle:            participation.NewRaffle(service, messenger),
			handlers.ActionSubscriptionCheck: participation.NewSubscription(service, messenger),
			handlers.ActionHowToParticipate:  participation.NewHowToParticipate(messenger),
			handlers.ActionChances:           participation.NewChances(service, messenger),
			handlers.ActionRules:             rules.NewRules(messenger),
		},
		messenger: messenger,
		service:   service,
		log:       log,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	userID := update.UserID()
	peerID := update.PeerID()
	h.log.DebugContext(ctx, "VK update received", slog.String("type", update.Type), slog.Int64("user_id", userID))
	switch update.Type {
	case "message_new":
		return h.handleMessage(ctx, update, userID, peerID)
	case "message_event":
		return h.handleCallback(ctx, update, userID, peerID)
	default:
		return nil
	}
}

func (h *Handler) handleMessage(ctx context.Context, update Update, userID, peerID int64) error {
	if update.Object.Message == nil || userID <= 0 || peerID <= 0 {
		return fmt.Errorf("message_new without sender or peer")
	}
	if update.Object.Message.Out != 0 || peerID != userID {
		return nil
	}
	message := strings.TrimSpace(update.Object.Message.Text)
	if navigation.IsStart(message) {
		return h.start(ctx, userID)
	}
	pending, err := h.service.PendingPrizeName(ctx, userID)
	if err != nil {
		return err
	}
	if pending {
		return prize.NewName(h.service, h.messenger, message)(ctx, userID)
	}
	return h.messageUnknown(ctx, userID)
}

func (h *Handler) handleCallback(ctx context.Context, update Update, userID, peerID int64) error {
	if update.Object.EventID == "" || userID <= 0 || peerID <= 0 {
		return fmt.Errorf("message_event without event, user or peer ID")
	}
	if peerID != userID {
		return nil
	}
	if err := h.messenger.Ack(ctx, update.Object.EventID, userID, peerID); err != nil {
		return err
	}
	action := update.Action()
	if handler, ok := h.actions[action]; ok {
		return handler(ctx, userID)
	}
	if strings.HasPrefix(action, handlers.ActionPrizeClaimPrefix) {
		if drawID, err := strconv.ParseInt(strings.TrimPrefix(action, handlers.ActionPrizeClaimPrefix), 10, 64); err == nil && drawID > 0 {
			return prize.NewClaim(h.service, h.messenger, drawID)(ctx, userID)
		}
	}
	if strings.HasPrefix(action, handlers.ActionPrizeConsentPrefix) {
		if drawID, err := strconv.ParseInt(strings.TrimPrefix(action, handlers.ActionPrizeConsentPrefix), 10, 64); err == nil && drawID > 0 {
			return prize.NewConsent(h.service, h.messenger, drawID)(ctx, userID)
		}
	}
	return h.actionUnknown(ctx, userID)
}
