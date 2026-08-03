package dao

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TitleDAO — доступ к таблице user_titles.
type TitleDAO struct {
	pool *pgxpool.Pool
}

// NewTitleDAO создаёт TitleDAO.
func NewTitleDAO(pool *pgxpool.Pool) *TitleDAO {
	return &TitleDAO{pool: pool}
}

// Upsert устанавливает или обновляет заголовок пользователя.
func (d *TitleDAO) Upsert(ctx context.Context, userID int64, title string) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO user_titles (user_id, title)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET title = EXCLUDED.title`,
		userID, title,
	)
	if err != nil {
		return fmt.Errorf("title upsert: %w", err)
	}
	return nil
}

// GetByUserID возвращает заголовок пользователя.
func (d *TitleDAO) GetByUserID(ctx context.Context, userID int64) (string, error) {
	var title string
	err := d.pool.QueryRow(ctx,
		`SELECT title FROM user_titles WHERE user_id = $1`, userID,
	).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("title get: %w", err)
	}
	return title, nil
}
