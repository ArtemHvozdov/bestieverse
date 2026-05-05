# CLAUDE.md — Telegram Game Bot

Этот файл читается Claude Code при каждом запуске. Следуй этим инструкциям строго.

---

## Обзор проекта

Telegram-бот для проведения групповой игры в Telegram-чатах. Бот добавляется в существующий чат, создаёт игру, управляет участниками и последовательно публикует 12 тасок на протяжении нескольких дней.

**Ключевые особенности:**
- Несколько игр одновременно в разных чатах (полная изоляция логики)
- Один игрок может участвовать в нескольких играх одновременно
- Три независимых сервиса: бот, нотификатор, планировщик
- Состояние полностью персистентно в MySQL (рестарт безопасен)

---

## Технический стек

| Компонент | Технология |
|---|---|
| Язык | Go 1.22+ |
| Telegram | gopkg.in/telebot.v3 |
| База данных | MySQL 8.0 |
| Миграции | golang-migrate/migrate |
| Конфиг | godotenv + YAML (gopkg.in/yaml.v3) |
| Логирование | zerolog (цветной вывод в терминал) |
| OpenAI | sashabaranov/go-openai (модель gpt-image-1) |
| Контейнеры | Docker + Docker Compose |
| Тесты | testify + gomock |

---

## Структура проекта

```
/
├── cmd/
│   ├── bot/main.go              ← точка входа бота
│   ├── notifier/main.go         ← сервис напоминаний
│   └── scheduler/main.go        ← сервис публикации тасок и итогов
│
├── internal/
│   ├── domain/
│   │   ├── entity/              ← Go-структуры сущностей (без логики)
│   │   │   ├── game.go
│   │   │   ├── player.go
│   │   │   ├── task.go
│   │   │   ├── task_response.go
│   │   │   ├── task_lock.go
│   │   │   ├── player_state.go
│   │   │   └── notification.go
│   │   └── repository/          ← ТОЛЬКО интерфейсы, никаких реализаций
│   │       ├── game.go
│   │       ├── player.go
│   │       ├── task_response.go
│   │       ├── task_lock.go
│   │       ├── player_state.go
│   │       └── notification.go
│   │
│   ├── usecase/                 ← ВСЯ бизнес-логика здесь
│   │   ├── game/
│   │   │   ├── create.go        ← создание игры при добавлении бота в чат
│   │   │   ├── join.go          ← кнопка "Приєднатися до гри"
│   │   │   ├── leave.go         ← кнопка "Вийти з гри"
│   │   │   └── start.go         ← кнопка "Розпочати гру" (только admin)
│   │   ├── task/
│   │   │   ├── publish.go       ← публикация таски (вызывается scheduler)
│   │   │   ├── answer.go        ← обработка ответа юзера на таску
│   │   │   ├── skip.go          ← кнопка "Пропустити"
│   │   │   ├── request_answer.go ← кнопка "Хочу відповісти"
│   │   │   ├── finalize.go      ← подведение итогов таски (вызывается scheduler)
│   │   │   └── subtask/
│   │   │       ├── voting_collage.go  ← таска 2: голосование + коллаж
│   │   │       ├── who_is_who.go      ← таска 4: кто из нас
│   │   │       ├── poll.go            ← таска 10: Telegram Poll
│   │   │       ├── meme_voiceover.go  ← таска 10b: озвучка мемов
│   │   │       └── admin_only.go      ← таска 12: только админ отвечает
│   │   ├── player/
│   │   │   └── check_membership.go   ← проверка: в игре ли юзер
│   │   └── notification/
│   │       └── send_reminder.go
│   │
│   ├── infrastructure/
│   │   ├── mysql/
│   │   │   └── repository/      ← реализации интерфейсов из domain/repository
│   │   ├── telegram/
│   │   │   └── client.go        ← тонкая обёртка над telebot.v3
│   │   ├── openai/
│   │   │   └── client.go        ← клиент генерации коллажей (gpt-image-1)
│   │   └── media/
│   │       └── local.go         ← чтение медиафайлов (интерфейс для будущего S3)
│   │
│   ├── delivery/
│   │   └── bot/
│   │       ├── handler/
│   │       │   ├── chat_member.go    ← бот добавлен в чат
│   │       │   ├── callback.go       ← все inline-кнопки
│   │       │   ├── message.go        ← входящие сообщения (ответы на таски)
│   │       │   └── poll_answer.go    ← голоса в Telegram Poll
│   │       ├── middleware/
│   │       │   ├── player_check.go   ← middleware: в игре ли юзер
│   │       │   └── recover.go        ← recovery от паники
│   │       └── keyboard/
│   │           └── factory.go        ← все фабрики inline-клавиатур
│   │
│   └── config/
│       ├── config.go            ← загрузка .env и YAML
│       └── timings.go           ← ВСЕ временные интервалы (см. ниже)
│
├── pkg/
│   ├── logger/
│   │   └── logger.go            ← цветной zerolog, формат: [LEVEL] [chat_id] [user_id:username] msg
│   ├── formatter/
│   │   └── telegram.go          ← единственная точка отправки сообщений
│   └── lock/
│       └── manager.go           ← менеджер эксклюзивных локов через БД
│
├── content/
│   ├── tasks/
│   │   ├── task_01.yaml
│   │   ├── task_02.yaml
│   │   └── ... (task_03 — task_12)
│   ├── messages.yaml            ← все системные и вариативные тексты
│   └── game.yaml                ← стартовые и финальные сообщения
│
├── assets/
│   └── media/                   ← все 84 медиафайла (gif, jpg, pdf)
│       ├── tasks/               ← task_01.gif ... task_12.gif
│       ├── task_02/             ← 40 файлов для коллажа таски 2
│       └── task_10/             ← 20 мемов для озвучки
│       └── game/                ← файл для начала игры и финального сообщения
│
├── migrations/                  ← SQL миграции (нумерованные: 001_, 002_, ...)
├── docs/                        ← документация
│   ├── architecture.md
│   ├── database.md
│   └── test_cases.md
├── .env                         ← НЕ коммитить в git
├── .env.example                 ← коммитить, без секретов
├── docker-compose.yml
├── docker-compose.test.yml      ← для тестов с тестовой БД
├── Makefile
└── CLAUDE.md                    ← этот файл
```

