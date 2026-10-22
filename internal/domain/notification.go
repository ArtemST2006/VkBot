package domain

type Notification struct {
	ID       int64
	UserID   int64
	Attempts int
	Message  string
	Action   string
}
