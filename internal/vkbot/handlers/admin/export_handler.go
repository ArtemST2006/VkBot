package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"time"

	"github.com/ArtemST2006/VkBot/internal/domain"
	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

type DocumentSender interface {
	SendDocument(ctx context.Context, peerID int64, filename string, contents []byte) error
}

func NewExportStats(service StatsService, sender DocumentSender) handlers.Func {
	return func(ctx context.Context, adminVKID int64) error {
		stats, err := service.Stats(ctx)
		if err != nil {
			return err
		}
		contents, err := EncodeStatsCSV(stats)
		if err != nil {
			return err
		}
		stamp := time.Now().In(time.FixedZone("MSK", 3*60*60)).Format("2006-01-02")
		if err := sender.SendDocument(ctx, adminVKID, "stats_"+stamp+".csv", contents); err != nil {
			return err
		}
		drawsCSV, err := EncodeDrawsCSV(stats.Draws)
		if err != nil {
			return err
		}
		if err := sender.SendDocument(ctx, adminVKID, "draws_"+stamp+".csv", drawsCSV); err != nil {
			return err
		}
		return nil
	}
}

// EncodeDrawsCSV contains aggregate draw metrics without participant identifiers.
func EncodeDrawsCSV(draws []domain.DrawStats) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(&out)
	writer.Comma = ';'
	rows := [][]string{{"ID розыгрыша", "Тип", "Дата (МСК)", "Партнёр", "Проведён (МСК)",
		"Билетов", "Участников", "Победителей", "Резервных", "Подтверждён"}}
	moscow := time.FixedZone("MSK", 3*60*60)
	for _, draw := range draws {
		approved := "нет"
		if draw.Approved {
			approved = "да"
		}
		rows = append(rows, []string{strconv.FormatInt(draw.ID, 10), draw.Kind, draw.Date.Format("2006-01-02"), draw.PartnerName,
			draw.DrawnAt.In(moscow).Format("2006-01-02 15:04:05"), strconv.FormatInt(draw.Tickets, 10),
			strconv.FormatInt(draw.Participants, 10), strconv.FormatInt(draw.Winners, 10),
			strconv.FormatInt(draw.Reserves, 10), approved})
	}
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// EncodeStatsCSV uses semicolons and a UTF-8 BOM so Russian spreadsheet apps open it correctly.
func EncodeStatsCSV(stats domain.Stats) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(&out)
	writer.Comma = ';'
	rows := [][]string{{"Тип", "Дата (МСК)", "Партнёр", "Новые пользователи", "Запросы промокода", "Выдано промокодов", "Остаток промокодов", "Участники розыгрыша", "Всего пользователей"},
		{"Итого", "", "", "", strconv.FormatInt(stats.PromoRequests, 10), strconv.FormatInt(stats.PromosIssued, 10), "", "", strconv.FormatInt(stats.UsersStarted, 10)}}
	newUsersByDate := make(map[string]int64, len(stats.NewUsers))
	for _, day := range stats.NewUsers {
		newUsersByDate[day.Date.Format("2006-01-02")] = day.Count
	}
	for _, partner := range stats.Partners {
		date := partner.Date.Format("2006-01-02")
		newUsers := ""
		if count, ok := newUsersByDate[date]; ok {
			newUsers = strconv.FormatInt(count, 10)
		}
		rows = append(rows, []string{"День", date, partner.PartnerName,
			newUsers, strconv.FormatInt(partner.PromoRequests, 10), strconv.FormatInt(partner.PromosIssued, 10),
			strconv.FormatInt(partner.PromosLeft, 10), strconv.FormatInt(partner.RaffleEntrants, 10), ""})
	}
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
