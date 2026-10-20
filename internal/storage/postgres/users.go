package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Users persists bot users and reads their active campaign days.
type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

func (r *Users) EnsureUser(ctx context.Context, userID int64) error {
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

func (r *Users) ActiveDayCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::integer
		FROM active_days AS a
		JOIN users AS u ON u.id = a.user_id
		WHERE u.platform = 'vk' AND u.external_id = $1
	`, strconv.FormatInt(userID, 10)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active days for VK user %d: %w", userID, err)
	}
	return count, nil
}
