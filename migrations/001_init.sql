-- Пользователи подписок (login = email клиента в 3x-ui; default — служебный).
CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL PRIMARY KEY,
    login      TEXT        NOT NULL UNIQUE,
    sub_token  TEXT        UNIQUE,
    xui_sub_id TEXT,
    xui_email  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индивидуальный заголовок подписки (один на пользователя).
CREATE TABLE IF NOT EXISTS user_titles (
    user_id BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    title   TEXT NOT NULL
);

-- Анонсы с round-robin по last_shown_at.
CREATE TABLE IF NOT EXISTS user_announces (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body          TEXT        NOT NULL,
    last_shown_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_announces_user_shown
    ON user_announces (user_id, last_shown_at NULLS FIRST, id);

-- Служебный пользователь для fallback title/announce (без подписки).
INSERT INTO users (login, sub_token, xui_sub_id, xui_email)
VALUES ('default', NULL, NULL, NULL)
ON CONFLICT (login) DO NOTHING;

INSERT INTO user_titles (user_id, title)
SELECT id, 'Окно в европу для избранных ★ '
FROM users
WHERE login = 'default'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_announces (user_id, body)
SELECT u.id, 'Оплата натурой еженедельно!'
FROM users u
WHERE u.login = 'default'
  AND NOT EXISTS (
    SELECT 1
    FROM user_announces a
    WHERE a.user_id = u.id
      AND a.body = 'Оплата натурой еженедельно!'
  );
