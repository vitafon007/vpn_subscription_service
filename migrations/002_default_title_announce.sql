-- Сид title/announce для default (для БД, где уже применена 001_init без контента).
INSERT INTO user_titles (user_id, title)
SELECT id, 'Окно в европу для избранных ★ '
FROM users
WHERE login = 'default'
ON CONFLICT (user_id) DO UPDATE
SET title = EXCLUDED.title;

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
