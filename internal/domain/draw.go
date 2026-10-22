package domain

import (
	"crypto/rand"
	"io"
	"math/big"
	"sort"
	"time"
)

type DrawSummary struct {
	ID           int64     `json:"id"`
	Kind         string    `json:"kind"`
	Date         time.Time `json:"date,omitempty"`
	Tickets      int       `json:"tickets"`
	Participants int       `json:"participants"`
	Winners      int       `json:"winners"`
	Reserves     int       `json:"reserves"`
}

type DrawTicket struct {
	UserID int64 `json:"user_id"`
	DayID  int64 `json:"day_id"`
}

// SelectWinners draws distinct users. Repeated IDs are extra tickets, not extra prizes.
func SelectWinners(random io.Reader, tickets []int64, count int) ([]int64, error) {
	weights := make(map[int64]int)
	for _, userID := range tickets {
		weights[userID]++
	}
	users := make([]int64, 0, len(weights))
	for userID := range weights {
		users = append(users, userID)
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })
	if count > len(users) {
		count = len(users)
	}
	selected := make([]int64, 0, count)
	total := len(tickets)
	for range count {
		n, err := rand.Int(random, big.NewInt(int64(total)))
		if err != nil {
			return nil, err
		}
		pick := int(n.Int64())
		for i, userID := range users {
			pick -= weights[userID]
			if pick < 0 {
				selected = append(selected, userID)
				total -= weights[userID]
				users = append(users[:i], users[i+1:]...)
				break
			}
		}
	}
	return selected, nil
}
