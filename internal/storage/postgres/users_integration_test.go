package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/service"
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
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE version_id IN (1, 2, 3, 4) AND is_applied").Scan(&migrationsApplied); err != nil || migrationsApplied != 4 {
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

	repo := New(pool)
	if err := repo.EnsureUser(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureUser(ctx, 42); err != nil {
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
	if _, err := pool.Exec(ctx, "INSERT INTO raffle_entries (user_id, partner_day_id) VALUES ($1, $2)", userID, dayID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO raffle_entries (user_id, partner_day_id) VALUES ($1, $2)", userID, dayID); err == nil {
		t.Fatal("same user registered twice for one partner day")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO promo_codes (partner_day_id, code, assigned_user_id, assigned_at) VALUES ($1, 'A', $2, now())", dayID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO promo_codes (partner_day_id, code, assigned_user_id, assigned_at) VALUES ($1, 'B', $2, now())", dayID, userID); err == nil {
		t.Fatal("same user received two promo codes for one partner day")
	}
	summary, err := repo.Chances(ctx, 42)
	if err != nil || summary.Earned != 1 {
		t.Fatalf("promo and raffle in one day should give one chance: count=%d, err=%v", summary.Earned, err)
	}

	if _, err := repo.IssuePromo(ctx, 999); !errors.Is(err, domain.ErrUserNotStarted) {
		t.Fatalf("promo for a user who did not press Start: %v", err)
	}
	first, err := repo.IssuePromo(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != domain.PromoIssued || first.PartnerName == "" || first.Code == "" || first.Instructions == "" || !first.ValidUntil.After(time.Now()) {
		t.Fatalf("first promo result = %+v", first)
	}
	repeat, err := repo.IssuePromo(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Outcome != domain.PromoAlreadyIssued || repeat.Code != first.Code {
		t.Fatalf("repeat promo result = %+v; first = %+v", repeat, first)
	}
	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.UsersStarted != 1 || stats.PromoRequests != 2 || stats.PromosIssued != 2 ||
		len(stats.Partners) != 11 || len(stats.NewUsers) != 11 || stats.Partners[0].PromosLeft != 4 {
		t.Fatalf("unexpected campaign stats: %+v", stats)
	}
	var requestsByPartner, issuedByPartner int64
	for _, partnerStats := range stats.Partners {
		requestsByPartner += partnerStats.PromoRequests
		issuedByPartner += partnerStats.PromosIssued
	}
	if requestsByPartner != stats.PromoRequests || issuedByPartner != stats.PromosIssued {
		t.Fatalf("partner stats requests=%d, issued=%d; totals=%d/%d",
			requestsByPartner, issuedByPartner, stats.PromoRequests, stats.PromosIssued)
	}
	summary, err = repo.Chances(ctx, 42)
	if err != nil || summary.Earned != 2 {
		t.Fatalf("active days after promo=%d, err=%v", summary.Earned, err)
	}

	if err := repo.EnsureUser(ctx, 43); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	concurrentResults := make(chan domain.PromoResult, 2)
	concurrentErrors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := repo.IssuePromo(ctx, 43)
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
		if err := repo.EnsureUser(ctx, vkID); err != nil {
			t.Fatal(err)
		}
		result, err := repo.IssuePromo(ctx, vkID)
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
	if err := repo.EnsureUser(ctx, 50); err != nil {
		t.Fatal(err)
	}
	expired, err := repo.IssuePromo(ctx, 50)
	if err != nil || expired.Outcome != domain.PromoUnavailable {
		t.Fatalf("expired promo=%+v, err=%v", expired, err)
	}
	repeat, err = repo.IssuePromo(ctx, 42)
	if err != nil || repeat.Outcome != domain.PromoAlreadyIssued || repeat.Code != first.Code {
		t.Fatalf("assigned promo after expiry=%+v, err=%v", repeat, err)
	}
	summary, err = repo.Chances(ctx, 50)
	if err != nil || summary.Earned != 0 {
		t.Fatalf("active days after expired promo=%d, err=%v", summary.Earned, err)
	}

	partner, err := repo.TodayPartner(ctx)
	if err != nil || partner == nil || partner.Name == "" || partner.AccountURL == "" || partner.PrizeName == "" {
		t.Fatalf("today's partner=%+v, err=%v", partner, err)
	}
	bot := service.New(repo)
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
	summary, err = repo.Chances(ctx, 42)
	if err != nil || summary.Earned != 2 {
		t.Fatalf("active days after promo and raffle=%d, err=%v", summary.Earned, err)
	}
	joined, err = bot.JoinRaffle(ctx, 47)
	if err != nil || joined.Outcome != domain.RaffleJoined || joined.Chances != 1 {
		t.Fatalf("raffle after exhausted promo=%+v, err=%v", joined, err)
	}

	if err := repo.EnsureUser(ctx, 48); err != nil {
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

	if _, err := repo.DrawFinal(ctx); err == nil {
		t.Fatal("final draw started before campaign end")
	}
	var yesterday time.Time
	if err := pool.QueryRow(ctx, `
		UPDATE partner_days SET event_date = (now() AT TIME ZONE 'Europe/Moscow')::date - 1
		WHERE id = $1 RETURNING event_date
	`, partner.ID).Scan(&yesterday); err != nil {
		t.Fatal(err)
	}
	dueDays, finalDue, err := repo.DueDraws(ctx)
	if err != nil || len(dueDays) != 1 || !dueDays[0].Equal(yesterday) || finalDue {
		t.Fatalf("due draws: days=%v, final=%v, err=%v", dueDays, finalDue, err)
	}
	draw, err := repo.DrawDaily(ctx, yesterday)
	if err != nil || draw.Tickets != 3 || draw.Winners != 2 || draw.Reserves != 1 {
		t.Fatalf("daily draw=%+v, err=%v", draw, err)
	}
	if _, err := repo.DrawDaily(ctx, yesterday); err == nil {
		t.Fatal("same daily draw was created twice")
	}
	dueDays, finalDue, err = repo.DueDraws(ctx)
	if err != nil || len(dueDays) != 0 || finalDue {
		t.Fatalf("drawn day is still due: days=%v, final=%v, err=%v", dueDays, finalDue, err)
	}
	var active, reserve, queued int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE state = 'active'),
		COUNT(*) FILTER (WHERE state = 'waiting') FROM draw_results WHERE draw_id = $1`, draw.ID).Scan(&active, &reserve); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_outbox WHERE draw_id = $1`, draw.ID).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if active != draw.Winners || reserve != draw.Reserves || queued != 0 {
		t.Fatalf("draw results: active=%d, reserve=%d, queued=%d", active, reserve, queued)
	}
	statsBeforeApproval, err := repo.Stats(ctx)
	if err != nil || len(statsBeforeApproval.Draws) != 1 || statsBeforeApproval.Draws[0].ID != draw.ID ||
		statsBeforeApproval.Draws[0].Tickets != int64(draw.Tickets) ||
		statsBeforeApproval.Draws[0].Winners != int64(draw.Winners) || statsBeforeApproval.Draws[0].Approved {
		t.Fatalf("draw stats before approval=%+v, err=%v", statsBeforeApproval.Draws, err)
	}
	pendingDraws, err := repo.PendingDraws(ctx)
	if err != nil || len(pendingDraws) != 1 || pendingDraws[0].ID != draw.ID || pendingDraws[0].WinnerIDs == "" {
		t.Fatalf("pending draws=%+v, err=%v", pendingDraws, err)
	}
	if notice, err := repo.NextNotification(ctx); err != nil || notice != nil {
		t.Fatalf("notification before approval=%+v, err=%v", notice, err)
	}
	var unapprovedWinnerVKID int64
	if err := pool.QueryRow(ctx, `SELECT u.external_id::bigint FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		WHERE res.draw_id = $1 AND res.state = 'active' LIMIT 1`, draw.ID).Scan(&unapprovedWinnerVKID); err != nil {
		t.Fatal(err)
	}
	if status, err := bot.StartPrizeClaim(ctx, draw.ID, unapprovedWinnerVKID); err != nil || status != domain.ClaimNotWinner {
		t.Fatalf("claim before approval=%s, err=%v", status, err)
	}
	approval, err := repo.ApproveDraw(ctx, draw.ID, 777)
	if err != nil || approval != domain.DrawApproved {
		t.Fatalf("approve draw=%q, err=%v", approval, err)
	}
	if approval, err = repo.ApproveDraw(ctx, draw.ID, 777); err != nil || approval != domain.DrawAlreadyApproved {
		t.Fatalf("repeat approval=%q, err=%v", approval, err)
	}
	var approvedBy int64
	if err := pool.QueryRow(ctx, `SELECT approved_by_vk_id FROM draws WHERE id = $1`, draw.ID).Scan(&approvedBy); err != nil || approvedBy != 777 {
		t.Fatalf("approved by=%d, err=%v", approvedBy, err)
	}
	statsAfterApproval, err := repo.Stats(ctx)
	if err != nil || len(statsAfterApproval.Draws) != 1 || !statsAfterApproval.Draws[0].Approved {
		t.Fatalf("draw stats after approval=%+v, err=%v", statsAfterApproval.Draws, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_outbox WHERE draw_id = $1`, draw.ID).Scan(&queued); err != nil || queued != draw.Winners {
		t.Fatalf("queued after approval=%d, err=%v", queued, err)
	}
	firstNotice, err := repo.NextNotification(ctx)
	if err != nil || firstNotice == nil {
		t.Fatalf("first winner notification=%+v, err=%v", firstNotice, err)
	}
	if err := repo.MarkNotificationSent(ctx, *firstNotice); err != nil {
		t.Fatal(err)
	}
	status, err := bot.StartPrizeClaim(ctx, draw.ID, firstNotice.UserID)
	if err != nil || status != domain.ClaimAwaitingName {
		t.Fatalf("start prize claim=%s, err=%v", status, err)
	}
	if _, _, err := bot.SavePrizeName(ctx, firstNotice.UserID, "Иван"); !errors.Is(err, domain.ErrInvalidFullName) {
		t.Fatalf("invalid name error=%v", err)
	}
	claimDraw, status, err := bot.SavePrizeName(ctx, firstNotice.UserID, "Иван Иванов")
	if err != nil || claimDraw != draw.ID || status != domain.ClaimAwaitingConsent {
		t.Fatalf("save name draw=%d, status=%s, err=%v", claimDraw, status, err)
	}
	status, err = bot.ConsentPrizeClaim(ctx, draw.ID, firstNotice.UserID)
	if err != nil || status != domain.ClaimCompleted {
		t.Fatalf("consent status=%s, err=%v", status, err)
	}
	var savedName string
	if err := pool.QueryRow(ctx, `SELECT full_name FROM draw_results AS res
		JOIN users AS u ON u.id = res.user_id
		WHERE res.draw_id = $1 AND u.platform = 'vk' AND u.external_id = $2`,
		draw.ID, strconv.FormatInt(firstNotice.UserID, 10)).Scan(&savedName); err != nil || savedName != "Иван Иванов" {
		t.Fatalf("saved prize name=%q, err=%v", savedName, err)
	}
	secondNotice, err := repo.NextNotification(ctx)
	if err != nil || secondNotice == nil || secondNotice.UserID == firstNotice.UserID {
		t.Fatalf("second winner notification=%+v, err=%v", secondNotice, err)
	}
	if err := repo.MarkNotificationSent(ctx, *secondNotice); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE draw_results SET response_deadline = now() - interval '1 second'
		WHERE draw_id = $1 AND user_id = (SELECT id FROM users WHERE external_id = $2 AND platform = 'vk')
	`, draw.ID, strconv.FormatInt(secondNotice.UserID, 10)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ExpirePrizes(ctx); err != nil {
		t.Fatal(err)
	}
	reserveNotice, err := repo.NextNotification(ctx)
	if err != nil || reserveNotice == nil || reserveNotice.UserID == secondNotice.UserID {
		t.Fatalf("reserve notification=%+v, err=%v", reserveNotice, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE partner_days SET event_date = '2020-01-01' WHERE id = $1`, dayID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE partner_days SET event_date = event_date - 30 WHERE id <> $1 AND id <> $2`, partner.ID, dayID); err != nil {
		t.Fatal(err)
	}
	_, finalDue, err = repo.DueDraws(ctx)
	if err != nil || !finalDue {
		t.Fatalf("final draw should be due: due=%v, err=%v", finalDue, err)
	}
	final, err := repo.DrawFinal(ctx)
	if err != nil || final.Tickets == 0 || final.Winners == 0 {
		t.Fatalf("final draw=%+v, err=%v", final, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_outbox WHERE draw_id = $1`, final.ID).Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("final notifications before approval=%d, err=%v", queued, err)
	}
	if approval, err := repo.ApproveDraw(ctx, final.ID, 777); err != nil || approval != domain.DrawApproved {
		t.Fatalf("approve final draw=%q, err=%v", approval, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_outbox WHERE draw_id = $1`, final.ID).Scan(&queued); err != nil || queued != final.Winners {
		t.Fatalf("final notifications after approval=%d, winners=%d, err=%v", queued, final.Winners, err)
	}
	_, finalDue, err = repo.DueDraws(ctx)
	if err != nil || finalDue {
		t.Fatalf("final draw is still due: due=%v, err=%v", finalDue, err)
	}
	var emptyDay time.Time
	if err := pool.QueryRow(ctx, `SELECT d.event_date FROM partner_days AS d
		WHERE d.id <> $1 AND d.id <> $2
		  AND NOT EXISTS (SELECT 1 FROM draws WHERE partner_day_id = d.id)
		ORDER BY d.id LIMIT 1`, partner.ID, dayID).Scan(&emptyDay); err != nil {
		t.Fatal(err)
	}
	emptyDraw, err := repo.DrawDaily(ctx, emptyDay)
	if err != nil || emptyDraw.Tickets != 0 || emptyDraw.Winners != 0 {
		t.Fatalf("empty daily draw=%+v, err=%v", emptyDraw, err)
	}
	var ticketType string
	if err := pool.QueryRow(ctx, `SELECT jsonb_typeof(tickets) FROM draws WHERE id = $1`, emptyDraw.ID).Scan(&ticketType); err != nil || ticketType != "array" {
		t.Fatalf("empty draw ticket JSON type=%q, err=%v", ticketType, err)
	}
	// Older bot versions stored a JSON null when a draw had no participants.
	if _, err := pool.Exec(ctx, `UPDATE draws SET tickets = 'null'::jsonb WHERE id = $1`, emptyDraw.ID); err != nil {
		t.Fatal(err)
	}
	statsWithLegacyDraw, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("stats with legacy empty draw: %v", err)
	}
	var foundEmpty bool
	for _, drawStats := range statsWithLegacyDraw.Draws {
		if drawStats.ID == emptyDraw.ID {
			foundEmpty = true
			if drawStats.Tickets != 0 || drawStats.Participants != 0 {
				t.Fatalf("legacy empty draw stats=%+v", drawStats)
			}
		}
	}
	if !foundEmpty {
		t.Fatalf("legacy empty draw #%d missing from stats", emptyDraw.ID)
	}
	pendingWithLegacyDraw, err := repo.PendingDraws(ctx)
	if err != nil || len(pendingWithLegacyDraw) != 1 || pendingWithLegacyDraw[0].ID != emptyDraw.ID || pendingWithLegacyDraw[0].Tickets != 0 {
		t.Fatalf("pending legacy empty draw=%+v, err=%v", pendingWithLegacyDraw, err)
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
