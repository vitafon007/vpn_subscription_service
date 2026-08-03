# VPN Subscription Service

Сервис подписок VPN (v1 scaffold). Домен: `https://chistotel.webtm.ru`, префикс `/babasub`.

В v1 реализованы только health/ready, слойная структура, Docker и CI/CD. **Тестов нет.**

## Архитектура

Строгое разделение слоёв:

| Слой | Путь | Ответственность |
|------|------|-----------------|
| handler | `internal/handler` | HTTP (Gin), без доступа к БД |
| service | `internal/service` | бизнес-логика, без знания Gin |
| dao | `internal/dao` | доступ к данным, без HTTP |
| db | `internal/db` | пул pgx |
| model | `internal/model` | DTO ответов |
| config / server | `internal/config`, `internal/server` | конфиг и сборка роутера |

## API

Базовый путь: `/babasub/api/v1`

- `GET /health` — liveness `{ "status": "ok" }`
- `GET /ready` — readiness (ping Postgres)

Swagger UI: https://chistotel.webtm.ru/babasub/swagger/index.html

## Переменные окружения

См. `.env.example`:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `PORT` | `23452` | порт HTTP |
| `DATABASE_URL` | postgres://… | строка подключения Postgres |
| `GIN_MODE` | `debug` | режим Gin |
| `BASE_PATH` | `/babasub` | префикс маршрутов |
| `IMAGE` | `ghcr.io/chistotel/vpn_subscription_service:latest` | образ для compose |

## Локальный запуск

### Docker Compose

```powershell
Copy-Item .env.example .env
Set-Location deploy
docker compose up --build -d
```

Проверка:

```powershell
Invoke-RestMethod http://127.0.0.1:23452/babasub/api/v1/health
```

### Go без Docker

Нужен запущенный Postgres и `DATABASE_URL`.

```powershell
go mod tidy
go run ./cmd/server
```

Swagger локально (если `GIN_MODE=debug` и схемы в docs с host продакшена):  
http://127.0.0.1:23452/babasub/swagger/index.html

## Nginx

Пример проксирования `/babasub/` — `deploy/nginx-babasub.conf.example`:

```nginx
location /babasub/ {
    proxy_pass http://127.0.0.1:23452/babasub/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## CI/CD

Workflow `.github/workflows/ci-cd.yml` при push в `main`:

1. `go build` (**без тестов**)
2. Сборка и push образа в GHCR: `ghcr.io/<owner>/vpn_subscription_service:<sha>` и `:latest`
3. Деплой по SSH: `docker compose pull && up -d`

### Secrets репозитория

| Secret | Назначение |
|--------|------------|
| `SSH_HOST` | хост сервера |
| `SSH_USER` | пользователь SSH |
| `SSH_PRIVATE_KEY` | приватный ключ |
| `SSH_PORT` | порт SSH (опционально, по умолчанию 22) |
| `DEPLOY_PATH` | каталог с `docker-compose.yml` на сервере |

Для публикации пакетов в workflow задано `permissions.packages: write` (используется `GITHUB_TOKEN`).

## Roadmap

- `GET /babasub/api/v1/sub/:token` — выдача конфигурации подписки по токену
- Миграции схемы БД в `migrations/`
- Генерация VPN-ссылок (не в v1)

## Модуль

`github.com/chistotel/vpn_subscription_service`
