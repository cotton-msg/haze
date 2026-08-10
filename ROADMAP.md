# Haze Messenger — Роудмап разработки

## Структура проекта

```
haze/
├── backend/                  # Go backend
│   ├── cmd/                  # Точки входа сервисов
│   │   ├── auth/
│   │   ├── chat/
│   │   ├── media/
│   │   ├── call/
│   │   ├── bot/
│   │   ├── notification/
│   │   ├── search/
│   │   ├── premium/
│   │   └── gateway/
│   ├── internal/             # Бизнес-логика
│   │   ├── models/
│   │   ├── repository/
│   │   ├── service/
│   │   └── handler/
│   ├── pkg/                  # Общие пакеты
│   │   ├── auth/
│   │   ├── ws/
│   │   ├── middleware/
│   │   └── utils/
│   ├── migrations/           # SQL миграции
│   ├── configs/              # Конфиги
│   ├── docker/               # Dockerfile для каждого сервиса
│   └── go.mod
├── frontend/                 # Vue веб-приложение
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── stores/
│   │   ├── services/
│   │   ├── styles/
│   │   └── utils/
│   └── package.json
├── desktop/                  # Tauri 2 приложение
│   ├── src-tauri/
│   └── ... (ссылка на frontend)
├── mobile/
│   ├── ios/                  # Swift iOS
│   └── android/              # Kotlin Android
├── shared/                   # Общий код (типы, протоколы)
├── docker-compose.yml
└── TECH_DOC.md
```

---

## Этап 1: Инфраструктура и базовый бэкенд

### Цель
Настроить проект, Docker-окружение, базовую архитектуру бэкенда и авторизацию через SSA.

---

### 1.1 Инициализация проекта

**Промпт для ИИ:**
```
Создай структуру проекта для мессенджера "Haze" на Go (бэкенд) и Vue (фронтенд).

Бэкенд:
- Многомодульная архитектура (monorepo) с микросервисами: auth, chat, media, call, bot, notification, search, premium, gateway
- Фреймворк: Fiber для HTTP, Gorilla WebSocket
- БД: PostgreSQL + Redis
- Очередь: NATS
- Медиа-хранилище: MinIO

Создай:
1. Корневую структуру папок
2. go.mod с зависимостями
3. cmd/ для каждого сервиса с минимальным main.go
4. internal/ с пакетами: models, repository, service, handler
5. pkg/ с общими утилитами
6. configs/ с YAML-конфигами для каждого сервиса
7. docker/ с Dockerfile для каждого сервиса
8. docker-compose.yml с сервисами: postgres, redis, nats, minio
9. migrations/ с начальной миграцией для users и sessions

Код должен компилироваться и docker-compose up запускать все сервисы.
```

### 1.2 API Gateway

**Промпт для ИИ:**
```
Реализуй API Gateway для мессенджера на Go (Fiber):

1. Проксирование запросов к микросервисам по префиксам:
   - /api/auth/* -> auth-service
   - /api/chat/* -> chat-service
   - /api/media/* -> media-service
   - /api/call/* -> call-service
   - /api/bot/* -> bot-service
   - /api/notification/* -> notification-service
   - /api/search/* -> search-service
   - /api/premium/* -> premium-service

2. Rate limiting (Redis-based): 100 запросов/мин для обычных, 1000 для premium
3. CORS middleware
4. Request ID пропагация
5. Логирование всех запросов (structured logging через Zap)
6. Health check эндпоинты (/health, /ready)

Gateway должен читать конфиг из configs/gateway.yaml и подключаться к каждому сервису.
Используй http-proxy или кастомный проксинг через Go net/http.
```

### 1.3 Auth Service — SSA интеграция

**Промпт для ИИ:**
```
Реализуй Auth Service для мессенджера на Go:

1. Модели данных:
   - User: id, ssa_id, username, email, phone, avatar_url, created_at, updated_at
   - Session: id, user_id, refresh_token, user_agent, ip, expires_at

2. SSA интеграция (OAuth2/OIDC):
   - POST /auth/ssa/callback — принимает SSA токен, создаёт/находит пользователя
   - GET /auth/ssa/authorize — redirect на SSA authorize URL
   - Обмен code на tokens через SSA token endpoint

3. JWT-авторизация:
   - Access token (15 мин) + Refresh token (30 дней)
   - POST /auth/register — регистрация (через SSA)
   - POST /auth/login — логин (через SSA)
   - POST /auth/refresh — обновление токена
   - POST /auth/logout — выход (удаление сессии)
   - GET /auth/me — текущий пользователь

4. Middleware для проверки JWT в других сервисах

5. PostgreSQL: миграции для users, sessions таблиц
6. Redis: кэширование сессий

Каждый эндпоинт должен иметь валидацию входных данных, обработку ошибок и structured logging.
```

### 1.4 Базовые модели и WebSocket

**Промпт для ИИ:**
```
Создай базовые модели и WebSocket-хаб для мессенджера на Go:

1. Общие модели (internal/models):
   - Message: id, chat_id, sender_id, content, type (text/image/file/voice/video), reply_to, created_at, updated_at, status (sent/delivered/read)
   - Chat: id, type (personal/group/channel), title, avatar, created_at, updated_at
   - ChatMember: chat_id, user_id, role (admin/member), joined_at
   - Contact: user_id, contact_id, created_at

2. WebSocket-хаб (pkg/ws):
   - Hub с управлением соединений по user_id
   - Поддержка множественных соединений на пользователя
   - Броадкаст в чат, отправка конкретному пользователю
   - JSON-протокол: { type: "message"|"typing"|"status"|"read", payload: ... }
   - Ping/Pong heartbeat (30 сек)
   - Реконнект с восстановлением состояния

3. Chat Service:
   - POST /chat/create — создание чата
   - GET /chat/list — список чатов пользователя
   - POST /chat/{id}/message — отправка сообщения (через WS)
   - GET /chat/{id}/messages — история сообщений (пагинация)
   - WS /ws — WebSocket эндпоинт

4. SQL миграции для chats, messages, chat_members таблиц

Код должен быть модульным и тестируемым.
```

