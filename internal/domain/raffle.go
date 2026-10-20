package domain

type PartnerDay struct {
	ID         int64
	Slug       string
	Name       string
	AccountURL string
	PrizeName  string
}

type RaffleOutcome string

const (
	RaffleJoined        RaffleOutcome = "joined"
	RaffleAlreadyJoined RaffleOutcome = "already_joined"
	RaffleUnsubscribed  RaffleOutcome = "unsubscribed"
	RaffleUnavailable   RaffleOutcome = "unavailable"
)

type RaffleResult struct {
	Outcome     RaffleOutcome
	PartnerName string
	AccountURL  string
	Chances     int
}
