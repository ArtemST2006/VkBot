package service

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

// UserRepository is the storage needed by the first bot scenarios.
type UserRepository interface {
	EnsureUser(ctx context.Context, userID int64) error
	ActiveDayCount(ctx context.Context, userID int64) (int, error)
}

// SubscriptionChecker can later be replaced with a real community check.
type SubscriptionChecker interface {
	IsSubscribed(ctx context.Context, userID int64, partnerID string) (bool, error)
}

type Service struct {
	users         UserRepository
	subscriptions SubscriptionChecker
}

func NewService(users UserRepository, subscriptions SubscriptionChecker) *Service {
	return &Service{users: users, subscriptions: subscriptions}
}

func (s *Service) Start(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.users.EnsureUser(ctx, userID)
}

func (s *Service) MyChances(ctx context.Context, userID int64) (domain.Chances, error) {
	if userID <= 0 {
		return domain.Chances{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	count, err := s.users.ActiveDayCount(ctx, userID)
	if err != nil {
		return domain.Chances{}, err
	}
	if count > domain.MaxChances {
		count = domain.MaxChances
	}
	return domain.Chances{Earned: count, Maximum: domain.MaxChances}, nil
}

func (s *Service) CheckSubscription(ctx context.Context, userID int64, partnerID string) (bool, error) {
	if userID <= 0 {
		return false, fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.subscriptions.IsSubscribed(ctx, userID, partnerID)
}
