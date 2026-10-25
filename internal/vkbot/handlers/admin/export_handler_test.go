package admin

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

func TestEncodeStatsCSV(t *testing.T) {
	date := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	contents, err := EncodeStatsCSV(domain.Stats{
		UsersStarted: 5, PromoRequests: 8, PromosIssued: 3,
		NewUsers: []domain.DailyUsers{{Date: date, Count: 2}},
		Partners: []domain.PartnerStats{{Date: date, PartnerName: "Кофейня; Центр", PromoRequests: 8,
			PromosIssued: 3, PromosLeft: 4, RaffleEntrants: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(contents, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("CSV has no UTF-8 BOM")
	}
	reader := csv.NewReader(strings.NewReader(string(contents[3:])))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][4] != "8" || rows[1][5] != "3" || rows[1][8] != "5" ||
		rows[2][1] != "2026-10-24" || rows[2][2] != "Кофейня; Центр" || rows[2][3] != "2" ||
		rows[2][4] != "8" || rows[2][5] != "3" || rows[2][6] != "4" || rows[2][7] != "2" {
		t.Fatalf("CSV rows=%q", rows)
	}
}

func TestEncodeDrawsCSV(t *testing.T) {
	contents, err := EncodeDrawsCSV([]domain.DrawStats{{
		ID: 7, Kind: "daily", Date: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC),
		DrawnAt: time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC), PartnerName: "Кофейня",
		Tickets: 3, Participants: 2, Winners: 1, Reserves: 1, Approved: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(contents[3:])))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "7" || rows[1][1] != "daily" ||
		rows[1][4] != "2026-10-25 01:30:00" || rows[1][5] != "3" ||
		rows[1][6] != "2" || rows[1][7] != "1" || rows[1][8] != "1" || rows[1][9] != "да" {
		t.Fatalf("draw CSV rows=%q", rows)
	}
}
