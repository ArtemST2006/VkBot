package postgres

import (
	"context"
	"fmt"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Stats reads every counter from the same database snapshot.
func (r *Repository) Stats(ctx context.Context) (domain.Stats, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Stats{}, fmt.Errorf("begin stats transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var stats domain.Stats
	err = tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM users WHERE platform = 'vk'),
		       (SELECT count(*) FROM promo_requests),
		       (SELECT count(*) FROM promo_codes WHERE assigned_user_id IS NOT NULL)
	`).Scan(&stats.UsersStarted, &stats.PromoRequests, &stats.PromosIssued)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("read stats totals: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT d.event_date, p.name,
		       (SELECT count(*) FROM promo_requests AS req WHERE req.partner_day_id = d.id),
		       (SELECT count(*) FROM promo_codes AS c
		        WHERE c.partner_day_id = d.id AND c.assigned_user_id IS NOT NULL),
		       (SELECT count(*) FROM promo_codes AS c
		        WHERE c.partner_day_id = d.id AND c.assigned_user_id IS NULL),
		       (SELECT count(*) FROM raffle_entries AS e WHERE e.partner_day_id = d.id),
		       (SELECT count(*) FROM users AS u
		        WHERE u.platform = 'vk'
		          AND (u.started_at AT TIME ZONE 'Europe/Moscow')::date = d.event_date)
		FROM partner_days AS d
		JOIN partners AS p ON p.id = d.partner_id
		ORDER BY d.event_date
	`)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("read stats by day: %w", err)
	}
	for rows.Next() {
		var partner domain.PartnerStats
		var newUsers domain.DailyUsers
		if err := rows.Scan(&partner.Date, &partner.PartnerName, &partner.PromoRequests, &partner.PromosIssued,
			&partner.PromosLeft, &partner.RaffleEntrants, &newUsers.Count); err != nil {
			rows.Close()
			return domain.Stats{}, fmt.Errorf("scan stats by day: %w", err)
		}
		newUsers.Date = partner.Date
		stats.Partners = append(stats.Partners, partner)
		stats.NewUsers = append(stats.NewUsers, newUsers)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Stats{}, fmt.Errorf("iterate stats by day: %w", err)
	}
	rows.Close()
	rows, err = tx.Query(ctx, `
		SELECT d.id, d.kind,
		       COALESCE(pd.event_date, (d.drawn_at AT TIME ZONE 'Europe/Moscow')::date),
		       d.drawn_at, COALESCE(p.name, ''), jsonb_array_length(
		           CASE WHEN jsonb_typeof(d.tickets) = 'array' THEN d.tickets ELSE '[]'::jsonb END),
		       (SELECT count(DISTINCT ticket.value->>'user_id')
		        FROM jsonb_array_elements(
		            CASE WHEN jsonb_typeof(d.tickets) = 'array' THEN d.tickets ELSE '[]'::jsonb END
		        ) AS ticket(value)),
		       (SELECT count(*) FROM draw_results AS res
		        WHERE res.draw_id = d.id AND res.position <= d.winner_count),
		       (SELECT count(*) FROM draw_results AS res
		        WHERE res.draw_id = d.id AND res.position > d.winner_count),
		       d.approved_at IS NOT NULL
		FROM draws AS d
		LEFT JOIN partner_days AS pd ON pd.id = d.partner_day_id
		LEFT JOIN partners AS p ON p.id = pd.partner_id
		ORDER BY d.id
	`)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("read draw stats: %w", err)
	}
	for rows.Next() {
		var draw domain.DrawStats
		if err := rows.Scan(&draw.ID, &draw.Kind, &draw.Date, &draw.DrawnAt, &draw.PartnerName,
			&draw.Tickets, &draw.Participants, &draw.Winners, &draw.Reserves, &draw.Approved); err != nil {
			rows.Close()
			return domain.Stats{}, fmt.Errorf("scan draw stats: %w", err)
		}
		stats.Draws = append(stats.Draws, draw)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Stats{}, fmt.Errorf("iterate draw stats: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return domain.Stats{}, fmt.Errorf("commit stats transaction: %w", err)
	}
	return stats, nil
}
