package domain

import "errors"

var ErrInvalidFullName = errors.New("enter first and last name")

type ClaimStatus string

const (
	ClaimNotWinner       ClaimStatus = "not_winner"
	ClaimExpired         ClaimStatus = "expired"
	ClaimOtherOpen       ClaimStatus = "other_open"
	ClaimAwaitingName    ClaimStatus = "awaiting_name"
	ClaimAwaitingConsent ClaimStatus = "awaiting_consent"
	ClaimCompleted       ClaimStatus = "completed"
)
