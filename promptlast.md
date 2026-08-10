# Haze Messenger — Prompt Last

## Выполнено: Этапы 1.1 + 1.2

### 1.1 — Инициализация проекта
Создана полная структура монорепозитория `haze/`:
- **Backend** (Go): 9 микросервисов (auth, chat, media, call, bot, notification, search, premium, gateway)
- **Frontend** (Vue): заглушка с package.json и папками
- **Mobile/Desktop/Shared**: структуры-заглушки
- **docker-compose.yml**: postgres, redis, nats, minio, meilisearch + все сервисы
- **migrations**: `001_init.sql` (users, sessions, user_badges)
- **configs**: YAML конфиги для всех сервисов
- **pkg/**: auth (JWT), ws (WebSocket Hub), middleware, utils
- **models**: User, Session, Chat, Message, Call, File, Reaction, Sticker и др.
- Все 9 сервисов компилируются (`go build ./...`) и проходят `go vet`

### 1.2 — API Gateway
Gateway на `net/http` (Fiber отложен из-за проблем с сетью):
- Reverse proxy по префиксам (`/api/auth/*`, `/api/chat/*` и т.д.)
- In-memory rate limiter (настраивается через `GATEWAY_RATE_LIMIT`, дефолт 100/мин)
- CORS middleware
- Request ID propagation
- Structured logging с duration/status
- Health check: `/health`, `/ready`
- Конфиг через env vars (YAML-файлы как документация)
- Smoke test: `/health` отвечает `{"status":"ok","service":"gateway"}`

### !!! ВАЖНО: TODO — Заменить net/http на Fiber
- Сейчас используем net/http из-за проблем с загрузкой klauspost/compress (транзитив Fiber)
- Перед продакшеном переписать хендлеры: `func(w, r)` → `func(c *fiber.Ctx) error`
- TODO добавлен в `go.mod`

### Используемые технологии
- Go 1.22 (net/http, не Fiber — пока)
- gorilla/websocket, golang-jwt, google/uuid, go.uber.org/zap (в go.sum)

### Следующий шаг
**Этап 1.3: Auth Service** — SSA OAuth2 интеграция, JWT access/refresh, регистрация через SSA, CRUD пользователей