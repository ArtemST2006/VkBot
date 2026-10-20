// Package storage assembles the repositories used by application services.
package storage

import (
	"github.com/ArtemST2006/VkBot/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository groups storage implementations in one dependency, as in GorkyCode.
type Repository struct {
	*postgres.Users
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Users: postgres.NewUsers(pool)}
}
