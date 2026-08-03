package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Announce — строка таблицы user_announces.
type Announce struct {
	// ID — первичный ключ.
	ID int64
	// UserID — владелец анонса.
	UserID int64
	// Body — текст анонса.
	Body string
	// LastShownAt — время последней выдачи.
	LastShownAt *time.Time
	// CreatedAt — время создания.
	CreatedAt time.Time
}

// AnnounceDAO — доступ к таблице user_announces.
type AnnounceDAO struct {
	pool *pgxpool.Pool
}

// NewAnnounceDAO создаёт AnnounceDAO.
func NewAnnounceDAO(pool *pgxpool.Pool) *AnnounceDAO {
	return &AnnounceDAO{pool: pool}
}

// Create добавляет анонс пользователю.
func (d *AnnounceDAO) Create(ctx context.Context, userID int64, body string) (Announce, error) {
	var a Announce
	err := d.pool.QueryRow(ctx, `
		INSERT INTO user_announces (user_id, body)
		VALUES ($1, $2)
		RETURNING id, user_id, body, last_shown_at, created_at`,
		userID, body,
	).Scan(&a.ID, &a.UserID, &a.Body, &a.LastShownAt, &a.CreatedAt)
	if err != nil {
		return Announce{}, fmt.Errorf("announce create: %w", err)
	}
	return a, nil
}

// ListByUserID возвращает все анонсы пользователя.
func (d *AnnounceDAO) ListByUserID(ctx context.Context, userID int64) ([]Announce, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, user_id, body, last_shown_at, created_at
		FROM user_announces
		WHERE user_id = $1
		ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("announce list: %w", err)
	}
	defer rows.Close()

	var out []Announce
	for rows.Next() {
		var a Announce
		if err := rows.Scan(&a.ID, &a.UserID, &a.Body, &a.LastShownAt, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("announce list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Delete удаляет анонс по id; проверяет принадлежность userID.
func (d *AnnounceDAO) Delete(ctx context.Context, userID, announceID int64) error {
	tag, err := d.pool.Exec(ctx,
		`DELETE FROM user_announces WHERE id = $1 AND user_id = $2`,
		announceID, userID,
	)
	if err != nil {
		return fmt.Errorf("announce delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PickNextAndMarkShown выбирает следующий анонс (NULLS FIRST по last_shown_at)
// в транзакции с FOR UPDATE и обновляет last_shown_at = now().
func (d *AnnounceDAO) PickNextAndMarkShown(ctx context.Context, userID int64) (Announce, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return Announce{}, fmt.Errorf("announce pick begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var a Announce
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, body, last_shown_at, created_at
		FROM user_announces
		WHERE user_id = $1
		ORDER BY last_shown_at NULLS FIRST, id
		LIMIT 1
		FOR UPDATE`, userID,
	).Scan(&a.ID, &a.UserID, &a.Body, &a.LastShownAt, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Announce{}, ErrNotFound
	}
	if err != nil {
		return Announce{}, fmt.Errorf("announce pick: %w", err)
	}

	err = tx.QueryRow(ctx, `
		UPDATE user_announces
		SET last_shown_at = now()
		WHERE id = $1
		RETURNING last_shown_at`, a.ID,
	).Scan(&a.LastShownAt)
	if err != nil {
		return Announce{}, fmt.Errorf("announce mark shown: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Announce{}, fmt.Errorf("announce pick commit: %w", err)
	}
	return a, nil
}
