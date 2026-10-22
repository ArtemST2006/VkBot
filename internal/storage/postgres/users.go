package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) EnsureUser(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (platform, external_id)
		VALUES ('vk', $1)
		ON CONFLICT (platform, external_id)
		DO UPDATE SET last_seen_at = now()
	`, strconv.FormatInt(userID, 10))
	if err != nil {
		return fmt.Errorf("ensure VK user %d: %w", userID, err)
	}
	return nil
}

func (r *Repository) Chances(ctx context.Context, userID int64) (domain.Chances, error) {
	var chances domain.Chances
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE event_date < (now() AT TIME ZONE 'Europe/Moscow')::date)::integer,
		       COUNT(*) FILTER (WHERE event_date >= (now() AT TIME ZONE 'Europe/Moscow')::date)::integer
		FROM partner_days
	`).Scan(&chances.PassedDays, &chances.RemainingDays)
	if err != nil {
		return domain.Chances{}, fmt.Errorf("count campaign days: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT d.event_date, p.name
		FROM active_days AS a
		JOIN users AS u ON u.id = a.user_id
		JOIN partner_days AS d ON d.id = a.partner_day_id
		JOIN partners AS p ON p.id = d.partner_id
		WHERE u.platform = 'vk' AND u.external_id = $1
		ORDER BY d.event_date
	`, strconv.FormatInt(userID, 10))
	if err != nil {
		return domain.Chances{}, fmt.Errorf("list active days for VK user %d: %w", userID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var day domain.ActiveDay
		if err := rows.Scan(&day.Date, &day.PartnerName); err != nil {
			return domain.Chances{}, fmt.Errorf("scan active day for VK user %d: %w", userID, err)
		}
		chances.ActiveDays = append(chances.ActiveDays, day)
	}
	if err := rows.Err(); err != nil {
		return domain.Chances{}, fmt.Errorf("list active days for VK user %d: %w", userID, err)
	}
	chances.Earned = len(chances.ActiveDays)
	return chances, nil
}

// lockUser serializes a user's promo and raffle writes within their transactions.
func lockUser(ctx context.Context, tx pgx.Tx, userID int64) (int64, error) {
	var internalID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM users
		WHERE platform = 'vk' AND external_id = $1
		FOR UPDATE
	`, strconv.FormatInt(userID, 10)).Scan(&internalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("VK user %d: %w", userID, domain.ErrUserNotStarted)
	}
	if err != nil {
		return 0, fmt.Errorf("find VK user %d: %w", userID, err)
	}
	return internalID, nil
}
