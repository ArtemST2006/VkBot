package service

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

// UserRepository stores users and reads their active days.
type UserRepository interface {
	EnsureUser(ctx context.Context, userID int64) error
	ActiveDayCount(ctx context.Context, userID int64) (int, error)
}

type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) Start(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.users.EnsureUser(ctx, userID)
}

func (s *UserService) MyChances(ctx context.Context, userID int64) (domain.Chances, error) {
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
