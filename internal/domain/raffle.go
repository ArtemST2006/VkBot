package domain

type PartnerDay struct {
	ID         int64
	Name       string
	AccountURL string
	PrizeName  string
}

type RaffleOutcome string

const (
	RaffleJoined        RaffleOutcome = "joined"
	RaffleAlreadyJoined RaffleOutcome = "already_joined"
	RaffleUnavailable   RaffleOutcome = "unavailable"
)

type RaffleResult struct {
	Outcome     RaffleOutcome
	PartnerName string
	AccountURL  string
	PrizeName   string
	Chances     int
}