---

## Правила разработки (СТРОГО)

### 1. Тексты — только в YAML, никогда в Go

```go
// ❌ ЗАПРЕЩЕНО
bot.Send(chat, "🎯 Завдання прийнято!")

// ✅ ПРАВИЛЬНО
msg := cfg.Messages.TaskAnswerAccepted
bot.Send(chat, msg, tele.ModeHTML)
```

Все тексты хранятся в `content/messages.yaml`. Каждый текст имеет ключ. Go-код использует только ключи через структуру `config.Messages`.

### 2. Форматирование текста — только HTML, через formatter

Telegram ParseMode всегда `tele.ModeHTML`. Использовать **только** функции из `pkg/formatter/telegram.go`.

```go
// Теги в YAML-файлах:
// <b>жирный</b>
// <i>курсив</i>
// <a href="https://...">ссылка</a>
// <code>моноширинный</code>
// Тег юзера: <a href="tg://user?id={{.UserID}}">@{{.Username}}</a>
// Смайлики вставляются напрямую: 🎉
```

### 3. Временные интервалы — только в timings.go

```go
// ❌ ЗАПРЕЩЕНО
time.Sleep(10 * time.Second)

// ✅ ПРАВИЛЬНО
time.Sleep(cfg.Timings.DeleteMessageDelay)
```

Все интервалы в `internal/config/timings.go`. При разработке/тестах значения берутся из `.env` (`TEST_MODE=true`).

### 4. Архитектурные границы

- `domain/` не импортирует ничего из `infrastructure/` или `delivery/`
- `usecase/` не импортирует `delivery/` и `infrastructure/` напрямую — только через интерфейсы из `domain/repository/`
- `delivery/handler/` вызывает только `usecase/`, никогда не работает с БД напрямую
- `infrastructure/` реализует интерфейсы, не содержит бизнес-логики

### 5. Обработка ошибок

```go
// Всегда оборачивать с контекстом
if err != nil {
    return fmt.Errorf("usecase/game.Join: %w", err)
}
```

### 6. Логирование

Использовать только `pkg/logger`. Формат каждого лога:
```
[INFO]  [chat:123456789] [user:987654321:@username] действие: детали
[ERROR] [chat:123456789] [user:987654321:@username] ошибка: details
```

Уровни цветов в терминале (zerolog):
- `DEBUG` — серый
- `INFO`  — зелёный  
- `WARN`  — жёлтый
- `ERROR` — красный
- `FATAL` — красный + жирный

### 7. Тесты

- Каждый usecase покрывается unit-тестами через моки (gomock)
- Файл теста рядом с файлом: `answer.go` → `answer_test.go`
- Интеграционные тесты в `_test/integration/` (работают с тестовой БД)
- Запуск: `make test` (unit), `make test-integration` (интеграционные)

### 8. Медиафайлы

Использовать только через интерфейс `media.Storage`:
```go
type Storage interface {
    GetFile(name string) (*tele.Document, error)
    GetPhoto(name string) (*tele.Photo, error)
    GetAnimation(name string) (*tele.Animation, error) // GIF
}
```
Текущая реализация — `infrastructure/media/local.go`. В будущем — S3 без изменения usecase.

---

## Временные интервалы (timings.go)

```go
type Timings struct {
    // Сообщения
    DeleteMessageDelay     time.Duration // 10s — удаление временных сообщений
    JoinMessageDelay       time.Duration // 1s  — задержка перед "Розпочати гру"
    TaskInfoInterval       time.Duration // 1s  — интервал между инфо-сообщениями
    
    // Лок эксклюзивных сабтасок
    SubtaskLockTimeout     time.Duration // 15m — таймаут захвата лока
    
    // Планировщик
    TaskPublishInterval    time.Duration // 24h (prod) / настраивается через .env
    TaskFinalizeOffset     time.Duration // 23h (prod) — через сколько после публикации итоги
    PollDuration           time.Duration // настраивается в task_10.yaml
    
    // Нотификатор
    ReminderDelay          time.Duration // 12h (prod) / настраивается через .env
}
```

