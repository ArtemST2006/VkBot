package domain

import "time"

type PromoOutcome string

const (
	PromoIssued        PromoOutcome = "issued"
	PromoAlreadyIssued PromoOutcome = "already_issued"
	PromoExhausted     PromoOutcome = "exhausted"
	PromoUnavailable   PromoOutcome = "unavailable"
)

type PromoResult struct {
	Outcome      PromoOutcome
	PartnerName  string
	Code         string
	ValidUntil   time.Time
	Instructions string
}
