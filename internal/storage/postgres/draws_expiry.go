package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ExpirePrizes gives unanswered prizes to the next reserve user.
func (r *Repository) ExpirePrizes(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
        SELECT res.draw_id, res.user_id, res.prize_slot, d.kind,
               COALESCE(p.name, ''), COALESCE(pd.prize_name, 'приз')
        FROM draw_results AS res
        JOIN draws AS d ON d.id = res.draw_id
        LEFT JOIN partner_days AS pd ON pd.id = d.partner_day_id
        LEFT JOIN partners AS p ON p.id = pd.partner_id
        WHERE res.state = 'active' AND res.response_deadline <= now()
        FOR UPDATE OF res
    `)
	if err != nil {
		return err
	}
	type expired struct {
		drawID, userID, slot int64
		kind, partner, prize string
	}
	var prizes []expired
	for rows.Next() {
		var prize expired
		if err := rows.Scan(&prize.drawID, &prize.userID, &prize.slot, &prize.kind, &prize.partner, &prize.prize); err != nil {
			rows.Close()
			return err
		}
		prizes = append(prizes, prize)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, prize := range prizes {
		if _, err := tx.Exec(ctx, `UPDATE draw_results
            SET state = 'forfeited', claim_step = 'none', full_name = NULL
            WHERE draw_id = $1 AND user_id = $2`, prize.drawID, prize.userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE notification_outbox SET status = 'failed'
            WHERE draw_id = $1 AND user_id = $2 AND status = 'pending'`, prize.drawID, prize.userID); err != nil {
			return err
		}
		var reserveID int64
		err := tx.QueryRow(ctx, `SELECT user_id FROM draw_results
            WHERE draw_id = $1 AND state = 'waiting' ORDER BY position LIMIT 1 FOR UPDATE`, prize.drawID).Scan(&reserveID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE draw_results
            SET state = 'active', prize_slot = $3
            WHERE draw_id = $1 AND user_id = $2`, prize.drawID, reserveID, prize.slot); err != nil {
			return err
		}
		if err := queueWinner(ctx, tx, prize.drawID, reserveID, prize.kind, prize.partner, prize.prize); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
