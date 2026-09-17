package model

import "time"

// InviteResetRequest — тело POST /api/v1/admin/invite/reset.
type InviteResetRequest struct {
	// Password — тот же секрет, что ADMIN_TOKEN.
	Password string `json:"password" binding:"required" example:"change-me"`
}

// InviteResetResponse — результат очистки данных открытки.
type InviteResetResponse struct {
	Message         string `json:"message" example:"ok"`
	SessionsDeleted int64  `json:"sessions_deleted" example:"1"`
	EventsDeleted   int64  `json:"events_deleted" example:"12"`
	RSVPDeleted     int64  `json:"rsvp_deleted" example:"1"`
}

// InviteEventRequest — тело POST /invite/api/events.
type InviteEventRequest struct {
	Type    string         `json:"type" binding:"required"`
	Payload map[string]any `json:"payload"`
}

// InviteRSVPRequest — тело POST /invite/api/rsvp.
type InviteRSVPRequest struct {
	Answer   string   `json:"answer" binding:"required"`
	DatePref string   `json:"date_pref"`
	TimePref string   `json:"time_pref"`
	Vibe     []string `json:"vibe"`
	Food     []string `json:"food"`
	Meet     string   `json:"meet"`
	Notes    string   `json:"notes"`
}

// InviteRSVPResponse — сохранённый RSVP.
type InviteRSVPResponse struct {
	Answer    string    `json:"answer"`
	DatePref  string    `json:"date_pref"`
	TimePref  string    `json:"time_pref"`
	Vibe      []string  `json:"vibe"`
	Food      []string  `json:"food"`
	Meet      string    `json:"meet"`
	Notes     string    `json:"notes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InviteAdminStats — сводка для секретной админ-страницы.
type InviteAdminStats struct {
	CountdownOpens   int64           `json:"countdown_opens"`
	CardOpens        int64           `json:"card_opens"`
	HeartbeatCount   int64           `json:"heartbeat_count"`
	HeartbeatMinutes float64         `json:"heartbeat_minutes"`
	NearReveal       bool            `json:"near_reveal"`
	GateOpens        int64           `json:"gate_opens"`
	Sessions         int64           `json:"sessions"`
	RSVPs            []InviteRSVPDTO `json:"rsvps"`
	Events           []InviteEvent   `json:"events"`
	GateURL          string          `json:"gate_url"`
	PageURL          string          `json:"page_url"`
	RevealAt         string          `json:"reveal_at"`
	Timezone         string          `json:"timezone"`
	Now              string          `json:"now"`
	Revealed         bool            `json:"revealed"`
}

// InviteRSVPDTO — RSVP в админ-сводке.
type InviteRSVPDTO struct {
	SessionID string    `json:"session_id"`
	Answer    string    `json:"answer"`
	DatePref  string    `json:"date_pref"`
	TimePref  string    `json:"time_pref"`
	Vibe      []string  `json:"vibe"`
	Food      []string  `json:"food"`
	Meet      string    `json:"meet"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InviteEvent — одно событие в ленте.
type InviteEvent struct {
	ID        int64          `json:"id"`
	SessionID string         `json:"session_id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}
