package subscription

import "context"

// StubChecker marks every demo check as successful. It never contacts a partner.
type StubChecker struct{}

func NewStubChecker() StubChecker { return StubChecker{} }

func (StubChecker) IsSubscribed(ctx context.Context, _ int64, _ string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return true, nil
}
