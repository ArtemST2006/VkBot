package service

// Repositories lists the storage operations needed by the application layer.
type Repositories interface {
	UserRepository
}

// Services groups application services for transport wiring.
type Services struct {
	*Service
}

func New(repo Repositories, subscriptions SubscriptionChecker) *Services {
	return &Services{Service: NewService(repo, subscriptions)}
}
