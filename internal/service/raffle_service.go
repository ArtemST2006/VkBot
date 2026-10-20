package service

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

type RaffleRepository interface {
	TodayPartner(ctx context.Context) (*domain.PartnerDay, error)
	ExistingRaffle(ctx context.Context, userID, dayID int64) (*domain.RaffleResult, error)
	RegisterRaffle(ctx context.Context, userID, dayID int64) (domain.RaffleResult, error)
}

// SubscriptionChecker can later be replaced with a real community check.
type SubscriptionChecker interface {
	IsSubscribed(ctx context.Context, userID int64, partnerID string) (bool, error)
}

type RaffleService struct {
	raffles       RaffleRepository
	subscriptions SubscriptionChecker
}

func NewRaffleService(raffles RaffleRepository, subscriptions SubscriptionChecker) *RaffleService {
	return &RaffleService{raffles: raffles, subscriptions: subscriptions}
}

func (s *RaffleService) TodayPartner(ctx context.Context) (*domain.PartnerDay, error) {
	return s.raffles.TodayPartner(ctx)
}

func (s *RaffleService) JoinRaffle(ctx context.Context, userID int64) (domain.RaffleResult, error) {
	if userID <= 0 {
		return domain.RaffleResult{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	partner, err := s.raffles.TodayPartner(ctx)
	if err != nil {
		return domain.RaffleResult{}, err
	}
	if partner == nil {
		return domain.RaffleResult{Outcome: domain.RaffleUnavailable}, nil
	}

	existing, err := s.raffles.ExistingRaffle(ctx, userID, partner.ID)
	if err != nil {
		return domain.RaffleResult{}, err
	}
	var result domain.RaffleResult
	if existing != nil {
		result = *existing
	} else {
		subscribed, err := s.subscriptions.IsSubscribed(ctx, userID, partner.Slug)
		if err != nil {
			return domain.RaffleResult{}, err
		}
		if !subscribed {
			return domain.RaffleResult{
				Outcome:     domain.RaffleUnsubscribed,
				PartnerName: partner.Name,
				AccountURL:  partner.AccountURL,
			}, nil
		}
		result, err = s.raffles.RegisterRaffle(ctx, userID, partner.ID)
		if err != nil {
			return domain.RaffleResult{}, err
		}
	}

	result.AccountURL = partner.AccountURL
	if result.Chances > domain.MaxChances {
		result.Chances = domain.MaxChances
	}
	return result, nil
}
