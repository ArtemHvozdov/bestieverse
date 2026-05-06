# Architecture — Bestieverse Bot

## Обзор системы

Три независимых Go-сервиса + MySQL:

```
┌─────────────┐   ┌──────────────┐   ┌───────────────┐
│     bot     │   │   notifier   │   │   scheduler   │
│  (telebot)  │   │ (send remind)│   │ (pub/finalize)│
└──────┬──────┘   └──────┬───────┘   └───────┬───────┘
       │                 │                   │
       └─────────────────┴───────────────────┘
                         │
                   ┌─────▼──────┐
                   │   MySQL    │
                   │   8.0      │
                   └────────────┘
```

Каждый сервис независим: падение `notifier` не влияет на `bot`.

---

## Конфигурация и инфраструктура запуска

### Трёхслойная конфигурация

```
.env               ← секреты (BOT_TOKEN, DB_PASSWORD, OPENAI_API_KEY)
content/*.yaml     ← контент (тексты, таски, финальные сообщения)
env vars           ← тайминги и пути (можно задавать через Docker env_file)
```

Функция `config.Load()` читает всё при старте приложения. Единственный объект `*config.Config` передаётся во все компоненты через конструкторы (DI вручную, без контейнеров).

```
config.Load()
  ├── godotenv.Load(".env")          — секреты
  ├── loadTimings(testMode)          — тайминги из env
  ├── loadMessages("content/messages.yaml")
  ├── loadGameMessages("content/game.yml")
  └── loadTasks("content/tasks/")   — сортировка по task.Order
```

### TEST_MODE

При `TEST_MODE=true` `loadTimings` переключается на `TEST_*` переменные (короткие интервалы для разработки). Prod-значения не меняются.

| Prod var              | Test var                   | Default |
|-----------------------|----------------------------|---------|
| TASK_PUBLISH_INTERVAL | TEST_TASK_PUBLISH_INTERVAL | 24h / 2m |
| TASK_FINALIZE_OFFSET  | TEST_TASK_FINALIZE_OFFSET  | 23h / 1m50s |
| REMINDER_DELAY        | TEST_REMINDER_DELAY        | 12h / 30s |
| POLL_DURATION         | TEST_POLL_DURATION         | 24h / 1m |

### Логгер

Пакет `pkg/logger` — тонкая обёртка над `zerolog`.

Формат лога:
```
2006-01-02 15:04:05 INF действие: детали chat=123456789 user=987654321:@username
```

Хелперы `WithChat` / `WithUser` добавляют структурированные поля к дочернему логгеру:
```go
log := logger.New(cfg.Log.Level, cfg.Log.File)
log = logger.WithChat(log, chat.ID)
log = logger.WithUser(log, user.ID, user.Username)
log.Info().Msg("game created")
```

Уровни: DEBUG (серый), INFO (зелёный), WARN (жёлтый), ERROR (красный).

### Миграции

Файлы в `migrations/` — нумерованные SQL-скрипты, применяются через `golang-migrate`:

```
001_create_games.{up,down}.sql
002_create_players.{up,down}.sql
003_create_player_states.{up,down}.sql
004_create_task_responses.{up,down}.sql
005_create_task_locks.{up,down}.sql
006_create_subtask_progress.{up,down}.sql
007_create_task_results.{up,down}.sql
008_create_notifications_log.{up,down}.sql
009_create_referral_clicks.{up,down}.sql
```

`make migrate-up` применяет все непримененные миграции.
`make migrate-down` откатывает последнюю.

Порядок `down`-миграций обратный: удаляет таблицы с FK-зависимостями последними.

---

*(Этот документ обновляется в конце каждого этапа разработки)*

---

## Доменный слой

### Принцип изоляции

Пакет `internal/domain/` не импортирует ничего из `internal/infrastructure/` или `internal/delivery/`. Он содержит только Go-структуры сущностей и Go-интерфейсы репозиториев — никаких реализаций.

```
domain/
├── entity/       ← структуры данных (Game, Player, TaskResponse, ...)
└── repository/   ← интерфейсы (GameRepository, PlayerRepository, ...)
```

Реализации репозиториев живут в `internal/infrastructure/mysql/repository/` и зависят от домена, но не наоборот.

### Сущности

| Структура | Таблица | Назначение |
|---|---|---|
| `Game` | `games` | Игра в конкретном чате; хранит текущую таску и статус |
| `Player` | `players` | Участник конкретной игры; хранит счётчик пропусков |
| `PlayerState` | `player_states` | Текущее состояние игрока: idle или awaiting_answer |
| `TaskResponse` | `task_responses` | Ответ или пропуск игрока на конкретную таску |
| `TaskLock` | `task_locks` | Эксклюзивный лок для сабтасок 2, 4, 10 |
| `SubtaskProgress` | `subtask_progress` | Промежуточный прогресс сабтаски (текущий вопрос + ответы) |
| `TaskResult` | `task_results` | Итог финализации таски (победители, коллаж и т.п.) |
| `NotificationLog` | `notifications_log` | Журнал отправленных напоминаний |

### Интерфейсы репозиториев

Каждый интерфейс определён в отдельном файле `repository/<entity>.go`. Бизнес-логика (`usecase/`) зависит только от этих интерфейсов, что позволяет подменять реализацию (MySQL → другая БД) без изменения usecase-кода.

### Зависимости между сущностями

```
Game ──┬── Player ──── PlayerState
       │         └─── TaskResponse
       │         └─── TaskLock
       │         └─── SubtaskProgress
       │         └─── NotificationLog
       └── TaskResult
```

`Player` всегда принадлежит конкретной `Game` (поле `GameID`). Удаление игрока через `CASCADE` убирает все связанные записи.