---

## Этап 2: Клиент (Web) — Базовый UI

### Цель
Создать Vue-приложение с Liquid Glass дизайном, базовым UI чата и подключением к бэкенду.

---

### 2.1 Инициализация Vue-проекта

**Промпт для ИИ:**
```
Создай Vue-проект для мессенджера "Haze" с современным дизайном Liquid Glass:

1. Инициализируй Nuxt 3 с TypeScript
2. Настрой Tailwind CSS 4 с кастомной темой:
   - Цвета: #6C5CE7 (primary), #00CEC9 (accent), #0D1117 (dark bg)
   - Liquid Glass эффекты: backdrop-blur, semi-transparent фон, glow-эффекты
   - Тени и градиенты для glass-компонентов

3. Создай базовые компоненты:
   - GlassCard — карточка с liquid glass эффектом
   - GlassButton — кнопка с hover-анимацией
   - GlassInput — поле ввода с glow при фокусе
   - GlassModal — модальное окно с blur фоном
   - Avatar — аватар пользователя с онлайн-индикатором

4. Настрой @vueuse/motion и CSS анимации:
   - Плавное появление компонентов
   - Hover-эффекты с масштабированием
   - Анимации переходов между страницами

5. Создай layout:
   - Sidebar (список чатов) — 320px
   - Main area (окно чата) — остаток
   - Header с поиском и профилем

6. Настрой Reactive (Pinia):
   - userStore: текущий пользователь, авторизация
   - chatStore: список чатов, текущий чат
   - messageStore: сообщения текущего чата

Компоненты должны быть переиспользуемыми и стилизованными в едином стиле Liquid Glass.
```

### 2.2 Авторизация в UI

**Промпт для ИИ:**
```
Реализуй экраны авторизации для мессенджера Haze на Vue:

1. Страница логина (/login):
   - Glass-форма по центру экрана
   - Кнопка "Войти через SSA" (redirect на SSA authorize)
   - Анимированный фон с gradient animation
   - Логотип Haze с glow-эффектом

2. Страница регистрации (/register):
   - Форма: имя, username, email
   - Автоматическое создание через SSA
   - Валидация полей в реальном времени

3. Обработка callback от SSA (/auth/callback):
   - Извлечение code из URL
   - Обмен на JWT-токены
   - Сохранение в Zustand store + localStorage
   - Редирект на главную страницу

4. Auth middleware/hook:
   - Проверка токена при загрузке
   - Автоматический refresh при истечении
   - Редирект на /login если не авторизован

5. Страница настроек профиля (/settings/profile):
   - Редактирование аватара (с предпросмотром)
   - Имя, username, био
   - Сохранение через API

Все формы должны использовать Glass-компоненты и Framer Motion анимации.
```

### 2.3 Список чатов и окно сообщений

**Промпт для ИИ:**
```
Реализуй основной UI мессенджера Haze на Vue:

1. Sidebar — список чатов:
   - Glass-карточки чатов с hover-эффектами
   - Аватар чата/контакта
   - Последнее сообщение (превью, обрезка текста)
   - Время последнего сообщения
   - Индикатор непрочитанных (badge с glow)
   - Строка поиска сверху с glass-стилем
   - Кнопка "Новый чат" (Floating Action Button)

2. Main area — окно чата:
   - Header: аватар контакта, имя, статус онлайн
   - Область сообщений с автоматическим скроллом
   - Bubble-сообщения:
     - Входящие: слева, серый фон
     - Исходящие: справа, primary цвет
     - Время прочтения (✓ ✓)
     - Контекстное меню (ответить, скопировать, удалить, переслать)
   - Поле ввода:
     - GlassInput с иконками (прикрепить файл, эмодзи, голос)
     - Кнопка отправки с анимацией
     - Индикатор "печатает..."

3. WebSocket подключение:
   - Хук useWebSocket для подключения к бэкенду
   - Автоматический реконнект
   - Обработка входящих сообщений в реальном времени
   - Отправка typing-статуса

4. Пагинация сообщений:
   - Бесконечный скролл вверх для загрузки истории
   - Спиннер загрузки

Все компоненты в Liquid Glass стиле с анимациями.
```

---

## Этап 3: Продвинутый функционал чата

### Цель
Добавить файлы, эмодзи, реакции, стикеры, кружки, голосовые сообщения.

---

### 3.1 Отправка файлов и изображений

**Промпт для ИИ:**
```
Реализуй отправку файлов и изображений в мессенджере Haze:

1. Backend (Media Service):
   - POST /media/upload — загрузка файла (multipart/form-data)
   - GET /media/{id} — получение файла
   - GET /media/{id}/thumbnail — миниатюра для изображений
   - Поддержка типов: image/*, video/*, audio/*, application/*
   - Лимиты: 50MB для обычных, 500MB для premium
   - Хранение в MinIO

2. Backend (Chat Service):
   - Тип сообщения: file, image, video
   - Модель File: id, message_id, url, mime_type, size, name, created_at

3. Frontend:
   - Кнопка прикрепления файла в поле ввода (иконка скрепки)
   - Drag & Drop зона на окно чата
   - Предпросмотр изображений перед отправкой (modal)
   - Прогресс-бар загрузки
   - Отображение файлов в чате:
     - Изображения: превью с возможностью открыть в полном размере
     - Файлы: иконка типа + имя + размер + кнопка скачивания
   - Галерея изображений чата (просмотр всех фото)

4. Превью ссылок ( unfurling ):
   - Парсинг URL в сообщениях
   - Загрузка Open Graph данных (title, description, image)
   - Рендер превью карточки ссылки

Всё в Liquid Glass стиле.
```

### 3.2 Эмодзи, стикеры и реакции

