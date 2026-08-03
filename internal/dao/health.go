// Package dao содержит доступ к данным без знания HTTP.
package dao

import (
	"context"

	"github.com/chistotel/vpn_subscription_service/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthDAO проверяет доступность базы данных.
type HealthDAO struct {
	pool *pgxpool.Pool
}

// NewHealthDAO создаёт HealthDAO с переданным пулом соединений.
func NewHealthDAO(pool *pgxpool.Pool) *HealthDAO {
	return &HealthDAO{pool: pool}
}

// Ping выполняет ping Postgres через пул соединений.
func (d *HealthDAO) Ping(ctx context.Context) error {
	return db.Ping(ctx, d.pool)
}
