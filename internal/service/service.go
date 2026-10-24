// Package service contains the bot's application logic without VK or SQL details.
package service

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

type Repository interface {
	EnsureUser(ctx context.Context, userID int64) error
	Chances(ctx context.Context, userID int64) (domain.Chances, error)
	IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error)
	TodayPartner(ctx context.Context) (*domain.PartnerDay, error)
	ExistingRaffle(ctx context.Context, userID, dayID int64) (*domain.RaffleResult, error)
	RegisterRaffle(ctx context.Context, userID, dayID int64) (domain.RaffleResult, error)
	Stats(ctx context.Context) (domain.Stats, error)
	StartPrizeClaim(ctx context.Context, drawID, userID int64) (domain.ClaimStatus, error)
	PendingPrizeName(ctx context.Context, userID int64) (bool, error)
	SavePrizeName(ctx context.Context, userID int64, fullName string) (int64, domain.ClaimStatus, error)
	ConsentPrizeClaim(ctx context.Context, drawID, userID int64) (domain.ClaimStatus, error)
}

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Start(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.repo.EnsureUser(ctx, userID)
}

func (s *Service) Stats(ctx context.Context) (domain.Stats, error) {
	return s.repo.Stats(ctx)
}

func (s *Service) MyChances(ctx context.Context, userID int64) (domain.Chances, error) {
	if userID <= 0 {
		return domain.Chances{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	chances, err := s.repo.Chances(ctx, userID)
	if err != nil {
		return domain.Chances{}, err
	}
	if chances.Earned > domain.MaxChances {
		chances.Earned = domain.MaxChances
	}
	chances.Maximum = domain.MaxChances
	return chances, nil
}

func (s *Service) IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error) {
	if userID <= 0 {
		return domain.PromoResult{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.repo.IssuePromo(ctx, userID)
}

func (s *Service) JoinRaffle(ctx context.Context, userID int64) (domain.RaffleResult, error) {
	if userID <= 0 {
		return domain.RaffleResult{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	partner, err := s.repo.TodayPartner(ctx)
	if err != nil {
		return domain.RaffleResult{}, err
	}
	if partner == nil {
		return domain.RaffleResult{Outcome: domain.RaffleUnavailable}, nil
	}
	existing, err := s.repo.ExistingRaffle(ctx, userID, partner.ID)
	if err != nil {
		return domain.RaffleResult{}, err
	}
	var result domain.RaffleResult
	if existing != nil {
		result = *existing
	} else {
		result, err = s.repo.RegisterRaffle(ctx, userID, partner.ID)
		if err != nil {
			return domain.RaffleResult{}, err
		}
	}
	result.AccountURL = partner.AccountURL
	result.PrizeName = partner.PrizeName
	if result.Chances > domain.MaxChances {
		result.Chances = domain.MaxChances
	}
	return result, nil
}

func (s *Service) StartPrizeClaim(ctx context.Context, drawID, userID int64) (domain.ClaimStatus, error) {
	return s.repo.StartPrizeClaim(ctx, drawID, userID)
}

func (s *Service) PendingPrizeName(ctx context.Context, userID int64) (bool, error) {
	return s.repo.PendingPrizeName(ctx, userID)
}

func (s *Service) SavePrizeName(ctx context.Context, userID int64, input string) (int64, domain.ClaimStatus, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 || utf8.RuneCountInString(input) > 120 {
		return 0, "", domain.ErrInvalidFullName
	}
	for _, part := range parts {
		for _, char := range part {
			if !unicode.IsLetter(char) && char != '-' {
				return 0, "", domain.ErrInvalidFullName
			}
		}
	}
	return s.repo.SavePrizeName(ctx, userID, strings.Join(parts, " "))
}

func (s *Service) ConsentPrizeClaim(ctx context.Context, drawID, userID int64) (domain.ClaimStatus, error) {
	return s.repo.ConsentPrizeClaim(ctx, drawID, userID)
}
