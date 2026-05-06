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

## Инфраструктурный слой

### Принцип

`internal/infrastructure/` реализует интерфейсы из `domain/repository/`, но не содержит бизнес-логики. Usecase-код работает только с интерфейсами — конкретные реализации внедряются в `cmd/*/main.go`.

### MySQL-репозитории

Каждый файл в `internal/infrastructure/mysql/repository/` реализует один интерфейс из `domain/repository/`:

| Файл | Интерфейс | Ключевые особенности |
|---|---|---|
| `game.go` | `GameRepository` | `SetActivePollID` использует `sql.NullString`; `UpdateStatus` автоматически ставит `started_at`/`finished_at` |
| `player.go` | `PlayerRepository` | `COALESCE` для nullable `username`/`first_name` |
| `player_state.go` | `PlayerStateRepository` | `Upsert` через `INSERT ... ON DUPLICATE KEY UPDATE` |
| `task_response.go` | `TaskResponseRepository` | `GetAllByTask` возвращает только `answered`, отсортированные по `created_at` |
| `task_lock.go` | `TaskLockRepository` | `Acquire` через `INSERT IGNORE` — только первый вызов победит по уникальному ключу |
| `subtask_progress.go` | `SubtaskProgressRepository` | `Upsert` через `INSERT ... ON DUPLICATE KEY UPDATE` для безопасного обновления прогресса |
| `task_result.go` | `TaskResultRepository` | Простой CRUD |
| `notification.go` | `NotificationRepository` | `GetUnnotifiedPlayers` — LEFT JOIN players с `notifications_log` и `task_responses`; возвращает тех, у кого нет обоих |

Все конструкторы принимают `*sql.DB`. Ошибки оборачиваются с контекстом: `fmt.Errorf("mysql/game.Create: %w", err)`.

### media.Storage

Интерфейс `media.Storage` объявлен в `internal/infrastructure/media/local.go`:

```go
type Storage interface {
    GetFile(name string) (*tele.Document, error)
    GetPhoto(name string) (*tele.Photo, error)
    GetAnimation(name string) (*tele.Animation, error)
}
```

`LocalStorage` читает файлы с диска через `tele.FromDisk`. Проверяет существование через `os.Stat` перед возвратом. В будущем можно добавить `S3Storage` без изменения usecase-кода — интерфейс останется тем же.

### pkg/formatter

Единственная точка рендеринга сообщений в HTML-формат для Telegram. Константа `ParseMode = tele.ModeHTML` используется во всех отправках.

Функция `Mention` формирует HTML-ссылку на пользователя; `RenderTemplate` выполняет Go-шаблоны с произвольными данными.

### pkg/lock/manager

`LockManager.TryAcquire` реализует алгоритм захвата эксклюзивного лока через БД:
1. `ReleaseExpired` — чистит истёкшие локи (всегда первым)
2. `Acquire` — `INSERT IGNORE` (только один игрок пишет успешно по уникальному ключу)
3. `Get` → проверить `lock.PlayerID == playerID` — возвращает `true` если этот игрок победил

Это атомарная операция на уровне MySQL — параллельные вызовы от разных игроков корректно разрешаются без дополнительной синхронизации в коде.

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

---

## Управление игрой (Stage 4)

### Жизненный цикл игры

```
[бот добавлен в чат]
        │
        ▼
   status=pending  ← Creator.Create (идемпотентно)
        │
   [admin нажимает "Розпочати гру"]
        │
        ▼
   status=active   ← Starter.Start → UpdateStatus → publish task 1
        │
   [12 тасок пройдено]
        │
        ▼
   status=finished ← task/finalize (Stage 5+)
```

### Обработка события «бот добавлен в чат»

```
tele.OnMyChatMember
        │
        ▼
ChatMemberHandler.OnMyChatMember
        │
        ├── game.Creator.Create(chatID, chatName, adminUser)
        │       └── gameRepo.GetByChatID → уже есть? nil,nil (идемпотент)
        │           gameRepo.Create (status=pending)
        │           playerRepo.Create (admin)
        │           playerStateRepo.Upsert (idle)
        │
        ├── bot.Send(chat, welcomeMsg + JoinKeyboard)
        │
        └── time.Sleep(JoinMessageDelay) → bot.Send(chat, startMsg + StartKeyboard)
```

### Middleware-цепочка

```
Recover → PlayerCheck → Handler
```

- **Recover**: оборачивает `panic` в `log.Error`, предотвращает крэш бота
- **PlayerCheck**: для каждого callback на кнопки таски — проверяет наличие `Game` (по `chatID`) и `Player` (по `game.ID` + `senderID`); кладёт найденные объекты в `c.Set("game", g)` / `c.Set("player", p)`; при отсутствии — отправляет `not_in_game` и прерывает цепочку

Маршруты без PlayerCheck: `game:join` (пользователь ещё не в игре).

### Inline-кнопки (keyboard factory)

Все `*tele.ReplyMarkup` создаются только через `internal/delivery/bot/keyboard/factory.go`:

| Функция | Callback data | Применение |
|---|---|---|
| `JoinKeyboard()` | `game:join` | Приглашение войти в игру |
| `StartKeyboard()` | `game:start` | Кнопка "Розпочати гру" (только для admin) |
| `LeaveConfirmKeyboard()` | `game:leave_confirm` / `game:leave_cancel` | Подтверждение выхода |
| `TaskKeyboard(taskID)` | `task:<id>:answer` / `task:<id>:skip` | Кнопки ответа на таску |

### Формат callback_data

```
game:join
game:leave
game:leave_confirm
game:leave_cancel
game:start
task:<taskID>:answer
task:<taskID>:skip
```

### Интерфейс Sender

`usecase/game` определяет минимальный интерфейс отправки сообщений:

```go
type Sender interface {
    Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error)
    Delete(msg tele.Editable) error
}
```

`*tele.Bot` реализует этот интерфейс. В тестах используется `mockSender`.

### TaskPublisher (заглушка до Stage 5)

`Starter` зависит от интерфейса `TaskPublisher`:

```go
type TaskPublisher interface {
    Publish(ctx context.Context, game *entity.Game) error
}
```

В `cmd/bot/main.go` пока передаётся `nil`; `Starter.Start` проверяет наличие перед вызовом. В Stage 5 будет подключён реальный `task.Publisher`.
