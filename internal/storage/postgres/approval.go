package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) PendingDraws(ctx context.Context) ([]domain.PendingDraw, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.kind, COALESCE(to_char(pd.event_date, 'DD.MM.YYYY'), ''),
		       COALESCE(p.name, ''), jsonb_array_length(
		           CASE WHEN jsonb_typeof(d.tickets) = 'array' THEN d.tickets ELSE '[]'::jsonb END),
		       COALESCE((SELECT string_agg(u.external_id, ', ' ORDER BY res.position)
		                 FROM draw_results AS res JOIN users AS u ON u.id = res.user_id
		                 WHERE res.draw_id = d.id AND res.state = 'active'), ''),
		       COALESCE((SELECT string_agg(u.external_id, ', ' ORDER BY res.position)
		                 FROM draw_results AS res JOIN users AS u ON u.id = res.user_id
		                 WHERE res.draw_id = d.id AND res.state = 'waiting'), '')
		FROM draws AS d
		LEFT JOIN partner_days AS pd ON pd.id = d.partner_day_id
		LEFT JOIN partners AS p ON p.id = pd.partner_id
		WHERE d.approved_at IS NULL
		ORDER BY d.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending draws: %w", err)
	}
	defer rows.Close()
	var draws []domain.PendingDraw
	for rows.Next() {
		var draw domain.PendingDraw
		if err := rows.Scan(&draw.ID, &draw.Kind, &draw.Date, &draw.PartnerName, &draw.Tickets, &draw.WinnerIDs, &draw.ReserveIDs); err != nil {
			return nil, fmt.Errorf("scan pending draw: %w", err)
		}
		draws = append(draws, draw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending draws: %w", err)
	}
	return draws, nil
}

// ApproveDraw atomically records approval and releases winner notifications.
func (r *Repository) ApproveDraw(ctx context.Context, drawID, adminVKID int64) (domain.ApprovalOutcome, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin draw approval: %w", err)
	}
	defer tx.Rollback(ctx)
	var kind, partner, prize string
	var approved bool
	err = tx.QueryRow(ctx, `
		SELECT d.kind, COALESCE(p.name, ''), COALESCE(pd.prize_name, ''),
		       d.approved_at IS NOT NULL
		FROM draws AS d
		LEFT JOIN partner_days AS pd ON pd.id = d.partner_day_id
		LEFT JOIN partners AS p ON p.id = pd.partner_id
		WHERE d.id = $1
		FOR UPDATE OF d
	`, drawID).Scan(&kind, &partner, &prize, &approved)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DrawNotFound, nil
	}
	if err != nil {
		return "", fmt.Errorf("find draw to approve: %w", err)
	}
	if approved {
		return domain.DrawAlreadyApproved, nil
	}
	rows, err := tx.Query(ctx, `SELECT user_id FROM draw_results
		WHERE draw_id = $1 AND state = 'active' ORDER BY position`, drawID)
	if err != nil {
		return "", fmt.Errorf("list winners to notify: %w", err)
	}
	var winnerIDs []int64
	for rows.Next() {
		var winnerID int64
		if err := rows.Scan(&winnerID); err != nil {
			rows.Close()
			return "", fmt.Errorf("scan winner to notify: %w", err)
		}
		winnerIDs = append(winnerIDs, winnerID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", fmt.Errorf("iterate winners to notify: %w", err)
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `UPDATE draws SET approved_at = now(), approved_by_vk_id = $2
		WHERE id = $1`, drawID, adminVKID); err != nil {
		return "", fmt.Errorf("record draw approval: %w", err)
	}
	for _, winnerID := range winnerIDs {
		if err := queueWinner(ctx, tx, drawID, winnerID, kind, partner, prize); err != nil {
			return "", fmt.Errorf("queue approved winner: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit draw approval: %w", err)
	}
	return domain.DrawApproved, nil
}
