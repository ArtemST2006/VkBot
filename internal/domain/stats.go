package domain

import "time"

// Stats contains a snapshot of the demo campaign's aggregate metrics.
type Stats struct {
	UsersStarted  int64
	PromoRequests int64
	PromosIssued  int64
	NewUsers      []DailyUsers
	Partners      []PartnerStats
}

type DailyUsers struct {
	Date  time.Time
	Count int64
}

type PartnerStats struct {
	Date           time.Time
	PartnerName    string
	PromosLeft     int64
	RaffleEntrants int64
}