**Промпт для ИИ:**
```
Реализуй систему эмодзи, стикеров и реакций для мессенджера Haze:

1. Backend:
   - Таблица stickers: id, name, image_url, pack_id, is_premium
   - Таблица sticker_packs: id, name, is_premium, thumbnail_url
   - Таблица reactions: id, message_id, user_id, emoji, created_at
   - POST /stickers/packs — список пакетов
   - POST /stickers/{pack_id} — стикеры пакета
   - POST /message/{id}/reaction — добавить/убрать реакцию
   - DELETE /message/{id}/reaction/{emoji} — убрать реакцию

2. Frontend — Emoji Picker:
   - Glass-модальное окно с табами: Smileys, Animals, Food, Activities, Travel, Objects, Symbols, Flags
   - Поиск по эмодзи
   - Часто используемые эмодзи
   - Кастомные эмодзи (Haze-тематика)
   - Анимация выбора эмодзи

3. Frontend — Sticker Picker:
   - Пакеты стикеров с превью
   - Анимированные стикеры (Lottie)
   - Недавние стикеры

4. Frontend — Реакции:
   - Долгое нажатие на сообщение → меню с реакциями
   - Быстрые реакции: 👍 ❤️ 😂 😮 😢 🔥
   - Отображение реакций под сообщением (счётчик + аватары)
   - Анимация появления реакции

5. Emoji-рендеринг:
   - Native emoji или кастомные спрайты
   - Поддержка肤色 модификаторов

В Liquid Glass стиле с анимациями.
```

### 3.3 Голосовые и видеосообщения (кружки)

**Промпт для ИИ:**
```
Реализуй голосовые сообщения и видео-кружки для мессенджера Haze:

1. Backend (Media Service):
   - POST /media/voice — загрузка голосового сообщения (ogg/mp3)
   - POST /media/video-circle — загрузка видео-кружка (mp4, max 60 сек)
   - Конвертация:ffmpeg для нормализации форматов
   - Генерация waveform для голосовых

2. Frontend — Запись голоса:
   - Кнопка микрофона (длинное нажатие для записи)
   - Визуализация звука (waveform) в реальном времени
   - Предпрослушивание перед отправкой
   - Отмена свайпом влево
   - Автоматическое определение длительности

3. Frontend — Запись видео-кружка:
   - Кнопка камеры (длинное нажатие)
   - Круглый превью с обратным отсчётом (3, 2, 1...)
   - Запись max 60 секунд
   - Круглый плеер в чате (автопри воспроизведении)
   - Превью перед отправкой

4. Frontend — Воспроизведение:
   - Голосовые: inline-плеер с play/pause, waveform, скоростью (1x, 1.5x, 2x)
   - Видео: круглый контейнер, тап для полного экрана
   - Прогресс-бар в Liquid Glass стиле

5. Хранение:
   - Waveform данные в JSON (привязаны к сообщению)
   - Превью-кадр для видео

В Liquid Glass стиле с плавными анимациями.
```

---

## Этап 4: Звонки и видеозвонки

### Цель
Реализовать аудио/видео звонки и демонстрацию экрана через WebRTC.

---

### 4.1 WebRTC Call Service

**Промпт для ИИ:**
```
Реализуй WebRTC Call Service для мессенджера Haze на Go:

1. Архитектура:
   - Pion WebRTC для Go
   - Signaling через WebSocket (тот же хаб что и чат)
   - STUN/TURN серверы (cotourn в Docker)

2. Модели:
   - Call: id, caller_id, callee_id, type (audio/video), status (ringing/active/ended/missed), started_at, ended_at
   - CallParticipant: call_id, user_id, joined_at

3. Signaling:
   - offer → answer → ice-candidate обмен
   - WS сообщения: call_offer, call_answer, call_ice, call_end, call_reject

4. API:
   - POST /call/start — начало звонка (создание Call + отправка offer через WS)
   - POST /call/answer — принятие звонка
   - POST /call/reject — отклонение звонка
   - POST /call/end — завершение звонка
   - GET /call/history — история звонков

5. Групповые звонки (SFU):
   - Selective Forwarding Unit через Pion
   - Поддержка до 8 участников
   - Сетевая топология: mesh для 2, SFU для 3+

6. TURN сервер:
   - Docker-compose: coturn с auth
   - TLS сертификаты

7. Docker:
   -coturn Dockerfile
   - Настройки в docker-compose

Код должен быть тестируемым с unit-тестами.
```

### 4.2 Клиент звонков

**Промпт для ИИ:**
```
Реализуй клиентскую часть звонков для мессенджера Haze на Vue:

1. UI вызова:
   - Исходящий вызов: полноэкранный glass-экран с аватаром вызываемого, анимация пульсации
   - Входящий вызов: модальное окно с accept/reject кнопками, звук звонка
   - Во время звонка: таймер, кнопки (mute, camera, end call, speaker, screen share)

2. Video Grid:
   - 1-1: Полноэкранное видео собеседника
   - Группа: сетка 2x2, 3x3 с auto-layout
   - Speaking indicator (подсветка активного говорящего)
   - Pin participant (закрепить участника)

3. Controls:
   - Mute microphone (иконка с анимацией)
   - Включить/выключить камеру
   - Демонстрация экрана (выбор окна/вкладки)
   - Переключение камеры (фронтальная/задняя на мобильном)
   - Кнопка завершения (красная, с confirm)

4. Screen Sharing:
   - getDisplayMedia API
   - Picture-in-Picture для своего видео
   - Панель с участниками сбоку

5. Интеграция с чатом:
   - Кнопка вызова в header чата
   - История звонков в списке чатов
   - Missed call уведомление

6. WebRTC hooks:
   - useWebRTC — управление peer connection
   - useMediaStream — захват камеры/микрофона
   - useScreenShare — демонстрация экрана

В Liquid Glass стиле с анимациями.
```

---

## Этап 5: Групповые чаты и продвинутые функции

