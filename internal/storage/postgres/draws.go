package postgres

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/jackc/pgx/v5"
)

// DueDraws returns finished days without a draw and whether the final draw is due.
func (r *Repository) DueDraws(ctx context.Context) ([]time.Time, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.event_date FROM partner_days AS d
		WHERE d.event_date < (now() AT TIME ZONE 'Europe/Moscow')::date
		  AND NOT EXISTS (SELECT 1 FROM draws WHERE partner_day_id = d.id)
		ORDER BY d.event_date
	`)
	if err != nil {
		return nil, false, err
	}
	var days []time.Time
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			rows.Close()
			return nil, false, err
		}
		days = append(days, day)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	var final bool
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) > 0
		  AND MAX(event_date) < (now() AT TIME ZONE 'Europe/Moscow')::date
		  AND NOT EXISTS (SELECT 1 FROM draws WHERE kind = 'final')
		FROM partner_days
	`).Scan(&final)
	return days, final, err
}

func (r *Repository) DrawDaily(ctx context.Context, date time.Time) (domain.DrawSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	defer tx.Rollback(ctx)

	var dayID int64
	var winners int
	err = tx.QueryRow(ctx, `SELECT d.id, d.winner_count
		FROM partner_days AS d
		WHERE d.event_date = $1 AND d.event_date < (now() AT TIME ZONE 'Europe/Moscow')::date`,
		date.Format("2006-01-02")).Scan(&dayID, &winners)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DrawSummary{}, fmt.Errorf("partner day is missing or has not ended")
	}
	if err != nil {
		return domain.DrawSummary{}, err
	}
	tickets, err := readTickets(ctx, tx, `SELECT user_id, partner_day_id FROM raffle_entries WHERE partner_day_id = $1 ORDER BY id`, dayID)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	summary, err := createDraw(ctx, tx, "daily", &dayID, tickets, winners)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	summary.Date = date
	return summary, tx.Commit(ctx)
}

func (r *Repository) DrawFinal(ctx context.Context) (domain.DrawSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	defer tx.Rollback(ctx)
	var ended bool
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) > 0 AND MAX(event_date) < (now() AT TIME ZONE 'Europe/Moscow')::date FROM partner_days`).Scan(&ended); err != nil {
		return domain.DrawSummary{}, err
	}
	if !ended {
		return domain.DrawSummary{}, fmt.Errorf("campaign has not ended")
	}
	tickets, err := readTickets(ctx, tx, `SELECT user_id, partner_day_id FROM (
        SELECT a.user_id, a.partner_day_id,
               row_number() OVER (PARTITION BY a.user_id ORDER BY d.event_date, a.partner_day_id) AS chance
        FROM active_days AS a JOIN partner_days AS d ON d.id = a.partner_day_id
    ) AS tickets WHERE chance <= $1 ORDER BY user_id, partner_day_id`, domain.MaxChances)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	summary, err := createDraw(ctx, tx, "final", nil, tickets, domain.FinalWinnerCount)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	return summary, tx.Commit(ctx)
}

func readTickets(ctx context.Context, tx pgx.Tx, query string, arg any) ([]domain.DrawTicket, error) {
	rows, err := tx.Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []domain.DrawTicket
	for rows.Next() {
		var ticket domain.DrawTicket
		if err := rows.Scan(&ticket.UserID, &ticket.DayID); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func createDraw(ctx context.Context, tx pgx.Tx, kind string, dayID *int64, tickets []domain.DrawTicket, winnerCount int) (domain.DrawSummary, error) {
	if tickets == nil {
		tickets = []domain.DrawTicket{}
	}
	snapshot, err := json.Marshal(tickets)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	var drawID int64
	err = tx.QueryRow(ctx, `INSERT INTO draws (kind, partner_day_id, winner_count, tickets)
		VALUES ($1, $2, $3, $4) RETURNING id`, kind, dayID, winnerCount, string(snapshot)).Scan(&drawID)
	if err != nil {
		return domain.DrawSummary{}, fmt.Errorf("create %s draw: %w", kind, err)
	}
	ids := make([]int64, len(tickets))
	for i, ticket := range tickets {
		ids[i] = ticket.UserID
	}
	selected, err := domain.SelectWinners(rand.Reader, ids, winnerCount*2)
	if err != nil {
		return domain.DrawSummary{}, err
	}
	winners := min(winnerCount, len(selected))
	for position, userID := range selected {
		if position < winners {
			if _, err := tx.Exec(ctx, `INSERT INTO draw_results (draw_id, user_id, position, state, prize_slot)
				VALUES ($1, $2, $3, 'active', $3)`, drawID, userID, position+1); err != nil {
				return domain.DrawSummary{}, err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO draw_results (draw_id, user_id, position)
			VALUES ($1, $2, $3)`, drawID, userID, position+1); err != nil {
			return domain.DrawSummary{}, err
		}
	}
	return domain.DrawSummary{ID: drawID, Kind: kind, Tickets: len(tickets), Participants: uniqueCount(ids), Winners: winners, Reserves: len(selected) - winners}, nil
}

func queueWinner(ctx context.Context, tx pgx.Tx, drawID, userID int64, kind, partner, prize string) error {
	var message string
	if kind == "final" {
		message = "Твой сентябрь выходит на главную сцену 🔥\nТы выиграл два билета на концерт Канье Уэста!\nНажми «Получить приз», чтобы оставить имя и фамилию и дать согласие на передачу данных."
	} else {
		message = fmt.Sprintf("Ты в числе хедлайнеров дня 🔥\nТы выиграл %s от %s.\nНажми «Получить приз», чтобы оставить имя и фамилию и дать согласие на передачу данных.", prize, partner)
	}
	_, err := tx.Exec(ctx, `INSERT INTO notification_outbox (user_id, draw_id, message, button_action)
		VALUES ($1, $2, $3, $4) ON CONFLICT (draw_id, user_id) DO NOTHING`,
		userID, drawID, message, "prize:claim:"+strconv.FormatInt(drawID, 10))
	return err
}

func uniqueCount(ids []int64) int {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return len(seen)
}