В `.env`:
```
TASK_PUBLISH_INTERVAL=24h
TASK_FINALIZE_OFFSET=23h  
REMINDER_DELAY=12h
SUBTASK_LOCK_TIMEOUT=15m
DELETE_MESSAGE_DELAY=10s
TEST_MODE=false
# При TEST_MODE=true все интервалы берутся из TEST_* переменных
TEST_TASK_PUBLISH_INTERVAL=2m
TEST_TASK_FINALIZE_OFFSET=1m50s
TEST_REMINDER_DELAY=30s
```

---

## Переменные окружения (.env.example)

```env
# Telegram
BOT_TOKEN=

# MySQL
DB_HOST=localhost
DB_PORT=3306
DB_NAME=gamebot
DB_USER=gamebot
DB_PASSWORD=
DB_PATH=./data/mysql    # путь к папке данных для Docker volume

# OpenAI
OPENAI_API_KEY=
OPENAI_MODEL=gpt-image-1

# Медиафайлы
MEDIA_PATH=./assets/media

# Логи
LOG_LEVEL=info          # debug | info | warn | error
LOG_FILE=./logs/bot.log # если пусто — только stdout

# Техподдержка (ссылка в кнопке)
SUPPORT_TELEGRAM_URL=https://t.me/username

# Тестирование
TEST_MODE=false
TEST_TASK_PUBLISH_INTERVAL=2m
TEST_TASK_FINALIZE_OFFSET=1m50s
TEST_REMINDER_DELAY=30s
TEST_POLL_DURATION=1m
```

---

## Команды Makefile

```makefile
make run-bot          # запустить бота локально
make run-notifier     # запустить нотификатор локально
make run-scheduler    # запустить планировщик локально
make docker-up        # docker compose up -d (все сервисы)
make docker-down      # docker compose down
make migrate-up       # применить миграции
make migrate-down     # откатить последнюю миграцию
make test             # unit тесты
make test-integration # интеграционные тесты
make test-coverage    # покрытие в HTML
make mock-gen         # сгенерировать моки из интерфейсов
make lint             # golangci-lint
```

---

## Тестовые команды бота (для разработки)

Бот регистрирует следующие команды для тестирования (только в TEST_MODE=true):

```
/test_task_1   — опубликовать таску 1 прямо сейчас
/test_task_2   — опубликовать таску 2 прямо сейчас
...
/test_task_N   — опубликовать таску N прямо сейчас
/test_finalize_N — подвести итоги таски N прямо сейчас
/test_notify   — отправить напоминание всем кто не ответил
/test_state    — показать текущее состояние игры в этом чате (JSON)
/test_reset    — сбросить игру в этом чате (только для разработки)
```

---

## Docker Compose (структура)

Три сервиса + MySQL:
```
services:
  bot:        build: ./cmd/bot
  notifier:   build: ./cmd/notifier
  scheduler:  build: ./cmd/scheduler
  mysql:      image: mysql:8.0
```

Все три сервиса независимы. Падение `notifier` не влияет на `bot`.

---

## Типы тасок (task type)

| Тип | Описание | Таски |
|---|---|---|
| `question_answer` | Базовый: GIF + текст, ответ любым медиа | 1,3,5,6,7,8,9,11 |
| `voting_collage` | Сабтаска: выбор варианта → коллаж | 2 |
| `who_is_who` | Сабтаска: выбор участника → итог | 4 |
| `poll_then_task` | Telegram Poll → результат определяет таску | 10 |
| `meme_voiceover` | Сабтаска таски 10: 5 мемов → тексты | 10b |
| `admin_only` | Только админ отвечает → OpenAI коллаж | 12 |

---

## Статусы игры (game.status)

```
pending    — бот добавлен, ждём нажатия "Розпочати гру"
active     — игра идёт
finished   — финал пройден
```

## Статусы игрока (player_state.state)

```
idle              — нет активного действия
awaiting_answer   — юзер нажал "Хочу відповісти", ждём сообщение
```

## Статусы ответа (task_response.status)

```
answered  — юзер дал ответ
skipped   — юзер пропустил
```

---

## Важные бизнес-правила (не нарушать)

1. **Проверка членства**: перед обработкой любого нажатия на кнопку таски — проверить, что юзер является участником игры в этом чате (`player_check middleware`)
2. **Один ответ**: юзер не может ответить или пропустить таску дважды
3. **Максимум 3 пропуска** на одну игру (не на таску — на всю игру)
4. **Эксклюзивный лок** для сабтасок 2, 4, 10: таймаут 15 минут, хранится в `task_locks`
5. **Только админ** запускает игру и отвечает на таску 12
6. **Автоудаление** временных сообщений: через 10 секунд (все "ошибочные" сообщения)
7. **Вариативность**: все публичные сообщения берутся рандомно из массива вариантов в `messages.yaml`