### Цель
Добавить групповые чаты, каналы, темы, папки, закреплённые сообщения.

---

### 5.1 Групповые чаты

**Промпт для ИИ:**
```
Реализуй групповые чаты для мессенджера Haze:

1. Backend:
   - POST /chat/group/create — создание группы (название, описание, аватар, участники)
   - POST /chat/group/{id}/members — добавление участников
   - DELETE /chat/group/{id}/members/{user_id} — удаление участника
   - PUT /chat/group/{id} — редактирование группы (название, описание, аватар)
   - PUT /chat/group/{id}/role — изменение роли участника
   - DELETE /chat/group/{id}/leave — выход из группы
   - POST /chat/group/{id}/pin — закрепление сообщения

2. Модели:
   - Chat.type: personal | group | channel
   - ChatMember.role: owner | admin | member
   - Chat: title, description, avatar, is_pinned, pinned_message_id

3. Frontend:
   - Создание группы: Glass-модалка с выбором участников, вводом названия
   - Настройки группы: header → настройки (для admin/owner)
   - Список участников с ролями
   - Добавление участников: поиск по username, выбор из контактов
   - Уведомления: "User joined", "User left", "Group created"

4. Особенности:
   - История для каждого нового участника (настраиваемая)
   - Тихий режим для участников
   - Ограничение записи (только admin может писать)

В Liquid Glass стиле.
```

### 5.2 Каналы и темы

**Промпт для ИИ:**
```
Реализуй каналы и темы для мессенджера Haze:

1. Backend:
   - Тип чата: channel (один ко многим, подписчики)
   - Таблица topics: id, chat_id, title, created_at, is_pinned
   - POST /chat/channel/create — создание канала
   - POST /chat/channel/{id}/subscribe — подписка
   - DELETE /chat/channel/{id}/unsubscribe — отписка
   - GET /chat/channel/{id}/topics — список тем
   - POST /chat/channel/{id}/topic — создание темы
   - POST /chat/topic/{id}/message — сообщение в тему

2. Frontend:
   - Каналы в списке чатов с иконкой📢
   - Подписка/отписка
   - Темы внутри канала (табы или список)
   - Ограничение записи: только admin может писать, другие комментируют в темах

3. Модель:
   - Channel: title, description, avatar, subscriber_count, is_public
   - Topic: title, message_count, last_message_at

В Liquid Glass стиле.
```

### 5.3 Папки чатов

**Промпт для ИИ:**
```
Реализуй папки (категории) для чатов мессенджера Haze:

1. Backend:
   - Таблица chat_folders: id, user_id, name, icon, emoji, position
   - Таблица chat_folder_chats: folder_id, chat_id
   - POST /folders — CRUD папок
   - PUT /folders/{id}/chats — добавление/удаление чатов из папки
   - GET /folders — список папок с чатами

2. Frontend:
   - Создание папок: Glass-модалка с названием, иконкой, эмодзи
   - Табы папок в sidebar (Все, Личные, Группы, Каналы, Избранное + кастомные)
   - Drag & Drop чатов между папками
   - Дефолтные папки: Все, Непрочитанные, Личные, Группы, Каналы
   - Настройки папок: редактирование, удаление, переупорядочивание

3. UI:
   - Табы с glass-стилем и анимацией переключения
   - Счётчик непрочитанных по папкам

В Liquid Glass стиле.
```

### 5.4 Дополнительно

**Промпт для ИИ:**
```
Реализуй дополнительные функции для мессенджера Haze:

1. Редактирование сообщений:
   - Long press → "Редактировать"
   - Индикатор "отредактировано"
   - Backend: PUT /message/{id}

2. Удаление сообщений:
   - Long press → "Удалить"
   - Удаление для всех / только для себя
   - Замена текста на "Сообщение удалено"
   - Backend: DELETE /message/{id}

3. Цитирование:
   - Long press → "Ответить"
   - Preview цитаты в поле ввода
   - Клик по цитате → скролл к оригиналу
   - Backend: reply_to в Message

4. Пересылка:
   - Long press → "Переслать"
   - Выбор чата для пересылки
   - Отметка "Переслано от @user"

5. Поиск:
   - Поиск по сообщениям в чате
   - Глобальный поиск (по всем чатам)
   - Фильтры: по дате, по типу (фото, видео, файлы)
   - Backend: MEILISEARCH интеграция

6. Закреплённые сообщения:
   - Pin/Unpin сообщение
   - Закреплённое сообщение в header чата
   - Список закреплённых в настройках чата

В Liquid Glass стиле.
```

---

## Этап 6: Кастомизация и настройки

### Цель
Полная настройка тем, обоев, splash-экранов, звуков, уведомлений.

---

### 6.1 Темы и оформление

**Промпт для ИИ:**
```
Реализуй систему тем и кастомизации для мессенджера Haze:

1. Backend:
   - Таблица themes: id, name, author_id, is_premium, colors (JSON), created_at
   - Таблица user_settings: user_id, theme_id, font_size, wallpaper_id, notification_sounds (JSON)
   - Таблица wallpapers: id, url, is_premium, preview_url
   - CRUD для тем и настроек

2. CSS-переменные темы:
   --bg-primary, --bg-secondary, --bg-tertiary
   --text-primary, --text-secondary
   --accent-primary, --accent-secondary
   --glass-bg, --glass-border, --glass-blur
   --shadow-color, --glow-color

3. Frontend — Настройки темы:
   - Светлая / Тёмная / Системная
   - Выбор акцентного цвета (color picker)
   - Предпросмотр темы в реальном времени
   - Glass-эффекты: интенсивность blur, opacity, border

4. Дефолтные темы:
   - Haze Dark (основная, тёмная)
   - Haze Light (светлая)
   - Haze Purple (фиолетовый акцент)
   - Haze Ocean (синий акцент)

5. Кастомные темы:
   - Создание темы с нуля
   - Импорт/экспорт тем (JSON)
   - Публикация в сообществе (для premium)

В Liquid Glass стиле.
```

