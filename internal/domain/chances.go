package domain

import "time"

const (
	MaxChances       = 10
	GrandPrize       = "два билета на концерт Канье Уэста"
	FinalWinnerCount = 5
)

type ActiveDay struct {
	Date        time.Time
	PartnerName string
}

type Chances struct {
	Earned        int
	Maximum       int
	PassedDays    int
	RemainingDays int
	ActiveDays    []ActiveDay
}
