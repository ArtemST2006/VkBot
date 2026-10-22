package service

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
)

type drawRepositoryStub struct {
	days  []time.Time
	calls []string
}

func (s *drawRepositoryStub) DueDraws(context.Context) ([]time.Time, bool, error) {
	return s.days, true, nil
}

func (s *drawRepositoryStub) DrawDaily(_ context.Context, day time.Time) (domain.DrawSummary, error) {
	s.calls = append(s.calls, day.Format("2006-01-02"))
	return domain.DrawSummary{ID: int64(len(s.calls))}, nil
}

func (s *drawRepositoryStub) DrawFinal(context.Context) (domain.DrawSummary, error) {
	s.calls = append(s.calls, "final")
	return domain.DrawSummary{ID: int64(len(s.calls))}, nil
}

func TestRunDueDrawsCatchesUpBeforeFinal(t *testing.T) {
	repo := &drawRepositoryStub{days: []time.Time{
		time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := runDueDraws(context.Background(), repo, log); err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-20", "2026-10-21", "final"}
	if !reflect.DeepEqual(repo.calls, want) {
		t.Fatalf("draw order = %v, want %v", repo.calls, want)
	}
}