### 6.2 Обои и фоны

**Промпт для ИИ:**
```
Реализуй систему обоев и фонов для мессенджера Haze:

1. Backend:
   - Таблица wallpapers: id, name, url, preview_url, is_premium, category
   - GET /wallpapers — каталог обоев
   - POST /wallpapers/upload — загрузка пользовательских обоев (premium)
   - PUT /settings/wallpaper — установка обоев (глобально или для чата)

2. Frontend — Выбор обоев:
   - Каталог обоев в настройках
   - Категории: Градиенты, Абстракция, Природа, Кастомные
   - Предпросмотр с blur/opacity настройками
   - Применение к конкретному чату или глобально

3. Frontend — Редактор обоев:
   - Выбор из галереи
   - Загрузка своего изображения
   - Настройки: blur, opacity, brightness, saturation
   - Цветовые фильтры

4. Фон чата:
   - Wallpaper позади сообщений
   - Glass-эффект для сообщений поверх обоев
   - Плавное скроллирование фона

В Liquid Glass стиле.
```

### 6.3 Splash-экран и звуки

**Промпт для ИИ:**
```
Реализуй настройку splash-экрана и звуков для мессенджера Haze:

1. Splash Screen:
   - Кастомные splash-экраны (premium)
   - Анимированный логотип Haze
   - Плавный transition в основное приложение
   - Настройки: длительность, анимация

2. Звуки уведомлений:
   - Каталог звуков: Default, Bubble, Chime, Digital, Gentle, Sharp
   - Предпрослушивание
   - Назначение звука по типу события:
     - Новое сообщение
     - Входящий звонок
     - Новый контакт
     - Реакция
   - Пользовательские звуки (загрузка)
   - Отключение звука по событиям

3. Настройки звука:
   - Общая громкость
   - Звук при отправке сообщения (вкл/выкл)
   - Звук при наборе текста (вкл/выкл)
   - Вибрация (мобильные)

4. Frontend:
   - Glass-менеджер звуков
   - Waveform preview для звуков
   - Настройки в профиле пользователя

В Liquid Glass стиле.
```

### 6.4 Уведомления

**Промпт для ИИ:**
```
Реализуй систему уведомлений для мессенджера Haze:

1. Backend (Notification Service):
   - Таблица notification_settings: user_id, event_type, enabled, sound_id
   - Таблица notification_devices: user_id, token, platform, last_active
   - POST /notifications/register — регистрация устройства
   - PUT /notifications/settings — настройки уведомлений
   - POST /notifications/send — отправка push (FCM/APNs/Web Push)

2. Типы уведомлений:
   - Новое сообщение (вкл/выкл, звук)
   - Входящий звонок (всегда вкл)
   - Новый контакт
   - Добавлен в группу
   - Реакция на сообщение
   - Упоминание (@username)
   - Новое сообщение в канале
   - Бот-уведомления

3. Frontend:
   - In-app уведомления (toast/toast-подобные)
   - Glass-стиль уведомлений
   - Настройки по каждому типу
   - Тихий режим (по расписанию)
   - Не беспокоить (DND)

4. Push уведомления:
   - Web Push API для браузера
   - FCM для Android
   - APNs для iOS
   - Объединение уведомлений (grouping)

В Liquid Glass стиле.
```

---

## Этап 7: Премиум и боты

### Цель
Премиум-подписка, Bot API, голосовые чаты.

---

### 7.1 Премиум-система

**Промпт для ИИ:**
```
Реализуй Premium-систему для мессенджера Haze:

1. Backend:
   - Таблица premium_plans: id, name, price, duration, features (JSON)
   - Таблица premium_subscriptions: user_id, plan_id, starts_at, ends_at, auto_renew
   - Таблица premium_payments: id, user_id, plan_id, amount, status, created_at
   - POST /premium/plans — список тарифов
   - POST /premium/subscribe — оформление подписки
   - POST /premium/cancel — отмена подписки
   - GET /premium/status — статус подписки

2. Premium возможности:
   - Расширенные лимиты (500MB файлы вместо 50MB)
   - Эксклюзивные эмодзи/стикеры
   - Приоритетная поддержка
   - Создание кастомных тем
   - Загрузка пользовательских обоев
   - Расширенные настройки
   - Имя пользователя с бейджем Premium
   - Голосовые чаты (до 50 человек вместо 8)

3. Frontend:
   - Premium-страница с описанием преимуществ
   - Glass-карточки тарифов
   - Оплата через Stripe/ЮKassa
   - Premium-бейдж в профиле
   - Эксклюзивный контент помечен иконкой⭐

4. Интеграция:
   - Проверка premium-статуса в API
   - Лимиты на основе подписки
   - Premium-стикерпаки

В Liquid Glass стиле.
```

### 7.2 Bot API

**Промпт для ИИ:**
```
Реализуй Bot API для мессенджера Haze:

1. Backend (Bot Service):
   - Таблица bots: id, owner_id, token, username, name, description, avatar, is_premium
   - Таблица bot_commands: bot_id, command, description
   - POST /bot/create — создание бота
   - POST /bot/{token}/setWebhook — установка вебхука
   - DELETE /bot/{token}/deleteWebhook — удаление вебхука
   - GET /bot/{token}/getUpdates — polling (альтернатива webhook)
   - POST /bot/{token}/sendMessage — отправка сообщения
   - POST /bot/{token}/editMessage — редактирование
   - DELETE /bot/{token}/deleteMessage — удаление

2. Bot API методы:
   - sendMessage, editMessage, deleteMessage
   - sendPhoto, sendVideo, sendDocument, sendVoice
   - sendSticker, sendAnimation
   - answerCallbackQuery
   - setMyCommands, getMyCommands
   - getUserProfilePhotos
   - kickChatMember, restrictChatMember

3. Webhook:
   - HTTPS endpoint для получения обновлений
   - Верификация через secret token
   - Retry机制 с exponential backoff

4. Inline Mode:
   - Inline query от пользователя
   - Результаты от бота
   - Выбор результата → вставка в чат

5. Команды:
   - /start, /help, /settings — дефолтные
   - Кастомные команды бота
   - Меню команд в UI

6. Frontend:
   - Профиль бота (как пользователя)
   - Кнопка "Начать диалог с ботом"
   - Меню команд
   - Отметка "бот" в профиле

В Liquid Glass стиле.
```

