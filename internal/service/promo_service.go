package service

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

type PromoRepository interface {
	IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error)
}

type PromoService struct {
	promos PromoRepository
}

func NewPromoService(promos PromoRepository) *PromoService {
	return &PromoService{promos: promos}
}

func (s *PromoService) IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error) {
	if userID <= 0 {
		return domain.PromoResult{}, fmt.Errorf("invalid user ID: %d", userID)
	}
	return s.promos.IssuePromo(ctx, userID)
}
