package vkbot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/lib/logger"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/admin"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/navigation"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/participation"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/prize"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/promo"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers/rules"
)

type BotService interface {
	prize.Service
	admin.StatsService
	admin.DrawsService
	Start(ctx context.Context, userID int64) error
	MyChances(ctx context.Context, userID int64) (domain.Chances, error)
	IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error)
	JoinRaffle(ctx context.Context, userID int64) (domain.RaffleResult, error)
}

type Messenger interface {
	handlers.Sender
	SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]handlers.Button) error
	SendDocument(ctx context.Context, peerID int64, filename string, contents []byte) error
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
	adminID        int64
	stats          handlers.Func
	exportStats    handlers.Func
	pendingDraws   handlers.Func
}

func NewHandler(service BotService, messenger Messenger, log *slog.Logger, adminID int64) *Handler {
	return &Handler{
		start:          withHandlerLog(log, "navigation.start", navigation.NewStart(service, messenger)),
		messageUnknown: withHandlerLog(log, "navigation.unknown_message", navigation.NewUnknownMessage(messenger)),
		actionUnknown:  withHandlerLog(log, "navigation.unknown_action", navigation.NewUnknownAction(messenger)),
		actions: map[string]handlers.Func{
			handlers.ActionMenu:             withHandlerLog(log, "navigation.menu", navigation.NewMenu(messenger)),
			handlers.ActionPromo:            withHandlerLog(log, "promo.issue", promo.NewPromo(service, messenger)),
			handlers.ActionRaffle:           withHandlerLog(log, "participation.raffle", participation.NewRaffle(service, messenger)),
			handlers.ActionHowToParticipate: withHandlerLog(log, "participation.how_to_participate", participation.NewHowToParticipate(messenger)),
			handlers.ActionChances:          withHandlerLog(log, "participation.chances", participation.NewChances(service, messenger)),
			handlers.ActionRules:            withHandlerLog(log, "rules.show", rules.NewRules(messenger)),
		},
		messenger:    messenger,
		service:      service,
		log:          log,
		adminID:      adminID,
		stats:        withHandlerLog(log, "admin.stats", admin.NewStats(service, messenger)),
		exportStats:  withHandlerLog(log, "admin.export_stats", admin.NewExportStats(service, messenger)),
		pendingDraws: withHandlerLog(log, "admin.draws", admin.NewPendingDraws(service, messenger)),
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
	if message == "/stats" {
		if h.adminID > 0 && userID == h.adminID {
			return h.stats(ctx, userID)
		}
		return h.messageUnknown(ctx, userID)
	}
	if message == "/export" {
		if h.adminID > 0 && userID == h.adminID {
			return h.exportStats(ctx, userID)
		}
		return h.messageUnknown(ctx, userID)
	}
	if message == "/draws" {
		if h.adminID > 0 && userID == h.adminID {
			return h.pendingDraws(ctx, userID)
		}
		return h.messageUnknown(ctx, userID)
	}
	pending, err := h.service.PendingPrizeName(ctx, userID)
	if err != nil {
		return err
	}
	if pending {
		return withHandlerLog(h.log, "prize.name", prize.NewName(h.service, h.messenger, message))(ctx, userID)
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
	if strings.HasPrefix(action, handlers.ActionDrawApprovePrefix) {
		if h.adminID <= 0 || userID != h.adminID {
			return h.actionUnknown(ctx, userID)
		}
		if drawID, err := strconv.ParseInt(strings.TrimPrefix(action, handlers.ActionDrawApprovePrefix), 10, 64); err == nil && drawID > 0 {
			return withHandlerLog(h.log, "admin.approve_draw", admin.NewApproveDraw(h.service, h.messenger, drawID))(ctx, userID)
		}
		return h.actionUnknown(ctx, userID)
	}
	if handler, ok := h.actions[action]; ok {
		return handler(ctx, userID)
	}
	if strings.HasPrefix(action, handlers.ActionPrizeClaimPrefix) {
		if drawID, err := strconv.ParseInt(strings.TrimPrefix(action, handlers.ActionPrizeClaimPrefix), 10, 64); err == nil && drawID > 0 {
			return withHandlerLog(h.log, "prize.claim", prize.NewClaim(h.service, h.messenger, drawID))(ctx, userID)
		}
	}
	if strings.HasPrefix(action, handlers.ActionPrizeConsentPrefix) {
		if drawID, err := strconv.ParseInt(strings.TrimPrefix(action, handlers.ActionPrizeConsentPrefix), 10, 64); err == nil && drawID > 0 {
			return withHandlerLog(h.log, "prize.consent", prize.NewConsent(h.service, h.messenger, drawID))(ctx, userID)
		}
	}
	return h.actionUnknown(ctx, userID)
}

func withHandlerLog(base *slog.Logger, op string, handler handlers.Func) handlers.Func {
	return func(ctx context.Context, userID int64) error {
		log := base.With(slog.String("op", op), slog.Int64("user_id", userID))
		if err := handler(ctx, userID); err != nil {
			log.ErrorContext(ctx, "handler failed", logger.Err(err))
			return err
		}
		log.InfoContext(ctx, "handler completed")
		return nil
	}
}