### 7.3 Голосовые чаты

**Промпт для ИИ:**
```
Реализуй голосовые чаты для мессенджера Haze:

1. Backend:
   - Таблица voice_chats: id, channel_id, title, started_at, ended_at
   - Таблица voice_chat_participants: chat_id, user_id, role (speaker/listener), joined_at
   - WebRTC SFU для группового аудио
   - До 50 участников (premium: до 100)

2. Frontend:
   - Кнопка "Начать голосовой чат" в канале/группе
   - UI голосового чата:
     - Список участников с аватарами
     - Speaking indicator (подсветка говорящего)
     - Кнопки: выйти, вкл/выкл микрофон, поднять руку
     - Роли: speaker (говорит), listener (слушает)
   - Поднятие руки → запрос на слово
   - Таймер длительности

3. Интеграция:
   - Запись голосового чата (для premium)
   - Транскрибирование (AI, premium)
   - Расписание голосовых чатов

В Liquid Glass стиле.
```

---

## Этап 8: Мобильные приложения

### Цель
iOS и Android приложения с нативным UX.

### Альтернативный подход: Telegram-клиенты как основа (рекомендуется)

Для ускорения MVP **форкни полностью** open-source клиенты Telegram и адаптируй под Haze.
**Дизайн Telegram оставляем как есть** — он уже идеален. Меняем только:
- Логотип → Haze
- Цвета на кастомные (Liquid Glass палитра)
- Название приложения
- Брендинг (splash screen, иконки)

Всё остальное (UI, UX, анимации, жесты) — **оставляем Telegram**.

**Telegram iOS** (`github.com/TelegramMessenger/Telegram-iOS`):
- Swift, UIKit — готовый UI чатов, звонков, медиа, стикеров
- **Оставляем дизайн**, заменяем только API слой + брендинг

**Telegram Android** (`github.com/nicegram/nicegram-android` / форки):
- Kotlin — оптимизирован под миллионы пользователей
- **Оставляем дизайн**, заменяем только API слой + брендинг

**Что делаем:**
1. Форкаем полностью
2. Заменяем MTProto → REST/WebSocket (Haze API)
3. Заменяем Telegram Auth → SSA OAuth2
4. Меняем логотип, название, цвета
5. Добавляем Liquid Glass акценты (где уместно, без изменения структуры UI)
6. Добавляем кастомные настройки Haze

**Этап 8A: Адаптация Telegram-клиента (ускоренный путь)**

**Промпт для ИИ:**
```
Адаптируй open-source клиент Telegram iOS под мессенджер Haze:

1. Форкни репозиторий Telegram-iOS
2. Создай слой абстракции для API:
   - Замени MTProto вызовы на REST/WebSocket (Haze API)
   - Создай HazeAPIProtocol с методами: sendMessage, getChats, getMessages, etc.
   - Адаптируй модели данных под Haze (Message, Chat, User)

3. Авторизация:
   - Замени Telegram Phone авторизацию на SSA OAuth2
   - Реализуй HazeAuthManager: login via SSA, JWT tokens, refresh

4. Дизайн (Liquid Glass):
   - Замени Telegram темы на кастомные
   - Добавь backdrop-blur, semi-transparent компоненты
   - Кастомные bubble-сообщения с glass-эффектом
   - Обнови splash screen и launch screen

5. Конфигурация:
   - HazeConfig.plist: API endpoint, SSA client ID
   - Все настройки перенеси в Haze Settings Bundle

6. Тестирование:
   - Unit тесты для нового API слоя
   - UI тесты для кастомных экранов
   - E2E тесты авторизации через SSA

Цель: рабочее приложение с базовым функционалом (чаты, сообщения, файлы) за 2-3 недели вместо 4-6.
```

**Этап 8B: Адаптация Telegram Android**

**Промпт для ИИ:**
```
Адаптируй open-source клиент Telegram Android под мессенджер Haze:

1. Форкни репозиторий (nicegram или форк Telegram Android)
2. API слой:
   - Замени TDLib/TGNet на Retrofit + WebSocket (Haze API)
   - Создай HazeApiService interface сuspend функциями
   - Адаптируй модели: HazeMessage, HazeChat, HazeUser

3. Авторизация:
   - Замени Phone авторизацию на SSA OAuth2
   - HazeAuthRepository: login, logout, refreshToken
   - Hilt DI для Auth модуля

4. Дизайн (Liquid Glass + Hazerial 3):
   - Кастомные темы через HazerialTheme
   - Glass-компоненты: Surface с blur, semi-transparent Card
   - Анимации через Motion API
   - Кастомные bubble-сообщения

5. Хранение:
   - Room: кэш сообщений и чатов
   - EncryptedSharedPreferences: токены
   - DataStore: настройки

6. Push:
   - Firebase Cloud Messaging
   - HazeNotificationService для обработки

Цель: рабочее приложение за 2-3 недели вместо 4-6.
```

---

### 8.1 iOS Приложение (с нуля)

