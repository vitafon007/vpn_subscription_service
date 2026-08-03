package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound возвращается, когда запись в БД не найдена.
var ErrNotFound = errors.New("dao: not found")

// User — строка таблицы users.
type User struct {
	// ID — первичный ключ.
	ID int64
	// Login — уникальный логин (email 3x-ui или default).
	Login string
	// SubToken — токен публичной подписки (nil у default).
	SubToken *string
	// XUISubID — subId клиента в 3x-ui.
	XUISubID *string
	// XUIEmail — email клиента в 3x-ui.
	XUIEmail *string
	// CreatedAt — время создания.
	CreatedAt time.Time
	// UpdatedAt — время обновления.
	UpdatedAt time.Time
}

// UserDAO — доступ к таблице users.
type UserDAO struct {
	pool *pgxpool.Pool
}

// NewUserDAO создаёт UserDAO.
func NewUserDAO(pool *pgxpool.Pool) *UserDAO {
	return &UserDAO{pool: pool}
}

// Create вставляет нового пользователя.
func (d *UserDAO) Create(ctx context.Context, login string, subToken, xuiSubID, xuiEmail *string) (User, error) {
	var u User
	err := d.pool.QueryRow(ctx, `
		INSERT INTO users (login, sub_token, xui_sub_id, xui_email)
		VALUES ($1, $2, $3, $4)
		RETURNING id, login, sub_token, xui_sub_id, xui_email, created_at, updated_at`,
		login, subToken, xuiSubID, xuiEmail,
	).Scan(&u.ID, &u.Login, &u.SubToken, &u.XUISubID, &u.XUIEmail, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("user create: %w", err)
	}
	return u, nil
}

// UpsertBind создаёт пользователя или обновляет xui-поля, сохраняя существующий sub_token.
func (d *UserDAO) UpsertBind(ctx context.Context, login, subToken, xuiSubID, xuiEmail string) (User, error) {
	var u User
	err := d.pool.QueryRow(ctx, `
		INSERT INTO users (login, sub_token, xui_sub_id, xui_email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (login) DO UPDATE SET
			xui_sub_id = EXCLUDED.xui_sub_id,
			xui_email  = EXCLUDED.xui_email,
			updated_at = now()
		RETURNING id, login, sub_token, xui_sub_id, xui_email, created_at, updated_at`,
		login, subToken, xuiSubID, xuiEmail,
	).Scan(&u.ID, &u.Login, &u.SubToken, &u.XUISubID, &u.XUIEmail, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("user upsert bind: %w", err)
	}
	return u, nil
}

// GetByLogin возвращает пользователя по логину.
func (d *UserDAO) GetByLogin(ctx context.Context, login string) (User, error) {
	return d.scanOne(ctx, `
		SELECT id, login, sub_token, xui_sub_id, xui_email, created_at, updated_at
		FROM users WHERE login = $1`, login)
}

// GetByToken возвращает пользователя по токену подписки.
func (d *UserDAO) GetByToken(ctx context.Context, token string) (User, error) {
	return d.scanOne(ctx, `
		SELECT id, login, sub_token, xui_sub_id, xui_email, created_at, updated_at
		FROM users WHERE sub_token = $1`, token)
}

// GetDefault возвращает служебного пользователя default.
func (d *UserDAO) GetDefault(ctx context.Context) (User, error) {
	return d.GetByLogin(ctx, "default")
}

// EnsureDefault гарантирует наличие пользователя default и возвращает его.
func (d *UserDAO) EnsureDefault(ctx context.Context) (User, error) {
	u, err := d.GetDefault(ctx)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	return d.Create(ctx, "default", nil, nil, nil)
}

func (d *UserDAO) scanOne(ctx context.Context, query string, args ...any) (User, error) {
	var u User
	err := d.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Login, &u.SubToken, &u.XUISubID, &u.XUIEmail, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("user query: %w", err)
	}
	return u, nil
}
