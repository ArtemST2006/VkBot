// Package service contains application scenarios between VK and storage.
package service

// Repositories lists the storage operations needed by all services.
type Repositories interface {
	UserRepository
	PromoRepository
	RaffleRepository
}

// Services groups application services for transport wiring.
type Services struct {
	*UserService
	*PromoService
	*RaffleService
}

func New(repo Repositories, subscriptions SubscriptionChecker) *Services {
	return &Services{
		UserService:   NewUserService(repo),
		PromoService:  NewPromoService(repo),
		RaffleService: NewRaffleService(repo, subscriptions),
	}
}
