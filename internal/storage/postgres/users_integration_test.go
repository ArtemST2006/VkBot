package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE version_id = 1 AND is_applied").Scan(&migrationsApplied); err != nil || migrationsApplied != 1 {
		t.Fatalf("Goose version 1 rows=%d, err=%v", migrationsApplied, err)
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
	count, err := users.ActiveDayCount(ctx, 42)
	if err != nil || count != 1 {
		t.Fatalf("active day count=%d, err=%v", count, err)
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
}