**Промпт для ИИ:**
```
Создай iOS-приложение мессенджера Haze на Swift/SwiftUI:

1. Архитектура:
   - Clean Architecture (Domain, Data, Presentation)
   - MVVM + Combine
   - Dependency Injection

2. Структура:
   - HazeApp (Entry point)
   - Scenes: Auth, Main, Settings
   - Features: Chat, Calls, Profile, Bots
   - Core: Network, Storage, WebSocket

3. UI (SwiftUI):
   - Liquid Glass дизайн (backdrop-blur, gradients)
   - Список чатов (List с swipe actions)
   - Окно чата (ScrollView с messages)
   - Звонки ( full-screen call UI )
   - Настройки (Form с navigation)

4. Сеть:
   - URLSession для REST API
   - Starscream для WebSocket
   - Codable для JSON

5. Хранение:
   - CoreData для кэша сообщений
   - Keychain для токенов
   - UserDefaults для настроек

6. Фичи:
   - Push уведомления (APNs)
   - Биометрия (FaceID/TouchID)
   - Picture-in-Picture для звонков
   - Share Extension для отправки файлов

Код должен следовать Apple Human Interface Guidelines.
```

### 8.2 Android Приложение

**Промпт для ИИ:**
```
Создай Android-приложение мессенджера Haze на Kotlin/Jetpack Compose:

1. Архитектура:
   - Clean Architecture (Domain, Data, Presentation)
   - MVVM + StateFlow
   - Hilt для DI

2. Структура:
   - HazeApplication
   - Features: auth, chat, calls, profile, settings, bots
   - Core: network, database, websocket, di

3. UI (Jetpack Compose):
   - Liquid Glass дизайн (Hazerial 3 + кастомные компоненты)
   - Список чатов (LazyColumn с swipe)
   - Окно чата (LazyColumn с messages)
   - Звонки (full-screen call)
   - Настройки (Settings composables)

4. Сеть:
   - Retrofit + OkHttp для REST
   - OkHttp WebSocket
   - Kotlinx Serialization

5. Хранение:
   - Room для кэша
   - EncryptedSharedPreferences для токенов
   - DataStore для настроек

6. Фичи:
   - Firebase Cloud Messaging
   - BiometricPrompt
   - Picture-in-Picture
   - Share Sheet

Код должен следовать Hazerial Design 3 гайдлайнам.
```

---

## Этап 9: Tauri Desktop

### Цель
Десктопное приложение через Tauri 2.

---

### 9.1 Tauri 2 Приложение

**Промпт для ИИ:**
```
Создай десктопное приложение мессенджера Haze на Tauri 2:

1. Архитектура:
   - Tauri 2.0 (Rust backend)
   - Vue фронтенд (из папки frontend/)
   - IPC через Tauri Commands

2. Rust Backend (src-tauri/):
   - System tray иконка
   - Desktop notifications
   - Auto-updater
   - Window management (minimize to tray)
   - Deep linking (haze://)
   - Clipboard access
   - File system access

3. Tauri Commands:
   - get_platform_info
   - show_notification
   - set_tray_icon
   - open_external_url
   - read/write_file
   - get_system_info

4. Frontend интеграция:
   - Vue в WebView
   - Tauri API для нативных фич
   - Кастомные заголовки окна
   - Безопасный IPC

5. Конфигурация:
   - tauri.conf.json: window size, title, icon
   - Permissions для каждой команды
   - CSP (Content Security Policy)

6. Сборка:
   - CI/CD для Windows/macOS/Linux
   - Code signing
   - Auto-update через Tauri updater

Код должен компилироваться и работать на всех платформах.
```

---

## Этап 10: Деплой и тестирование

### Цель
Настроить CI/CD, тесты, мониторинг, деплой.

---

### 10.1 Тестирование

**Промпт для ИИ:**
```
Реализуй тестирование для мессенджера Haze:

1. Backend тесты:
   - Unit-тесты для сервисов (httptest + testify)
   - Integration тесты для API
   - Тесты WebSocket соединений
   - Тесты БД (migrations + queries)
   - Mock'и для внешних сервисов

2. Frontend тесты:
   - Unit-тесты компонентов (Vitest + Vue Testing Library)
   - E2E тесты (Playwright)
   - Visual regression тесты
   - Тесты WebSocket hook'ов

3. Mobile тесты:
   - Unit-тесты (XCTest для iOS, JUnit для Android)
   - UI тесты (XCUITest, Espresso)
   - Snapshot тесты

4. CI/CD:
   - GitHub Actions workflow
   - Запуск тестов при PR
   - Linting (golangci-lint, ESLint, SwiftLint, ktlint)
   - Code coverage отчётность
   - Автоматическая сборка при мерже в main

5. Docker:
   - Тестирование в Docker
   - Smoke тесты после деплоя

Создай примеры тестов для каждого типа.
```

### 10.2 Деплой

**Промпт для ИИ:**
```
Настрой деплой мессенджера Haze:

1. Docker Compose (для разработки):
   - Все сервисы с volumes и networks
   - Env файлы для конфигурации
   - Health checks для каждого сервиса
   - Логирование в Loki

2. Kubernetes (для продакшена):
   - Helm чарты для каждого сервиса
   - Deployment, Service, Ingress
   - ConfigMap и Secret
   - HPA (автоскейлинг)
   - PersistentVolume для БД и медиа

3. CI/CD Pipeline:
   - Build → Test → Deploy (staging) → Deploy (production)
   - Blue/Green деплой
   - Rollback стратегия
   - Secret management (Vault)

4. Мониторинг:
   - Prometheus: метрики всех сервисов
   - Grafana: дашборды (QPS, latency, errors, saturation)
   - Loki: логи
   - Jaeger: трейсы
   - Алерты: PagerDuty/Telegram

5. Security:
   - Rate limiting
   - WAF правила
   - SSL/TLS everywhere
   - Secret rotation
   - Audit logging

6. Scripts:
   - deploy.sh — деплой на сервер
   - backup.sh — бэкап БД
   - monitor.sh — проверка здоровья

Настрой все в Docker/Kubernetes форматах.
```

---

## Этап 11: Юзернеймы, бейджи и админ-панель

### Цель
Реализовать систему @user юзернеймов, бейджи для ролей и админ-панель владельца.

---

### 11.1 Юзернеймы @user

