package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

// IssuePromo assigns a code and an active day in one transaction.
func (r *Repository) IssuePromo(ctx context.Context, userID int64) (domain.PromoResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.PromoResult{}, fmt.Errorf("begin promo transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Safe after Commit; rolls back on every earlier error.

	internalUserID, err := lockUser(ctx, tx, userID)
	if err != nil {
		return domain.PromoResult{}, err
	}

	var dayID int64
	var result domain.PromoResult
	var promoIsValid bool
	err = tx.QueryRow(ctx, `
		SELECT d.id, p.name,
		       COALESCE(d.promo_valid_until, (d.event_date + 1)::timestamp AT TIME ZONE 'Europe/Moscow'),
		       COALESCE(d.promo_instructions, ''),
		       (d.promo_valid_until IS NULL OR d.promo_valid_until >= now())
		FROM partner_days AS d
		JOIN partners AS p ON p.id = d.partner_id
		WHERE d.event_date = (now() AT TIME ZONE 'Europe/Moscow')::date
		FOR UPDATE OF d
	`).Scan(&dayID, &result.PartnerName, &result.ValidUntil, &result.Instructions, &promoIsValid)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PromoResult{Outcome: domain.PromoUnavailable}, nil
	}
	if err != nil {
		return domain.PromoResult{}, fmt.Errorf("find today's partner: %w", err)
	}

	err = tx.QueryRow(ctx, `
		SELECT code FROM promo_codes
		WHERE partner_day_id = $1 AND assigned_user_id = $2
	`, dayID, internalUserID).Scan(&result.Code)
	switch {
	case err == nil:
		result.Outcome = domain.PromoAlreadyIssued
	case !errors.Is(err, pgx.ErrNoRows):
		return domain.PromoResult{}, fmt.Errorf("find assigned promo code: %w", err)
	case !promoIsValid:
		result.Outcome = domain.PromoUnavailable
	default:
		result.Code, result.Outcome, err = assignAvailablePromo(ctx, tx, internalUserID, dayID)
		if err != nil {
			return domain.PromoResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO promo_requests (user_id, partner_day_id, outcome)
		VALUES ($1, $2, $3)`, internalUserID, dayID, string(result.Outcome)); err != nil {
		return domain.PromoResult{}, fmt.Errorf("record promo request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PromoResult{}, err
	}
	return result, nil
}

func assignAvailablePromo(ctx context.Context, tx pgx.Tx, userID, dayID int64) (string, domain.PromoOutcome, error) {
	var codeID int64
	var code string
	err := tx.QueryRow(ctx, `
		SELECT id, code FROM promo_codes
		WHERE partner_day_id = $1 AND assigned_user_id IS NULL
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, dayID).Scan(&codeID, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.PromoExhausted, nil
	}
	if err != nil {
		return "", "", fmt.Errorf("find available promo code: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE promo_codes
		SET assigned_user_id = $1, assigned_at = now()
		WHERE id = $2
	`, userID, codeID); err != nil {
		return "", "", fmt.Errorf("assign promo code: %w", err)
	}
	return code, domain.PromoIssued, nil
}
