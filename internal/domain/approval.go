package domain

type PendingDraw struct {
	ID          int64
	Kind        string
	Date        string
	PartnerName string
	Tickets     int
	WinnerIDs   string
	ReserveIDs  string
}

type ApprovalOutcome string

const (
	DrawApproved        ApprovalOutcome = "approved"
	DrawAlreadyApproved ApprovalOutcome = "already_approved"
	DrawNotFound        ApprovalOutcome = "not_found"
)
