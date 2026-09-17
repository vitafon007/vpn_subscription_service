package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InviteSession — строка invite_sessions.
type InviteSession struct {
	ID         string
	CreatedAt  time.Time
	LastSeenAt time.Time
	UserAgent  string
	IP         string
}

// InviteEventRow — строка invite_events.
type InviteEventRow struct {
	ID        int64
	SessionID string
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

// InviteRSVP — строка invite_rsvp.
type InviteRSVP struct {
	SessionID string
	Answer    string
	DatePref  string
	TimePref  string
	Vibe      []string
	Food      []string
	Meet      string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InviteDAO — доступ к таблицам приглашения.
type InviteDAO struct {
	pool *pgxpool.Pool
}

// NewInviteDAO создаёт InviteDAO.
func NewInviteDAO(pool *pgxpool.Pool) *InviteDAO {
	return &InviteDAO{pool: pool}
}

// UpsertSession создаёт сессию или обновляет last_seen / UA / IP.
func (d *InviteDAO) UpsertSession(ctx context.Context, id, userAgent, ip string) (InviteSession, error) {
	var s InviteSession
	err := d.pool.QueryRow(ctx, `
		INSERT INTO invite_sessions (id, user_agent, ip)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			last_seen_at = now(),
			user_agent   = CASE WHEN EXCLUDED.user_agent <> '' THEN EXCLUDED.user_agent ELSE invite_sessions.user_agent END,
			ip           = CASE WHEN EXCLUDED.ip <> '' THEN EXCLUDED.ip ELSE invite_sessions.ip END
		RETURNING id, created_at, last_seen_at, user_agent, ip`,
		id, userAgent, ip,
	).Scan(&s.ID, &s.CreatedAt, &s.LastSeenAt, &s.UserAgent, &s.IP)
	if err != nil {
		return InviteSession{}, fmt.Errorf("invite session upsert: %w", err)
	}
	return s, nil
}

// TouchSession обновляет last_seen_at.
func (d *InviteDAO) TouchSession(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE invite_sessions SET last_seen_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("invite session touch: %w", err)
	}
	return nil
}

// InsertEvent пишет событие аналитики.
func (d *InviteDAO) InsertEvent(ctx context.Context, sessionID, eventType string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("invite event marshal: %w", err)
	}
	_, err = d.pool.Exec(ctx, `
		INSERT INTO invite_events (session_id, event_type, payload)
		VALUES ($1, $2, $3::jsonb)`, sessionID, eventType, raw)
	if err != nil {
		return fmt.Errorf("invite event insert: %w", err)
	}
	return nil
}

// UpsertRSVP сохраняет или обновляет ответ.
func (d *InviteDAO) UpsertRSVP(ctx context.Context, r InviteRSVP) (InviteRSVP, error) {
	if r.Vibe == nil {
		r.Vibe = []string{}
	}
	if r.Food == nil {
		r.Food = []string{}
	}
	var out InviteRSVP
	err := d.pool.QueryRow(ctx, `
		INSERT INTO invite_rsvp (session_id, answer, date_pref, time_pref, vibe, food, meet, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id) DO UPDATE SET
			answer     = EXCLUDED.answer,
			date_pref  = EXCLUDED.date_pref,
			time_pref  = EXCLUDED.time_pref,
			vibe       = EXCLUDED.vibe,
			food       = EXCLUDED.food,
			meet       = EXCLUDED.meet,
			notes      = EXCLUDED.notes,
			updated_at = now()
		RETURNING session_id, answer, date_pref, time_pref, vibe, food, meet, notes, created_at, updated_at`,
		r.SessionID, r.Answer, r.DatePref, r.TimePref, r.Vibe, r.Food, r.Meet, r.Notes,
	).Scan(
		&out.SessionID, &out.Answer, &out.DatePref, &out.TimePref,
		&out.Vibe, &out.Food, &out.Meet, &out.Notes, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return InviteRSVP{}, fmt.Errorf("invite rsvp upsert: %w", err)
	}
	if out.Vibe == nil {
		out.Vibe = []string{}
	}
	if out.Food == nil {
		out.Food = []string{}
	}
	return out, nil
}