**Промпт для ИИ:**
```
Реализуй систему юзернеймов для мессенджера Haze:

1. Backend:
   - Таблица users: добавить поле username (unique, indexed)
   - Валидация: латиница, цифры, подчёркивание, 3-32 символа
   - Проверка уникальности: GET /auth/username/check/{username}
   - Смена юзернейма: PUT /auth/username (1 раз в 30 дней)
   - Поиск по юзернейму: GET /users/search?q=@username

2. Ссылки:
   - Веб: https://haze.app/u/username
   - Deeplink: haze://u/username
   - Рендеринг: кликабельные @username в сообщениях и профилях

3. Frontend:
   - Юзернейм в профиле пользователя (копируется по клику)
   - @username в header чата
   - Поиск по юзернейму в модалке "Новый чат"
   - Валидация в реальном времени при регистрации/смене
   - Отображение в списке участников группы

4. API:
   - POST /auth/username — установить/сменить
   - GET /users/{username} — профиль по юзернейму
   - GET /users/search?q= — поиск

В Liquid Glass стиле.
```

### 11.2 Бейджи пользователей

**Промпт для ИИ:**
```
Реализуй систему бейджей для мессенджера Haze:

1. Backend:
   - Таблица user_badges: id, user_id, badge_type, assigned_by, assigned_at
   - Типы бейджей: owner, developer, premium, bot, admin, verified
   - API:
     - GET /user/{id}/badges — получить бейджи
     - POST /user/{id}/badge — назначить (только owner/admin)
     - DELETE /user/{id}/badge/{type} — снять

2. Модель:
   - BadgeType enum: Owner, Developer, Premium, Bot, Admin, Verified
   - Владелец: только 1 аккаунт (создаётся при деплое)
   - Developer: назначается owner
   - Premium: автоматически при подписке
   - Bot: автоматически при создании бота

3. Frontend:
   - Иконка бейджа рядом с именем пользователя
   - Тултип при наведении (расшифровка)
   - Отображение в: профиле, чате, списке участников, поиске
   - Приоритет: owner > developer > admin > premium > verified > bot

4. Управление:
   - Админка: назначение/снятие бейджей
   - Лог: кто назначил, когда

В Liquid Glass стиле.
```

### 11.3 Админ-панель

**Промпт для ИИ:**
```
Реализуй админ-панель для мессенджера Haze:

1. Backend (Admin Service):
   - Middleware проверки роли: owner/developer
   - API:
     - GET /admin/dashboard — статистика (юзеры, чаты, сообщения, звонки)
     - GET /admin/users — список с фильтрами/поиском
     - PUT /admin/user/{id}/role — изменение роли
     - POST /admin/user/{id}/ban — бан пользователя
     - GET /admin/chats — список чатов
     - GET /admin/bots — список ботов
     - GET /admin/premium — статистика premium
     - GET /admin/logs — аудит-логи

2. Frontend (Vue + Nuxt):
   - Маршрут /admin (защищён проверкой роли)
   - Layout: sidebar навигация + контент
   - Разделы:
     a. Дашборд: графики, метрики, KPI
     b. Пользователи: таблица, поиск, действия
     c. Чаты: модерация, просмотр
     d. Боты: список, блокировка
     e. Premium: планы, подписки, платежи
     f. Бейджи: управление назначениями
     g. Настройки: лимиты, регистрация, maintenance
     h. Логи: аудит, экспорт

3. UI:
   - Glass-таблицы с сортировкой/фильтрацией
   - Модальные окна для действий
   - Confirm-dialog для опасных действий
   - Экспорт в CSV/JSON

4. Безопасность:
   - Доступ: только owner + developer
   - Двойная аутентификация для опасных действий
   - Логирование всех действий админа
   - Rate limiting на API

В Liquid Glass стиле.
```

---

## Приоритеты и временные рамки

### Вариант A: Разработка с нуля

| Этап | Описание | Ориентировочное время |
|------|----------|----------------------|
| 1 | Инфраструктура и базовый бэкенд | 2-3 недели |
| 2 | Клиент (Web) — Базовый UI | 2-3 недели |
| 3 | Продвинутый функционал чата | 2-3 недели |
| 4 | Звонки и видеозвонки | 2-3 недели |
| 5 | Групповые чаты и продвинутые функции | 2-3 недели |
| 6 | Кастомизация и настройки | 2-3 недели |
| 7 | Премиум и боты | 2-3 недели |
| 8 | Мобильные приложения | 4-6 недель |
| 9 | Tauri Desktop | 2-3 недели |
| 10 | Деплой и тестирование | 2-3 недели |
| 11 | Юзернеймы, бейджи, админ-панель | 2-3 недели |
| **Итого** | | **24-36 недель (6-9 месяцев)** |

### Вариант B: Telegram-клиенты как основа (рекомендуется для MVP)

| Этап | Описание | Ориентировочное время |
|------|----------|----------------------|
| 1 | Инфраструктура и базовый бэкенд | 2-3 недели |
| 2 | Клиент (Web) — Базовый UI | 2-3 недели |
| 3 | Продвинутый функционал чата | 2-3 недели |
| 4 | Звонки и видеозвонки | 2-3 недели |
| 5 | Групповые чаты и продвинутые функции | 2-3 недели |
| 6 | Кастомизация и настройки | 2-3 недели |
| 7 | Премиум и боты | 2-3 недели |
| 8A | Мобильные: адаптация Telegram iOS | **2-3 недели** |
| 8B | Мобильные: адаптация Telegram Android | **2-3 недели** |
| 9 | Tauri Desktop | 2-3 недели |
| 10 | Деплой и тестирование | 2-3 недели |
| 11 | Юзернеймы, бейджи, админ-панель | 2-3 недели |
| **Итого** | | **20-30 недель (5-7 месяцев)** |

**Экономия:** 4-6 недель за счёт переиспользования Telegram UI/UX.
