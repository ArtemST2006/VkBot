package postgres

import (
	"context"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

// The bot runs one delivery worker. VK random_id is the outbox ID, so a retry is safe.
func (r *Repository) NextNotification(ctx context.Context) (*domain.Notification, error) {
	var item domain.Notification
	err := r.pool.QueryRow(ctx, `SELECT o.id, u.external_id::bigint, o.attempts, o.message, COALESCE(o.button_action, '')
		FROM notification_outbox AS o
		JOIN users AS u ON u.id = o.user_id
		JOIN draws AS d ON d.id = o.draw_id
		WHERE o.status = 'pending' AND o.next_attempt_at <= now()
		  AND u.platform = 'vk' AND d.approved_at IS NOT NULL
        ORDER BY o.id LIMIT 1`).Scan(&item.ID, &item.UserID, &item.Attempts, &item.Message, &item.Action)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) MarkNotificationSent(ctx context.Context, item domain.Notification) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var drawID int64
	var userID int64
	err = tx.QueryRow(ctx, `UPDATE notification_outbox SET status = 'sent'
        WHERE id = $1 AND status = 'pending' RETURNING draw_id, user_id`, item.ID).Scan(&drawID, &userID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE draw_results AS res
		SET response_deadline = now() + interval '72 hours'
		WHERE res.draw_id = $1 AND res.user_id = $2
          AND res.state = 'active' AND res.response_deadline IS NULL`, drawID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) MarkNotificationFailed(ctx context.Context, item domain.Notification) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var drawID int64
	var userID int64
	err = tx.QueryRow(ctx, `UPDATE notification_outbox
        SET attempts = attempts + 1, status = CASE WHEN attempts + 1 >= 5 THEN 'failed' ELSE 'pending' END,
            next_attempt_at = now() + (attempts + 1) * interval '1 minute'
        WHERE id = $1 AND status = 'pending' RETURNING draw_id, user_id`, item.ID).Scan(&drawID, &userID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if item.Attempts+1 >= 5 {
		if _, err := tx.Exec(ctx, `UPDATE draw_results SET response_deadline = now()
            WHERE draw_id = $1 AND user_id = $2 AND state = 'active'`, drawID, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
