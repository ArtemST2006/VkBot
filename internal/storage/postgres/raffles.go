package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Raffles reads the partner of the day and registers users after verification.
type Raffles struct {
	pool *pgxpool.Pool
}

func NewRaffles(pool *pgxpool.Pool) *Raffles {
	return &Raffles{pool: pool}
}

func (r *Raffles) TodayPartner(ctx context.Context) (*domain.PartnerDay, error) {
	var partner domain.PartnerDay
	err := r.pool.QueryRow(ctx, `
		SELECT d.id, p.slug, p.name, p.account_url, COALESCE(d.prize_name, '')
		FROM partner_days AS d
		JOIN partners AS p ON p.id = d.partner_id
		WHERE d.event_date = (now() AT TIME ZONE 'Europe/Moscow')::date
	`).Scan(&partner.ID, &partner.Slug, &partner.Name, &partner.AccountURL, &partner.PrizeName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find today's partner: %w", err)
	}
	return &partner, nil
}

func (r *Raffles) ExistingRaffle(ctx context.Context, userID, dayID int64) (*domain.RaffleResult, error) {
	result := domain.RaffleResult{Outcome: domain.RaffleAlreadyJoined}
	err := r.pool.QueryRow(ctx, `
		SELECT p.name, (SELECT COUNT(*)::integer FROM active_days WHERE user_id = u.id)
		FROM raffle_entries AS e
		JOIN users AS u ON u.id = e.user_id
		JOIN partner_days AS d ON d.id = e.partner_day_id
		JOIN partners AS p ON p.id = d.partner_id
		WHERE u.platform = 'vk' AND u.external_id = $1 AND e.partner_day_id = $2
	`, strconv.FormatInt(userID, 10), dayID).Scan(&result.PartnerName, &result.Chances)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find existing raffle entry: %w", err)
	}
	return &result, nil
}

func (r *Raffles) RegisterRaffle(ctx context.Context, userID, dayID int64) (domain.RaffleResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.RaffleResult{}, fmt.Errorf("begin raffle transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	internalUserID, err := lockUser(ctx, tx, userID)
	if err != nil {
		return domain.RaffleResult{}, err
	}

	var result domain.RaffleResult
	err = tx.QueryRow(ctx, `
		SELECT p.name FROM partner_days AS d
		JOIN partners AS p ON p.id = d.partner_id
		WHERE d.id = $1 AND d.event_date = (now() AT TIME ZONE 'Europe/Moscow')::date
		FOR UPDATE OF d
	`, dayID).Scan(&result.PartnerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RaffleResult{Outcome: domain.RaffleUnavailable}, nil
	}
	if err != nil {
		return domain.RaffleResult{}, fmt.Errorf("find raffle day %d: %w", dayID, err)
	}

	var entryID int64
	err = tx.QueryRow(ctx, `
		SELECT id FROM raffle_entries
		WHERE user_id = $1 AND partner_day_id = $2
	`, internalUserID, dayID).Scan(&entryID)
	if err == nil {
		result.Outcome = domain.RaffleAlreadyJoined
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO raffle_entries (user_id, partner_day_id, subscription_checked_at)
			VALUES ($1, $2, now())
		`, internalUserID, dayID); err != nil {
			return domain.RaffleResult{}, fmt.Errorf("register raffle entry: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO active_days (user_id, partner_day_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, partner_day_id) DO NOTHING
		`, internalUserID, dayID); err != nil {
			return domain.RaffleResult{}, fmt.Errorf("add active day for raffle: %w", err)
		}
		result.Outcome = domain.RaffleJoined
	} else {
		return domain.RaffleResult{}, fmt.Errorf("find raffle entry: %w", err)
	}

	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)::integer FROM active_days WHERE user_id = $1
	`, internalUserID).Scan(&result.Chances); err != nil {
		return domain.RaffleResult{}, fmt.Errorf("count raffle chances: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RaffleResult{}, fmt.Errorf("commit raffle transaction: %w", err)
	}
	return result, nil
}
