package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) StartPrizeClaim(ctx context.Context, drawID, vkID int64) (domain.ClaimStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var userID int64
	var state, step string
	var deadline *time.Time
	err = tx.QueryRow(ctx, `SELECT u.id, res.state, res.claim_step, res.response_deadline
		FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		JOIN draws AS d ON d.id = res.draw_id
		WHERE res.draw_id = $1 AND u.platform = 'vk' AND u.external_id = $2
		  AND d.approved_at IS NOT NULL
		FOR UPDATE OF u, res`, drawID, strconv.FormatInt(vkID, 10)).Scan(&userID, &state, &step, &deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClaimNotWinner, nil
	}
	if err != nil {
		return "", err
	}
	if state == "claimed" {
		return domain.ClaimCompleted, nil
	}
	if state != "active" || (deadline != nil && !deadline.After(time.Now())) {
		return domain.ClaimExpired, nil
	}
	var other bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM draw_results
		WHERE user_id = $1 AND draw_id <> $2 AND state = 'active' AND claim_step <> 'none')`, userID, drawID).Scan(&other); err != nil {
		return "", err
	}
	if other {
		return domain.ClaimOtherOpen, nil
	}
	if step == "none" {
		if _, err := tx.Exec(ctx, `UPDATE draw_results SET claim_step = 'name' WHERE draw_id = $1 AND user_id = $2`, drawID, userID); err != nil {
			return "", err
		}
		step = "name"
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	if step == "consent" {
		return domain.ClaimAwaitingConsent, nil
	}
	return domain.ClaimAwaitingName, nil
}

func (r *Repository) PendingPrizeName(ctx context.Context, vkID int64) (bool, error) {
	var pending bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		JOIN draws AS d ON d.id = res.draw_id
		WHERE u.platform = 'vk' AND u.external_id = $1 AND res.state = 'active'
		  AND d.approved_at IS NOT NULL AND res.claim_step = 'name'
		  AND (res.response_deadline IS NULL OR res.response_deadline > now()))`,
		strconv.FormatInt(vkID, 10)).Scan(&pending)
	return pending, err
}

func (r *Repository) SavePrizeName(ctx context.Context, vkID int64, fullName string) (int64, domain.ClaimStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)
	var drawID int64
	var deadline *time.Time
	err = tx.QueryRow(ctx, `SELECT res.draw_id, res.response_deadline
		FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		JOIN draws AS d ON d.id = res.draw_id
		WHERE u.platform = 'vk' AND u.external_id = $1 AND res.state = 'active'
		  AND d.approved_at IS NOT NULL AND res.claim_step = 'name'
		FOR UPDATE OF res`, strconv.FormatInt(vkID, 10)).Scan(&drawID, &deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ClaimNotWinner, nil
	}
	if err != nil {
		return 0, "", err
	}
	if deadline != nil && !deadline.After(time.Now()) {
		return drawID, domain.ClaimExpired, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE draw_results SET full_name = $2, claim_step = 'consent'
		WHERE draw_id = $1 AND user_id = (SELECT id FROM users WHERE platform = 'vk' AND external_id = $3)`,
		drawID, fullName, strconv.FormatInt(vkID, 10)); err != nil {
		return 0, "", err
	}
	return drawID, domain.ClaimAwaitingConsent, tx.Commit(ctx)
}

func (r *Repository) ConsentPrizeClaim(ctx context.Context, drawID, vkID int64) (domain.ClaimStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var state, step string
	var deadline *time.Time
	var userID int64
	err = tx.QueryRow(ctx, `SELECT u.id, res.state, res.claim_step, res.response_deadline
		FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		JOIN draws AS d ON d.id = res.draw_id
		WHERE res.draw_id = $1 AND u.platform = 'vk' AND u.external_id = $2
		  AND d.approved_at IS NOT NULL FOR UPDATE OF res`,
		drawID, strconv.FormatInt(vkID, 10)).Scan(&userID, &state, &step, &deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClaimNotWinner, nil
	}
	if err != nil {
		return "", err
	}
	if state == "claimed" {
		return domain.ClaimCompleted, nil
	}
	if state != "active" || (deadline != nil && !deadline.After(time.Now())) {
		return domain.ClaimExpired, nil
	}
	if step == "name" {
		return domain.ClaimAwaitingName, nil
	}
	if step != "consent" {
		return domain.ClaimNotWinner, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE draw_results SET state = 'claimed', claim_step = 'none', consented_at = now()
		WHERE draw_id = $1 AND user_id = $2`, drawID, userID); err != nil {
		return "", err
	}
	return domain.ClaimCompleted, tx.Commit(ctx)
}
