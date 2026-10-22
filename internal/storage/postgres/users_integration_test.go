package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/service"
	"github.com/ArtemST2006/VkBot/internal/subscription"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresMigrationsAndUsers(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("vkbot_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	}()

	goosePath, err := exec.LookPath("goose")
	if err != nil {
		t.Fatalf("goose CLI is required for PostgreSQL integration test: %v", err)
	}
	migrationDir, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	testURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := testURL.Query()
	query.Set("search_path", schema)
	testURL.RawQuery = query.Encode()
	for range 2 {
		command := exec.CommandContext(ctx, goosePath, "-dir", migrationDir, "postgres", testURL.String(), "up")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("goose up failed: %v\n%s", err, output)
		}
	}
	config, err := pgxpool.ParseConfig(testURL.String())
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var migrationsApplied int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE version_id IN (1, 2) AND is_applied").Scan(&migrationsApplied); err != nil || migrationsApplied != 2 {
		t.Fatalf("Goose applied versions=%d, err=%v", migrationsApplied, err)
	}

	var demoPartners, demoDays, demoCodes int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM partners WHERE slug LIKE 'demo-partner-%'").Scan(&demoPartners); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM partner_days AS d
		JOIN partners AS p ON p.id = d.partner_id
		WHERE p.slug LIKE 'demo-partner-%'
		  AND d.promo_valid_until IS NOT NULL
		  AND d.promo_instructions IS NOT NULL
		  AND d.prize_name IS NOT NULL
	`).Scan(&demoDays); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM promo_codes AS c
		JOIN partner_days AS d ON d.id = c.partner_day_id
		JOIN partners AS p ON p.id = d.partner_id
		WHERE p.slug LIKE 'demo-partner-%' AND c.assigned_user_id IS NULL
	`).Scan(&demoCodes); err != nil {
		t.Fatal(err)
	}
	if demoPartners != 10 || demoDays != 10 || demoCodes != 50 {
		t.Fatalf("demo seed: partners=%d, days=%d, available codes=%d", demoPartners, demoDays, demoCodes)
	}

	users := NewUsers(pool)
	if err := users.EnsureUser(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if err := users.EnsureUser(ctx, 42); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM users WHERE platform = 'vk' AND external_id = '42'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var userCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil || userCount != 1 {
		t.Fatalf("user count=%d, err=%v", userCount, err)
	}

	var partnerID, dayID int64
	if err := pool.QueryRow(ctx, "INSERT INTO partners (slug, name, account_url) VALUES ('partner', 'Partner', 'https://vk.com/partner') RETURNING id").Scan(&partnerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO partner_days (partner_id, event_date) VALUES ($1, '2035-01-01') RETURNING id", partnerID).Scan(&dayID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO active_days (user_id, partner_day_id) VALUES ($1, $2)", userID, dayID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO active_days (user_id, partner_day_id) VALUES ($1, $2)", userID, dayID); err == nil {
		t.Fatal("same day created a duplicate chance")
	}
	summary, err := users.Chances(ctx, 42)
	if err != nil || summary.Earned != 1 {
		t.Fatalf("active day count=%d, err=%v", summary.Earned, err)
	}

	if _, err := pool.Exec(ctx, "INSERT INTO raffle_entries (user_id, partner_day_id, subscription_checked_at) VALUES ($1, $2, now())", userID, dayID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO raffle_entries (user_id, partner_day_id, subscription_checked_at) VALUES ($1, $2, now())", userID, dayID); err == nil {
		t.Fatal("same user registered twice for one partner day")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO promo_codes (partner_day_id, code, assigned_user_id, assigned_at) VALUES ($1, 'A', $2, now())", dayID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO promo_codes (partner_day_id, code, assigned_user_id, assigned_at) VALUES ($1, 'B', $2, now())", dayID, userID); err == nil {
		t.Fatal("same user received two promo codes for one partner day")
	}

	promos := NewPromos(pool)
	if _, err := promos.IssuePromo(ctx, 999); !errors.Is(err, domain.ErrUserNotStarted) {
		t.Fatalf("promo for a user who did not press Start: %v", err)
	}
	first, err := promos.IssuePromo(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != domain.PromoIssued || first.PartnerName == "" || first.Code == "" || first.Instructions == "" || !first.ValidUntil.After(time.Now()) {
		t.Fatalf("first promo result = %+v", first)
	}
	repeat, err := promos.IssuePromo(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Outcome != domain.PromoAlreadyIssued || repeat.Code != first.Code {
		t.Fatalf("repeat promo result = %+v; first = %+v", repeat, first)
	}
	summary, err = users.Chances(ctx, 42)
	if err != nil || summary.Earned != 2 {
		t.Fatalf("active days after promo=%d, err=%v", summary.Earned, err)
	}

	if err := users.EnsureUser(ctx, 43); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	concurrentResults := make(chan domain.PromoResult, 2)
	concurrentErrors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := promos.IssuePromo(ctx, 43)
			if err != nil {
				concurrentErrors <- err
				return
			}
			concurrentResults <- result
		}()
	}
	wg.Wait()
	close(concurrentResults)
	close(concurrentErrors)
	for err := range concurrentErrors {
		t.Fatal(err)
	}
	var concurrentCodes []string
	var concurrentOutcomes []domain.PromoOutcome
	for result := range concurrentResults {
		concurrentCodes = append(concurrentCodes, result.Code)
		concurrentOutcomes = append(concurrentOutcomes, result.Outcome)
	}
	if len(concurrentCodes) != 2 || concurrentCodes[0] == "" || concurrentCodes[0] != concurrentCodes[1] ||
		!hasOutcome(concurrentOutcomes, domain.PromoIssued) || !hasOutcome(concurrentOutcomes, domain.PromoAlreadyIssued) {
		t.Fatalf("concurrent results: codes=%v, outcomes=%v", concurrentCodes, concurrentOutcomes)
	}

	for _, vkID := range []int64{44, 45, 46, 47} {
		if err := users.EnsureUser(ctx, vkID); err != nil {
			t.Fatal(err)
		}
		result, err := promos.IssuePromo(ctx, vkID)
		if err != nil {
			t.Fatal(err)
		}
		want := domain.PromoIssued
		if vkID == 47 {
			want = domain.PromoExhausted
		}
		if result.Outcome != want {
			t.Fatalf("VK user %d promo outcome=%q, want %q", vkID, result.Outcome, want)
		}
	}
	var issuedCodes, issuedRequests, repeatRequests, exhaustedRequests int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE c.assigned_user_id IS NOT NULL)
		FROM promo_codes AS c
		JOIN partner_days AS d ON d.id = c.partner_day_id
		WHERE d.event_date = (now() AT TIME ZONE 'Europe/Moscow')::date
	`).Scan(&issuedCodes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE outcome = 'issued'),
		       COUNT(*) FILTER (WHERE outcome = 'already_issued'),
		       COUNT(*) FILTER (WHERE outcome = 'exhausted')
		FROM promo_requests
	`).Scan(&issuedRequests, &repeatRequests, &exhaustedRequests); err != nil {
		t.Fatal(err)
	}
	if issuedCodes != 5 || issuedRequests != 5 || repeatRequests != 2 || exhaustedRequests != 1 {
		t.Fatalf("promo counts: assigned=%d, issued=%d, repeated=%d, exhausted=%d",
			issuedCodes, issuedRequests, repeatRequests, exhaustedRequests)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE partner_days SET promo_valid_until = now() - interval '1 minute'
		WHERE event_date = (now() AT TIME ZONE 'Europe/Moscow')::date
	`); err != nil {
		t.Fatal(err)
	}
	if err := users.EnsureUser(ctx, 50); err != nil {
		t.Fatal(err)
	}
	expired, err := promos.IssuePromo(ctx, 50)
	if err != nil || expired.Outcome != domain.PromoUnavailable {
		t.Fatalf("expired promo=%+v, err=%v", expired, err)
	}
	repeat, err = promos.IssuePromo(ctx, 42)
	if err != nil || repeat.Outcome != domain.PromoAlreadyIssued || repeat.Code != first.Code {
		t.Fatalf("assigned promo after expiry=%+v, err=%v", repeat, err)
	}
	summary, err = users.Chances(ctx, 50)
	if err != nil || summary.Earned != 0 {
		t.Fatalf("active days after expired promo=%d, err=%v", summary.Earned, err)
	}

	raffles := NewRaffles(pool)
	partner, err := raffles.TodayPartner(ctx)
	if err != nil || partner == nil || partner.Name == "" || partner.AccountURL == "" || partner.PrizeName == "" {
		t.Fatalf("today's partner=%+v, err=%v", partner, err)
	}
	repo := struct {
		*Users
		*Promos
		*Raffles
	}{users, promos, raffles}
	bot := service.New(repo, subscription.NewStubChecker())
	progress, err := bot.MyChances(ctx, 42)
	if err != nil || progress.Earned != 2 || progress.Maximum != domain.MaxChances ||
		progress.PassedDays+progress.RemainingDays != 11 || len(progress.ActiveDays) != 2 ||
		progress.ActiveDays[0].PartnerName == "" || progress.ActiveDays[1].PartnerName != "Partner" {
		t.Fatalf("chance progress=%+v, err=%v", progress, err)
	}
	joined, err := bot.JoinRaffle(ctx, 42)
	if err != nil || joined.Outcome != domain.RaffleJoined || joined.Chances != 2 {
		t.Fatalf("raffle after promo=%+v, err=%v", joined, err)
	}
	repeatedJoin, err := bot.JoinRaffle(ctx, 42)
	if err != nil || repeatedJoin.Outcome != domain.RaffleAlreadyJoined || repeatedJoin.Chances != 2 {
		t.Fatalf("repeated raffle=%+v, err=%v", repeatedJoin, err)
	}
	summary, err = users.Chances(ctx, 42)
	if err != nil || summary.Earned != 2 {
		t.Fatalf("active days after promo and raffle=%d, err=%v", summary.Earned, err)
	}
	joined, err = bot.JoinRaffle(ctx, 47)
	if err != nil || joined.Outcome != domain.RaffleJoined || joined.Chances != 1 {
		t.Fatalf("raffle after exhausted promo=%+v, err=%v", joined, err)
	}

	if err := users.EnsureUser(ctx, 48); err != nil {
		t.Fatal(err)
	}
	concurrentRaffles := make(chan domain.RaffleResult, 2)
	concurrentErrors = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := bot.JoinRaffle(ctx, 48)
			if err != nil {
				concurrentErrors <- err
				return
			}
			concurrentRaffles <- result
		}()
	}
	wg.Wait()
	close(concurrentRaffles)
	close(concurrentErrors)
	for err := range concurrentErrors {
		t.Fatal(err)
	}
	var raffleOutcomes []domain.RaffleOutcome
	for result := range concurrentRaffles {
		raffleOutcomes = append(raffleOutcomes, result.Outcome)
	}
	if len(raffleOutcomes) != 2 || !hasRaffleOutcome(raffleOutcomes, domain.RaffleJoined) ||
		!hasRaffleOutcome(raffleOutcomes, domain.RaffleAlreadyJoined) {
		t.Fatalf("concurrent raffle outcomes=%v", raffleOutcomes)
	}

	if err := users.EnsureUser(ctx, 49); err != nil {
		t.Fatal(err)
	}
	denied := service.New(repo, deniedChecker{})
	notSubscribed, err := denied.JoinRaffle(ctx, 49)
	if err != nil || notSubscribed.Outcome != domain.RaffleUnsubscribed {
		t.Fatalf("denied raffle=%+v, err=%v", notSubscribed, err)
	}
	summary, err = users.Chances(ctx, 49)
	if err != nil || summary.Earned != 0 {
		t.Fatalf("active days without subscription=%d, err=%v", summary.Earned, err)
	}
	repeatedJoin, err = denied.JoinRaffle(ctx, 42)
	if err != nil || repeatedJoin.Outcome != domain.RaffleAlreadyJoined {
		t.Fatalf("existing entry after lost subscription=%+v, err=%v", repeatedJoin, err)
	}
}

func hasOutcome(outcomes []domain.PromoOutcome, want domain.PromoOutcome) bool {
	for _, outcome := range outcomes {
		if outcome == want {
			return true
		}
	}
	return false
}

func hasRaffleOutcome(outcomes []domain.RaffleOutcome, want domain.RaffleOutcome) bool {
	for _, outcome := range outcomes {
		if outcome == want {
			return true
		}
	}
	return false
}

type deniedChecker struct{}

func (deniedChecker) IsSubscribed(context.Context, int64, string) (bool, error) {
	return false, nil
}