// GetRSVPBySession возвращает RSVP сессии.
func (d *InviteDAO) GetRSVPBySession(ctx context.Context, sessionID string) (InviteRSVP, error) {
	var r InviteRSVP
	err := d.pool.QueryRow(ctx, `
		SELECT session_id, answer, date_pref, time_pref, vibe, food, meet, notes, created_at, updated_at
		FROM invite_rsvp WHERE session_id = $1`, sessionID,
	).Scan(
		&r.SessionID, &r.Answer, &r.DatePref, &r.TimePref,
		&r.Vibe, &r.Food, &r.Meet, &r.Notes, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return InviteRSVP{}, ErrNotFound
		}
		return InviteRSVP{}, fmt.Errorf("invite rsvp get: %w", err)
	}
	if r.Vibe == nil {
		r.Vibe = []string{}
	}
	if r.Food == nil {
		r.Food = []string{}
	}
	return r, nil
}

// LatestRSVP возвращает последний обновлённый RSVP (для админки).
func (d *InviteDAO) LatestRSVP(ctx context.Context) (InviteRSVP, error) {
	var r InviteRSVP
	err := d.pool.QueryRow(ctx, `
		SELECT session_id, answer, date_pref, time_pref, vibe, food, meet, notes, created_at, updated_at
		FROM invite_rsvp
		ORDER BY updated_at DESC
		LIMIT 1`,
	).Scan(
		&r.SessionID, &r.Answer, &r.DatePref, &r.TimePref,
		&r.Vibe, &r.Food, &r.Meet, &r.Notes, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return InviteRSVP{}, ErrNotFound
		}
		return InviteRSVP{}, fmt.Errorf("invite rsvp latest: %w", err)
	}
	if r.Vibe == nil {
		r.Vibe = []string{}
	}
	if r.Food == nil {
		r.Food = []string{}
	}
	return r, nil
}

// ListEvents возвращает последние N событий.
func (d *InviteDAO) ListEvents(ctx context.Context, limit int) ([]InviteEventRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.pool.Query(ctx, `
		SELECT id, session_id, event_type, payload, created_at
		FROM invite_events
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("invite events list: %w", err)
	}
	defer rows.Close()

	var out []InviteEventRow
	for rows.Next() {
		var e InviteEventRow
		if err := rows.Scan(&e.ID, &e.SessionID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("invite events scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountEventsByType считает события по типу.
func (d *InviteDAO) CountEventsByType(ctx context.Context, eventType string) (int64, error) {
	var n int64
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM invite_events WHERE event_type = $1`, eventType,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("invite count %s: %w", eventType, err)
	}
	return n, nil
}

// CountSessions считает сессии.
func (d *InviteDAO) CountSessions(ctx context.Context) (int64, error) {
	var n int64
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM invite_sessions`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("invite sessions count: %w", err)
	}
	return n, nil
}

// CountEventsNearReveal считает page_open/heartbeat в окне до reveal.
func (d *InviteDAO) CountEventsNearReveal(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM invite_events
		WHERE created_at >= $1 AND created_at < $2
		  AND event_type IN ('heartbeat', 'page_open', 'gate_open')`,
		from, to,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("invite near reveal: %w", err)
	}
	return n, nil
}

// CountPageOpenVariant считает page_open с конкретным variant в payload.
func (d *InviteDAO) CountPageOpenVariant(ctx context.Context, variant string) (int64, error) {
	var n int64
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM invite_events
		WHERE event_type = 'page_open'
		  AND payload->>'variant' = $1`, variant,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("invite page_open %s: %w", variant, err)
	}
	return n, nil
}
