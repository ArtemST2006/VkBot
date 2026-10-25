package domain

import "time"

// Stats contains a snapshot of the demo campaign's aggregate metrics.
type Stats struct {
	UsersStarted  int64
	PromoRequests int64
	PromosIssued  int64
	NewUsers      []DailyUsers
	Partners      []PartnerStats
	Draws         []DrawStats
}

type DailyUsers struct {
	Date  time.Time
	Count int64
}

type PartnerStats struct {
	Date           time.Time
	PartnerName    string
	PromoRequests  int64
	PromosIssued   int64
	PromosLeft     int64
	RaffleEntrants int64
}

type DrawStats struct {
	ID           int64
	Kind         string
	Date         time.Time
	DrawnAt      time.Time
	PartnerName  string
	Tickets      int64
	Participants int64
	Winners      int64
	Reserves     int64
	Approved     bool
}
