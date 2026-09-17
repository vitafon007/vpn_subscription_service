-- Secret invite landing: sessions, analytics events, RSVP answers.

CREATE TABLE IF NOT EXISTS invite_sessions (
    id           UUID PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent   TEXT        NOT NULL DEFAULT '',
    ip           TEXT        NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS invite_events (
    id         BIGSERIAL PRIMARY KEY,
    session_id UUID        NOT NULL REFERENCES invite_sessions(id) ON DELETE CASCADE,
    event_type TEXT        NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS invite_events_session_created_idx
    ON invite_events (session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS invite_events_type_created_idx
    ON invite_events (event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS invite_rsvp (
    session_id UUID PRIMARY KEY REFERENCES invite_sessions(id) ON DELETE CASCADE,
    answer     TEXT        NOT NULL CHECK (answer IN ('yes', 'no')),
    date_pref  TEXT        NOT NULL DEFAULT '',
    time_pref  TEXT        NOT NULL DEFAULT '',
    vibe       TEXT[]      NOT NULL DEFAULT '{}',
    food       TEXT[]      NOT NULL DEFAULT '{}',
    meet       TEXT        NOT NULL DEFAULT '',
    notes      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
