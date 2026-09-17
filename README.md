# VPN Subscription Service

Сервис выдачи VPN-подписок с интеграцией **3x-ui**: привязка существующих клиентов панели, публичный URL подписки и оверлей `Profile-Title` / `Announce` из Postgres.

Клиенты в 3x-ui **не создаются** — только bind по email (логин = email в панели).

## Архитектура

| Слой | Путь | Ответственность |
|------|------|-----------------|
| handler | `internal/handler` | HTTP (Gin), без доступа к БД |
| service | `internal/service` | бизнес-логика |
| dao | `internal/dao` | Postgres |
| xui | `internal/xui` | HTTP-клиент 3x-ui |
| db | `internal/db` | пул pgx + миграции |
| model | `internal/model` | DTO |
| middleware | `internal/middleware` | RequireAdmin |
| config / server | `internal/config`, `internal/server` | конфиг и роутер |

## API

Базовый путь: `/api/v1`

### Публичные

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | liveness |
| GET | `/ready` | readiness (Postgres) |
| GET | `/sub/:token` | подписка: body = base64 ссылок; заголовки Profile-Title, Announce, Subscription-Userinfo |

### Админ (`Authorization: Bearer ADMIN_TOKEN`)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/admin/users` | bind: `{ "login": "<email_3xui>" }` → `{ login, token, subscription_url }` |
| PUT | `/admin/users/:login/title` | заголовок: `{ "title": "..." }` |
| POST | `/admin/users/:login/announces` | добавить анонс: `{ "body": "..." }` |
| GET | `/admin/users/:login/announces` | список анонсов |
| DELETE | `/admin/users/:login/announces/:id` | удалить анонс |

Swagger UI: http://127.0.0.1:23452/swagger/index.html

### Title / Announce / default

- Title: свой → иначе у пользователя `login=default` → пусто.
- Announce: round-robin своих (`ORDER BY last_shown_at NULLS FIRST`, затем `last_shown_at = now()`) → иначе round-robin `default` → пусто.
- Пользователь `default` сидится миграцией; у него `sub_token = NULL` (подписаться как default нельзя).
- `POST /admin/users` с `login=default` — **отклонён**; title/announces для `default` работают отдельно.

Повторный bind того же логина обновляет `xui_sub_id` из панели и **сохраняет** прежний `sub_token`.

## Переменные окружения

См. `.env.example`:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `PORT` | `23452` | порт HTTP |
| `DATABASE_URL` | postgres://… | Postgres |
| `GIN_MODE` | `debug` | режим Gin |
| `ADMIN_TOKEN` | *(пусто)* | Bearer для `/admin/*`; пустой = deny-all |
| `XUI_BASE_URL` | *(пусто)* | база 3x-ui, напр. `http://3xui_app:2053` |
| `XUI_API_TOKEN` | *(пусто)* | Bearer API-токен панели |
| `PUBLIC_BASE_URL` | `http://127.0.0.1:23452` | база для `subscription_url` |
| `XUI_DOCKER_NETWORK` | `3xui_default` | внешняя docker-сеть 3x-ui |
| `IMAGE` | `vpn_subscription_service:latest` | образ compose |
| `INVITE_GATE_UID` | *(пусто)* | секретный UID QR-входа; пустой = лендинг выключен |
| `INVITE_PAGE_UID` | *(пусто)* | UID countdown/открытки |
| `INVITE_ADMIN_UID` | *(пусто)* | UID админ-таймлайна |
| `INVITE_TZ` | `Europe/Saratov` | timezone reveal |
| `INVITE_REVEAL_AT` | `2026-09-25T20:00:00` | локальное время открытия открытки |
| `INVITE_VIDEO_PATH` | `/data/invite/card.mp4` | путь к mp4 в контейнере |

Если `XUI_*` не заданы, `/health` и `/ready` работают, а bind/sub отвечают **503**.

## Secret invite landing

Опциональный мобильный лендинг-приглашение. Включается, когда заданы все три `INVITE_*_UID`.

- QR → `/{INVITE_GATE_UID}` → 302 на `/{INVITE_PAGE_UID}`
- до `INVITE_REVEAL_AT` (Саратов) — countdown; после — открытка + RSVP
- админка: `/{INVITE_ADMIN_UID}`
- видео: положите `card.mp4` в `deploy/invite-media/` (volume, без пересборки образа)

Боевые UID держите только в `.env` на сервере.

## Docker Compose и сеть с 3x-ui

Сервис `app` подключён к сети `default` (свой Postgres) и к внешней сети `xui_net`.

Узнать сеть контейнера 3x-ui:

```powershell
docker inspect 3xui_app --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{println}}{{end}}'
```

Затем в `.env` / окружении:

```powershell
$env:XUI_DOCKER_NETWORK = "имя_сети_из_inspect"
$env:XUI_BASE_URL = "http://3xui_app:2053"
$env:XUI_API_TOKEN = "токен_из_панели"
$env:ADMIN_TOKEN = "секрет"
```

Запуск:

```powershell
Copy-Item .env.example .env
# отредактируйте .env
Set-Location deploy
docker compose up --build -d
```

Проверка:

```powershell
Invoke-RestMethod http://127.0.0.1:23452/api/v1/health
```

Пример bind:

```powershell
$headers = @{ Authorization = "Bearer $env:ADMIN_TOKEN"; "Content-Type" = "application/json" }
Invoke-RestMethod -Method POST -Uri http://127.0.0.1:23452/api/v1/admin/users `
  -Headers $headers -Body '{"login":"user@example.com"}'
```

## Локальный запуск без Docker

Нужен Postgres и переменные окружения.

```powershell
go mod tidy
go run ./cmd/server
```

Миграции (`migrations/*.sql`) применяются автоматически при старте.

Swagger: http://127.0.0.1:23452/swagger/index.html

## Nginx

Пример проксирования — `deploy/nginx.conf.example`. Production-домен в этом репозитории не задаётся — укажите свой в nginx и `PUBLIC_BASE_URL`.

## CI/CD

Workflow `.github/workflows/ci-cd.yml` при push в `main`:

1. `go build` (**без тестов**)
2. Сборка и push образа в GHCR
3. Деплой по SSH: `docker compose pull && up -d`

### Secrets репозитория

| Secret | Назначение |
|--------|------------|
| `SSH_HOST` | хост сервера |
| `SSH_USER` | пользователь SSH |
| `SSH_PRIVATE_KEY` | приватный ключ |
| `SSH_PORT` | порт SSH (опционально) |
| `DEPLOY_PATH` | каталог с `docker-compose.yml` |

## Модуль

`github.com/chistotel/vpn_subscription_service`
