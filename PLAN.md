# PLAN.md — Детальный план разработки Bestieverse Bot

## Принципы плана

- Каждый этап — самодостаточная единица, заканчивается компилируемым кодом
- Unit-тесты пишутся в том же этапе, что и реализуемый код
- В конце каждого этапа обновляется `docs/architecture.md`
- Этапы идут от фундамента к бизнес-логике: основание → домен → инфраструктура → usecase → доставка
- Финализация тасок реализована через паттерн Router + интерфейс `TaskFinalizer`

---

## Этап 1 — Фундамент проекта ✅

**Цель:** запускаемый Go-проект с конфигурацией, логгером, подключением к БД и миграциями.

### 1.1 Инициализация модуля

- `go mod init github.com/bestieverse/bot`
- Зависимости:
  - `gopkg.in/telebot.v3`
  - `github.com/rs/zerolog`
  - `github.com/joho/godotenv`
  - `gopkg.in/yaml.v3`
  - `github.com/go-sql-driver/mysql`
  - `github.com/golang-migrate/migrate/v4`
  - `github.com/sashabaranov/go-openai`
  - `github.com/stretchr/testify` (dev)
  - `go.uber.org/mock/gomock` (dev)

### 1.2 `pkg/logger/logger.go`

- Zerolog с цветным выводом в stdout (ConsoleWriter)
- `New(level string) zerolog.Logger` — парсит строку уровня, возвращает настроенный логгер
- `WithChat(log zerolog.Logger, chatID int64) zerolog.Logger` — добавляет поле `chat`
- `WithUser(log zerolog.Logger, userID int64, username string) zerolog.Logger` — добавляет поле `user`
- Если `LOG_FILE` задан — дополнительный writer в файл (через `zerolog.MultiLevelWriter`)

### 1.3 `internal/config/config.go`

Структура `Config`:
```
Bot     BotConfig       (BOT_TOKEN)
DB      DBConfig        (DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD)
OpenAI  OpenAIConfig    (OPENAI_API_KEY, OPENAI_MODEL)
Media   MediaConfig     (MEDIA_PATH)
Log     LogConfig       (LOG_LEVEL, LOG_FILE)
Support SupportConfig   (SUPPORT_TELEGRAM_URL)
Timings Timings
Messages Messages
Tasks   []Task          (сортировано по order)
Game    GameMessages
TestMode bool
```

`Load() (*Config, error)`:
1. godotenv загружает `.env`
2. Читает `content/messages.yaml` → `Messages`
3. Glob `content/tasks/task_*.yaml` → `[]Task`, сортировка по `task.Order`
4. Читает `content/game.yml` → `GameMessages`
5. Заполняет все поля из env-переменных

### 1.4 `internal/config/timings.go`

Структура `Timings` (все поля из CLAUDE.md). При `TEST_MODE=true` берёт значения из `TEST_*` переменных, иначе из prod-переменных.

### 1.5 `internal/config/messages.go`

Структура `Messages` — поля для каждого ключа из `content/messages.yaml`. Поля-массивы имеют тип `[]string`. Метод `Random(variants []string) string` — возвращает случайный элемент из непустого слайса.

### 1.6 `internal/config/task.go`

Структуры десериализации YAML:
- `Task` — `id`, `order`, `type`, `media_file`, `text`, `summary`, `subtask`-специфичные поля
- `TaskSummary` — `type`, `text`, `header_text`, `predictions`
- `SubtaskVotingCollage` — `categories` с вложенными `options`
- `SubtaskWhoIsWho` — `questions []TaskQuestion`
- `SubtaskPoll` — `title`, `options []PollOption` (каждая опция может иметь `result_type`, `prepared_text`, `meme_files`)
- `SubtaskAdminOnly` — `messages []TaskMessage`, `questions []TaskQuestion`, `openai.prompt_template`

### 1.7 Миграции

Файлы в `migrations/` (из `migrations/schema.sql`, разбить по таблицам):
- `001_create_games.up.sql` / `.down.sql`
- `002_create_players.up.sql` / `.down.sql`
- `003_create_player_states.up.sql` / `.down.sql`
- `004_create_task_responses.up.sql` / `.down.sql`
- `005_create_task_locks.up.sql` / `.down.sql`
- `006_create_subtask_progress.up.sql` / `.down.sql`
- `007_create_task_results.up.sql` / `.down.sql`
- `008_create_notifications_log.up.sql` / `.down.sql`
- `009_create_referral_clicks.up.sql` / `.down.sql`

### 1.8 `Makefile`

Команды: `run-bot`, `run-notifier`, `run-scheduler`, `docker-up`, `docker-down`, `migrate-up`, `migrate-down`, `test`, `test-integration`, `test-coverage`, `mock-gen`, `lint`. Переменная `MIGRATE_DSN`.

### 1.9 `docker-compose.yml`

Сервисы `bot`, `notifier`, `scheduler`, `mysql`. `mysql` с healthcheck; остальные — `depends_on: {condition: service_healthy}`.

### 1.10 `.env.example`

Все переменные из CLAUDE.md с пустыми значениями и комментариями.

### Unit-тесты этапа 1

**`internal/config/timings_test.go`**
- Тест: при `TEST_MODE=false` берутся prod-значения (`TASK_PUBLISH_INTERVAL=24h`)
- Тест: при `TEST_MODE=true` берутся test-значения (`TEST_TASK_PUBLISH_INTERVAL=2m`)
- Тест: отсутствующая переменная возвращает ошибку

**`internal/config/messages_test.go`**
- Тест: `Random()` на слайсе из одного элемента возвращает его
- Тест: `Random()` на непустом слайсе возвращает один из элементов
- Тест: сериализация ключей messages.yaml корректно маппится в структуру

**`pkg/logger/logger_test.go`**
- Тест: `New("debug")` не возвращает ошибку
- Тест: `WithChat` добавляет корректное поле в контекст логгера
- Тест: `WithUser` добавляет корректные поля в контекст логгера

### Обновление `docs/architecture.md`

Добавить раздел **«Конфигурация и инфраструктура запуска»**:
- Описание трёхслойной конфигурации: `.env` (секреты) + `content/*.yaml` (тексты и контент) + код
- Схема: `config.Load()` читает всё при старте, один `Config`-объект передаётся во все компоненты
- Описание логгера: zerolog, форматы полей `[chat:ID]`, `[user:ID:@name]`, уровни цветов
- Описание структуры миграций: нумерованные SQL-файлы, применяются через `golang-migrate`

### Результат этапа

- `make migrate-up` успешно создаёт схему в БД
- `go build ./...` компилируется без ошибок
- `make test` проходит все тесты этапа

---

## Этап 2 — Доменный слой ✅

**Цель:** определить все сущности и интерфейсы репозиториев. Никакой логики — только контракты.

### 2.1 `internal/domain/entity/`

**`game.go`**
```
type GameStatus string
const GamePending, GameActive, GameFinished GameStatus

type Game struct {
    ID                   uint64
    ChatID               int64
    ChatName             string
    AdminUserID          int64
    AdminUsername        string
    Status               GameStatus
    CurrentTaskOrder     int
    CurrentTaskPublishedAt *time.Time
    ActivePollID         string
    CreatedAt            time.Time
    StartedAt            *time.Time
    FinishedAt           *time.Time
}
```

**`player.go`** — `ID`, `GameID`, `TelegramUserID`, `Username`, `FirstName`, `SkipCount`, `JoinedAt`

**`player_state.go`** — `ID`, `GameID`, `PlayerID`, `State PlayerStateType` (idle|awaiting_answer), `TaskID`, `UpdatedAt`

**`task_response.go`** — `ID`, `GameID`, `PlayerID`, `TaskID`, `Status ResponseStatus` (answered|skipped), `ResponseData json.RawMessage`, `CreatedAt`

**`task_lock.go`** — `ID`, `GameID`, `TaskID`, `PlayerID`, `AcquiredAt`, `ExpiresAt`

**`subtask_progress.go`** — `ID`, `GameID`, `PlayerID`, `TaskID`, `QuestionIndex`, `AnswersData json.RawMessage`, `UpdatedAt`

**`task_result.go`** — `ID`, `GameID`, `TaskID`, `ResultData json.RawMessage`, `FinalizedAt`

**`notification.go`** — `ID`, `GameID`, `PlayerID`, `TaskID`, `SentAt`

### 2.2 `internal/domain/repository/`

По одному файлу на каждый репозиторий — только интерфейс:

**`game.go`**
```
type GameRepository interface {
    Create(ctx, *Game) (*Game, error)
    GetByChatID(ctx, chatID int64) (*Game, error)
    GetByID(ctx, id uint64) (*Game, error)
    GetByActivePollID(ctx, pollID string) (*Game, error)
    UpdateStatus(ctx, id uint64, status GameStatus) error
    UpdateCurrentTask(ctx, id uint64, order int, publishedAt time.Time) error
    SetActivePollID(ctx, id uint64, pollID string) error
    GetAllActive(ctx) ([]*Game, error)
    SetFinished(ctx, id uint64) error
}
```

**`player.go`**
```
type PlayerRepository interface {
    Create(ctx, *Player) (*Player, error)
    GetByGameAndTelegramID(ctx, gameID uint64, telegramUserID int64) (*Player, error)
    GetAllByGame(ctx, gameID uint64) ([]*Player, error)
    IncrementSkipCount(ctx, playerID uint64) error
    Delete(ctx, playerID uint64) error
}
```

**`player_state.go`**
```
type PlayerStateRepository interface {
    Upsert(ctx, *PlayerState) error
    GetByPlayerAndGame(ctx, gameID, playerID uint64) (*PlayerState, error)
    GetAllAwaitingByGame(ctx, gameID uint64) ([]*PlayerState, error)
    SetIdle(ctx, gameID, playerID uint64) error
}
```

**`task_response.go`**
```
type TaskResponseRepository interface {
    Create(ctx, *TaskResponse) error
    GetByPlayerAndTask(ctx, gameID, playerID uint64, taskID string) (*TaskResponse, error)
    GetAllByTask(ctx, gameID uint64, taskID string) ([]*TaskResponse, error)
    CountAnsweredByTask(ctx, gameID uint64, taskID string) (int, error)
}
```

**`task_lock.go`**
```
type TaskLockRepository interface {
    Acquire(ctx, gameID uint64, taskID string, playerID uint64, expiresAt time.Time) error
    Get(ctx, gameID uint64, taskID string) (*TaskLock, error)
    Release(ctx, gameID uint64, taskID string) error
    ReleaseExpired(ctx) error
}
```

**`subtask_progress.go`**
```
type SubtaskProgressRepository interface {
    Upsert(ctx, *SubtaskProgress) error
    Get(ctx, gameID, playerID uint64, taskID string) (*SubtaskProgress, error)
    Delete(ctx, gameID, playerID uint64, taskID string) error
}
```

**`task_result.go`**
```
type TaskResultRepository interface {
    Create(ctx, *TaskResult) error
    GetByTask(ctx, gameID uint64, taskID string) (*TaskResult, error)
}
```

**`notification.go`**
```
type NotificationRepository interface {
    Create(ctx, *NotificationLog) error
    Exists(ctx, gameID, playerID uint64, taskID string) (bool, error)
    GetUnnotifiedPlayers(ctx, gameID uint64, taskID string) ([]*Player, error)
}
```

### Unit-тесты этапа 2

Домен не содержит логики — тесты не нужны. Компиляция подтверждает корректность типов.

### Обновление `docs/architecture.md`

Добавить раздел **«Доменный слой»**:
- Описание принципа: `domain/` не импортирует ничего из `infrastructure/` или `delivery/`
- Таблица всех сущностей с описанием их назначения
- Описание интерфейсов репозиториев — почему контракт в домене, реализация снаружи
- Диаграмма зависимостей между сущностями (в текстовом виде)

### Результат этапа

- `go build ./...` компилируется
- Все entity-структуры и repository-интерфейсы определены

---

## Этап 3 — Инфраструктура ✅

**Цель:** реализовать все репозитории, медиа-хранилище, Telegram-клиент, formatter, lock manager.

### 3.1 `internal/infrastructure/mysql/repository/`

Один файл на таблицу, реализует соответствующий интерфейс из `domain/repository/`:
- `game.go` — все методы `GameRepository`; `UpdateCurrentTask` обновляет `current_task_order` и `current_task_published_at` одним запросом
- `player.go` — все методы `PlayerRepository`
- `player_state.go` — `Upsert` через `INSERT ... ON DUPLICATE KEY UPDATE`
- `task_response.go` — `GetAllByTask` возвращает отсортированные по `created_at` записи
- `task_lock.go` — `Acquire` через `INSERT IGNORE`, затем `Get` для проверки что записал именно этот player; `ReleaseExpired` — `DELETE WHERE expires_at < NOW()`
- `subtask_progress.go` — `Upsert` через `INSERT ... ON DUPLICATE KEY UPDATE`
- `task_result.go` — простой CRUD
- `notification.go` — `GetUnnotifiedPlayers`: `JOIN players LEFT JOIN notifications_log` для данной `(game_id, task_id)`; возвращает игроков у которых нет записи в `notifications_log`

Все реализации:
- Принимают `*sql.DB` в конструкторе
- Оборачивают ошибки: `fmt.Errorf("mysql/game.GetByChatID: %w", err)`
- Используют `ctx` во всех запросах

### 3.2 `internal/infrastructure/media/local.go`

Интерфейс (объявить здесь же):
```
type Storage interface {
    GetFile(name string) (*tele.Document, error)
    GetPhoto(name string) (*tele.Photo, error)
    GetAnimation(name string) (*tele.Animation, error)
}
```

Реализация `LocalStorage`:
- Конструктор принимает `basePath string`
- Все методы: `return tele.FromDisk(filepath.Join(s.basePath, name))`
- Проверяют существование файла перед возвратом (`os.Stat`)

### 3.3 `internal/infrastructure/telegram/client.go`

Тонкая обёртка над `telebot.v3`:
- `NewBot(token string, settings tele.Settings) (*tele.Bot, error)`
- Вспомогательные методы с логированием ошибок: не паникуют, возвращают ошибку
- Метод `DeleteAfter(bot *tele.Bot, msg *tele.Message, delay time.Duration)` — горутина с `time.Sleep` + `bot.Delete`

### 3.4 `internal/infrastructure/openai/client.go`

- Структура `Client` с `sashabaranov/go-openai`
- `NewClient(apiKey, model string) *Client`
- `GenerateCollage(ctx context.Context, prompt string) ([]byte, error)`:
  - Вызывает `client.CreateImage` с моделью из конфига
  - Декодирует base64 из ответа → возвращает PNG-байты

### 3.5 `pkg/formatter/telegram.go`

- `Mention(userID int64, username, firstName string) string` — формирует `<a href="tg://user?id=X">имя</a>`; если `username` пустой — использует `firstName`
- `RenderTemplate(tmpl string, data any) (string, error)` — `text/template` с `{{.Mention}}` и другими полями
- Константа `ParseMode = tele.ModeHTML`

### 3.6 `pkg/lock/manager.go`

- `LockManager` принимает `TaskLockRepository` и `timeout time.Duration`
- `TryAcquire(ctx, gameID uint64, taskID string, playerID uint64) (bool, error)`:
  1. `repo.ReleaseExpired(ctx)`
  2. Попытка `repo.Acquire(ctx, ..., time.Now().Add(timeout))`
  3. `repo.Get(ctx, gameID, taskID)` → проверить что `lock.PlayerID == playerID`
  4. Если да → `true`; если чужой лок → `false`
- `Release(ctx, gameID uint64, taskID string) error`

### Unit-тесты этапа 3

**`pkg/formatter/telegram_test.go`**
- Тест: `Mention` с непустым username → `<a href="tg://user?id=123">@user</a>`
- Тест: `Mention` без username → `<a href="tg://user?id=123">FirstName</a>`
- Тест: `RenderTemplate("Привіт {{.Mention}}", data)` → корректная подстановка
- Тест: `RenderTemplate` с синтаксической ошибкой в шаблоне → возвращает ошибку

**`pkg/lock/manager_test.go`** (используют gomock мок `TaskLockRepository`)
- Тест: `TryAcquire` — лок свободен → `Acquire` вызван, возвращает `true`
- Тест: `TryAcquire` — лок занят другим игроком → возвращает `false`, `Acquire` не вызван повторно
- Тест: `TryAcquire` — `ReleaseExpired` всегда вызывается первым
- Тест: `Release` вызывает `repo.Release`

### Обновление `docs/architecture.md`

Добавить раздел **«Инфраструктурный слой»**:
- Описание паттерна: `infrastructure/` реализует интерфейсы, не содержит бизнес-логики
- MySQL-репозитории: почему `INSERT IGNORE` для локов, почему `ON DUPLICATE KEY UPDATE` для прогресса
- `media.Storage`: текущая реализация (`LocalStorage`) и контракт для будущего S3
- `pkg/formatter`: единственная точка рендеринга — зачем это нужно
- `pkg/lock/manager`: описание алгоритма захвата лока через БД

### Результат этапа

- `go build ./...` компилируется
- `make test` — проходят тесты formatter и lock manager
- `make mock-gen` генерирует моки для всех репозиториев

---

## Этап 4 — Управление игрой (join / leave / start)

**Цель:** работающий бот — добавляется в чат, регистрирует игроков, запускает игру.

### 4.1 `cmd/bot/main.go`

1. `config.Load()`
2. `logger.New(cfg.Log.Level)`
3. Подключение к MySQL (`sql.Open`), ping
4. Инициализация всех репозиториев
5. Инициализация usecase-ов
6. `tele.NewBot` + регистрация хэндлеров
7. `bot.Start()`

### 4.2 `internal/usecase/game/create.go`

`Create(ctx, chatID int64, chatName string, adminUser tele.User) (*entity.Game, error)`:
1. `gameRepo.GetByChatID` — если уже есть, вернуть `nil, nil` (идемпотентность)
2. `gameRepo.Create` — status=pending
3. `playerRepo.Create` — для админа
4. `playerStateRepo.Upsert` — state=idle для админа
5. Лог: `game created, admin: ...`

### 4.3 `internal/usecase/game/join.go`

`Join(ctx, chatID int64, user tele.User) error`:
1. `gameRepo.GetByChatID` — game.Status должен быть pending
2. Если `game.AdminUserID == user.ID` → отправить `join_admin_already` (удалить через 10s)
3. `playerRepo.GetByGameAndTelegramID` — если нашли → отправить `join_already_member` (удалить через 10s)
4. `playerRepo.Create` + `playerStateRepo.Upsert` (idle)
5. Отправить случайное `join_welcome` с `formatter.Mention`

### 4.4 `internal/usecase/game/leave.go`

`InitiateLeave(ctx, game, player) error` — проверяет admin, отправляет `leave_confirm` с кнопками

`ConfirmLeave(ctx, game, player, confirmMsgID) error`:
1. `playerRepo.Delete(player.ID)` — CASCADE удалит связанные записи
2. Отправить `leave_success`

`CancelLeave(ctx, game, player, confirmMsgID) error`:
1. Удалить сообщение с подтверждением
2. Отправить `cancel_leave` (удалить через 10s)

### 4.5 `internal/usecase/game/start.go`

`Start(ctx, game, player) error`:
1. Проверить `player.TelegramUserID == game.AdminUserID`; если нет → `start_only_admin` (удалить через 10s)
2. Удалить сообщение с кнопкой start
3. `gameRepo.UpdateStatus(game.ID, GameActive)`
4. С `TaskInfoInterval`: отправить `start_game_message_1` (GIF) → `start_game_message_2`
5. Вызвать `task/publish.Publish(ctx, game)` для первой таски

### 4.6 `internal/delivery/bot/handler/chat_member.go`

- Обработчик `tele.ChatMemberUpdated`
- Фильтр: событие = "бот добавлен в чат" (новый участник — наш бот)
- Вызов `usecase/game.Create`
- Отправка стартового сообщения + клавиатуры `JoinKeyboard()`
- Через `JoinMessageDelay` — отправка сообщения с `StartKeyboard()`

### 4.7 `internal/delivery/bot/handler/callback.go`

Маршрутизация по `c.Data`:
```
"game:join"         → game.Join
"game:leave"        → game.InitiateLeave
"game:leave_confirm"→ game.ConfirmLeave
"game:leave_cancel" → game.CancelLeave
"game:start"        → game.Start
```

### 4.8 `internal/delivery/bot/middleware/player_check.go`

- Получает game по `c.Chat().ID`, player по `(game.ID, c.Sender().ID)`
- Если не найден → `not_in_game` (удалить через 10s), `return`
- Иначе → устанавливает game и player в контекст

### 4.9 `internal/delivery/bot/middleware/recover.go`

Recovery от паники: `log.Error().Stack().Err(err).Msg("panic recovered")`

### 4.10 `internal/delivery/bot/keyboard/factory.go`

- `JoinKeyboard(supportURL string) *tele.ReplyMarkup`
- `StartKeyboard() *tele.ReplyMarkup`
- `LeaveConfirmKeyboard() *tele.ReplyMarkup`
- `TaskKeyboard(taskID string) *tele.ReplyMarkup`

### Unit-тесты этапа 4

Все тесты используют gomock-моки репозиториев и мок `tele.Bot` (или отдельный интерфейс отправки сообщений).

**`usecase/game/create_test.go`**
- Тест: новый чат — вызываются `gameRepo.Create`, `playerRepo.Create`, `playerStateRepo.Upsert`
- Тест: чат уже имеет игру — ни один репозиторий не вызывается повторно
- Тест: ошибка `gameRepo.Create` — возвращается ошибка, обёрнутая с контекстом

**`usecase/game/join_test.go`**
- Тест: успешное присоединение — создаются player + player_state, отправляется `join_welcome`
- Тест: уже является участником — `playerRepo.Create` не вызывается, отправляется `join_already_member`
- Тест: является админом — `playerRepo.Create` не вызывается, отправляется `join_admin_already`
- Тест: игра не в статусе pending — ранний выход без действий

**`usecase/game/leave_test.go`**
- Тест: `InitiateLeave` для не-админа — отправляется `leave_confirm` с кнопками
- Тест: `InitiateLeave` для админа — отправляется `leave_admin_blocked`
- Тест: `ConfirmLeave` — вызывается `playerRepo.Delete`, отправляется `leave_success`
- Тест: `CancelLeave` — сообщение удаляется, отправляется `cancel_leave`

**`usecase/game/start_test.go`**
- Тест: не-админ нажимает start — `gameRepo.UpdateStatus` не вызывается, отправляется `start_only_admin`
- Тест: админ нажимает start — `gameRepo.UpdateStatus(active)` вызван, отправлены 2 инфо-сообщения

### Обновление `docs/architecture.md`

Добавить раздел **«Управление игрой»**:
- Жизненный цикл игры: pending → active → finished
- Схема обработки события "бот добавлен в чат"
- Описание middleware-цепочки: `recover → player_check → handler`
- Принцип keyboard factory: все inline-кнопки создаются только через фабрику
- Таблица callback_data форматов для кнопок игры

### Результат этапа

Проверяемые TC: TC-01, TC-02, TC-03, TC-04, TC-05, TC-06, TC-07, TC-08, TC-09

---

## Этап 5 — Базовый ответ на таску (question_answer)

**Цель:** полный цикл таски типа `question_answer` — публикация, ответ, пропуск. Запуск scheduler.

### 5.1 `internal/usecase/task/publish.go`

`Publish(ctx, game *entity.Game) error`:
1. Найти таску с `order = game.CurrentTaskOrder + 1`; если не найдена → вызвать game finish
2. `gameRepo.UpdateCurrentTask(game.ID, task.Order, time.Now())`
3. Логика по `task.Type`:
   - `question_answer`: `media.GetAnimation(task.MediaFile)` → отправить с текстом + `TaskKeyboard(task.ID)`
   - `poll_then_task`: см. этап 9
   - `admin_only`: см. этап 11
4. Лог: `task published: task_NN for game:ID`

### 5.2 `internal/usecase/task/request_answer.go`

`RequestAnswer(ctx, game, player *entity.Player, task *config.Task) error`:
1. `taskResponseRepo.GetByPlayerAndTask` — если есть → `already_answered` (удалить через 10s)
2. Для `question_answer`: нет лока, сразу к шагу 3
3. `playerStateRepo.Upsert(state=awaiting_answer, taskID=task.ID)`
4. Отправить случайное `awaiting_answer` (удалить через 10s)

### 5.3 `internal/usecase/task/answer.go`

`Answer(ctx, game, player *entity.Player, msg *tele.Message) error`:
1. `playerStateRepo.GetByPlayerAndGame` — если `state != awaiting_answer` → `return nil` (игнорировать)
2. `taskResponseRepo.Create(status=answered, response_data=nil)`
3. `playerStateRepo.SetIdle(game.ID, player.ID)`
4. Отправить `answer_accepted` (удалить через 10s)
5. Лог: `task answered: task_NN`

### 5.4 `internal/usecase/task/skip.go`

`Skip(ctx, game, player *entity.Player, taskID string) error`:
1. `taskResponseRepo.GetByPlayerAndTask` — если статус `answered` → `already_answered`; если `skipped` → `already_skipped` (оба удалять через 10s)
2. Проверить `player.SkipCount < 3`; если нет → `skip_no_remaining` (удалить через 10s)
3. `playerRepo.IncrementSkipCount(player.ID)`
4. `taskResponseRepo.Create(status=skipped)`
5. Выбор сообщения по `player.SkipCount + 1`:
   - 1 пропуск (остаток 2): `skip_with_remaining_2`
   - 2 пропуска (остаток 1): `skip_with_remaining_1`
   - 3 пропуска (остаток 0): `skip_last`

### 5.5 `internal/delivery/bot/handler/message.go`

Обрабатывает все входящие сообщения в группах:
1. `gameRepo.GetByChatID` — если нет или не active → ignore
2. `playerRepo.GetByGameAndTelegramID` — если нет → ignore
3. `playerStateRepo.GetByPlayerAndGame` — если `state != awaiting_answer` → ignore
4. По `state.TaskID` определить тип таски:
   - `question_answer` → `task/answer.Answer`
   - Остальные → соответствующие хэндлеры (добавятся в следующих этапах)

### 5.6 Обновление `callback.go`

```
"task:request:{taskID}" → task.RequestAnswer
"task:skip:{taskID}"    → task.Skip
```

### 5.7 `cmd/scheduler/main.go` (базовый)

Цикл каждую минуту:
1. `gameRepo.GetAllActive`
2. Для каждой игры:
   - Проверить нужно ли публиковать следующую таску:
     `game.CurrentTaskPublishedAt + TaskPublishInterval <= now` (или первая таска если `CurrentTaskOrder == 0`)
   - Если да → `task/publish.Publish`
   - Проверить нужно ли финализировать текущую таску:
     `game.CurrentTaskPublishedAt + TaskFinalizeOffset <= now` и `task_results` для текущей таски нет
   - Если да → `task/finalize.FinalizeRouter.Finalize` (см. этап 6)

### Unit-тесты этапа 5

**`usecase/task/publish_test.go`**
- Тест: публикация первой таски — `gameRepo.UpdateCurrentTask` вызван с order=1, медиафайл запрошен
- Тест: публикация когда все таски пройдены — вызывается game finish

**`usecase/task/request_answer_test.go`**
- Тест: успешный запрос — state меняется на `awaiting_answer`, сообщение отправлено
- Тест: уже ответил — state не меняется, `already_answered` отправлен
- Тест: уже пропустил — state не меняется, `already_answered` отправлен

**`usecase/task/answer_test.go`**
- Тест: юзер в состоянии `awaiting_answer` → response создан, state=idle, `answer_accepted` отправлен
- Тест: юзер в состоянии `idle` → ранний выход, ни один репозиторий write-метод не вызван
- Тест: `taskResponseRepo.Create` возвращает ошибку → ошибка проброшена наверх

**`usecase/task/skip_test.go`**
- Тест: первый пропуск (SkipCount=0) → IncrementSkipCount вызван, отправлен `skip_with_remaining_2`
- Тест: второй пропуск (SkipCount=1) → отправлен `skip_with_remaining_1`
- Тест: третий пропуск (SkipCount=2) → отправлен `skip_last`
- Тест: четвёртый пропуск (SkipCount=3) → IncrementSkipCount не вызван, `skip_no_remaining`
- Тест: повторный пропуск (уже есть ответ skipped) → `already_skipped`, IncrementSkipCount не вызван
- Тест: пропуск уже отвеченной таски → `already_answered`, IncrementSkipCount не вызван

### Обновление `docs/architecture.md`

Добавить раздел **«Жизненный цикл таски»**:
- Схема состояний таски: опубликована → ожидание ответов → финализация
- Схема состояний игрока: `idle ↔ awaiting_answer`
- Описание message handler: как бот определяет к какой таске относится сообщение
- Описание scheduler: как определяется момент публикации и финализации
- Таблица callback_data форматов для кнопок тасок

### Результат этапа

Проверяемые TC: TC-10, TC-11, TC-12, TC-13, TC-14, TC-15, TC-16, TC-17, TC-18

---

## Этап 6 — Финализация тасок (Router + TaskFinalizer) и нотификатор

**Цель:** подведение итогов тасок через паттерн Router+Interface; отправка напоминаний.

### 6.1 Дизайн: `internal/usecase/task/finalize/`

Пакет `finalize` содержит интерфейс и конкретные реализации:

**`interface.go`**
```go
// TaskFinalizer — стратегия финализации для конкретного summary.type
type TaskFinalizer interface {
    // Finalize принимает всё необходимое и отправляет итоговое сообщение в чат.
    // Сохранение task_result — ответственность конкретного финализатора.
    Finalize(
        ctx         context.Context,
        game        *entity.Game,
        task        *config.Task,
        responses   []*entity.TaskResponse,
    ) error

    // SupportedSummaryType возвращает значение summary.type, которое обрабатывает этот финализатор.
    SupportedSummaryType() string
}

// SummaryType — константы для summary.type из YAML
const (
    SummaryTypeText          = "text"
    SummaryTypePredictions   = "predictions"
    SummaryTypeWhoIsWho      = "who_is_who_results"
    SummaryTypeCollage       = "collage"
    SummaryTypeOpenAICollage = "openai_collage"
)
```

**`router.go`** — `FinalizeRouter`
```go
type FinalizeRouter struct {
    finalizers       map[string]TaskFinalizer
    taskResultRepo   repository.TaskResultRepository
    taskResponseRepo repository.TaskResponseRepository
    gameRepo         repository.GameRepository
    bot              *tele.Bot
    messages         *config.Messages
    timings          *config.Timings
}

func NewFinalizeRouter(finalizers ...TaskFinalizer) *FinalizeRouter

// Finalize — точка входа для scheduler
func (r *FinalizeRouter) Finalize(ctx context.Context, game *entity.Game, task *config.Task) error:
    1. responses := taskResponseRepo.GetAllByTask(ctx, game.ID, task.ID)
    2. Если len(responses) == 0 → отправить случайное na_answers; return
    3. finalizer, ok := r.finalizers[task.Summary.Type]
    4. Если !ok → log.Error + return fmt.Errorf("unknown summary type: %s", ...)
    5. err := finalizer.Finalize(ctx, game, task, responses)
    6. Если err == nil → лог "task finalized: task_NN for game:ID"
    7. Проверить если это последняя таска → r.finishGame(ctx, game)
```

**`text.go`** — `TextFinalizer`
```go
// SupportedSummaryType: "text"
// Finalize: отправить task.Summary.Text в чат
```

**`predictions.go`** — `PredictionsFinalizer`
```go
// SupportedSummaryType: "predictions"
// Finalize:
//   1. Отправить task.Summary.HeaderText
//   2. Для каждого игрока из responses: случайное предсказание из task.Summary.Predictions с {{.Mention}}
//   3. Сохранить task_result с {"type": "predictions"}
```

**`who_is_who.go`** — `WhoIsWhoFinalizer` (заглушка в этом этапе, полная реализация в этапе 8)
```go
// SupportedSummaryType: "who_is_who_results"
// Finalize: подсчёт голосов + отправка результатов (реализуется в этапе 8)
```

**`collage.go`** — `CollageFinalizer` (заглушка в этом этапе, полная реализация в этапе 7)
```go
// SupportedSummaryType: "collage"
// Finalize: генерация коллажа через OpenAI (реализуется в этапе 7)
```

**`openai_collage.go`** — `OpenAICollageFinalizer` (заглушка, реализуется в этапе 11)
```go
// SupportedSummaryType: "openai_collage"
```

**`game_end.go`** — вспомогательный метод `FinalizeRouter.finishGame`:
1. `gameRepo.SetFinished(ctx, game.ID)`
2. Отправить `game.final_message_1` (GIF)
3. Через `TaskInfoInterval` отправить `game.final_message_2` с реферальной ссылкой
4. Лог: `game finished`

### 6.2 Обновление `cmd/scheduler/main.go`

Вызовы `FinalizeRouter.Finalize` по расписанию; добавить обработку нескольких игр параллельно через goroutines (с `sync.WaitGroup`).

### 6.3 `internal/usecase/notification/send_reminder.go`

`SendReminders(ctx context.Context) error`:
1. `gameRepo.GetAllActive`
2. Для каждой игры — найти текущую таску по `game.CurrentTaskOrder`
3. Проверить прошло ли `ReminderDelay` с `game.CurrentTaskPublishedAt`:
  - ВАЖНО: проверка времени на уровне игры, не игрока
  - if time.Since(game.CurrentTaskPublishedAt) < cfg.Timings.ReminderDelay → skip, continue
4. `notificationRepo.GetUnnotifiedPlayers(ctx, game.ID, task.ID)` — игроки без ответа и без уведомления
5. Для каждого: `notificationRepo.Create` + отправить случайное `reminder` с `{{.Mention}}`
6. Лог каждого напоминания

### 6.4 `cmd/notifier/main.go`

Цикл каждую минуту: вызов `notification.SendReminders(ctx)`.

### 6.5 cmd/scheduler/main.go

При старте:
1. Загрузить все игры со статусом active
2. Для каждой игры вычислить сколько времени осталось до следующего события:
   - до публикации следующей таски: (current_task_published_at + TaskPublishInterval) - now()
   - до финализации текущей таски: (current_task_published_at + TaskFinalizeOffset) - now()
3. Запустить time.AfterFunc с вычисленными задержками (не фиксированный тикер)
4. После каждого события — запланировать следующее

Важно: если задержка отрицательная (событие уже должно было случиться) — 
выполнить немедленно с задержкой 0.

### Unit-тесты этапа 6

**`usecase/task/finalize/router_test.go`**
- Тест: `Finalize` когда responses пуст → `na_answers` отправлен, `TaskFinalizer.Finalize` не вызван
- Тест: `Finalize` с ответами — диспатч к правильному финализатору по `task.Summary.Type`
- Тест: `Finalize` с неизвестным `summary.type` → возвращает ошибку, ничего не отправлено
- Тест: после финализации последней таски (order=12) → `gameRepo.SetFinished` вызван
- Тест: финализация не последней таски → `gameRepo.SetFinished` не вызван
- *(Используют mock-реализации `TaskFinalizer` через gomock)*

**`usecase/task/finalize/text_test.go`**
- Тест: `TextFinalizer.Finalize` → `task.Summary.Text` отправлен в чат
- Тест: `SupportedSummaryType()` возвращает `"text"`

**`usecase/task/finalize/predictions_test.go`**
- Тест: `PredictionsFinalizer.Finalize` с 3 игроками → `header_text` + 3 предсказания отправлены
- Тест: каждый игрок получает своё `{{.Mention}}` в тексте предсказания
- Тест: `task_result` создан с корректными данными

**`usecase/notification/send_reminder_test.go`**
- Тест: игрок без ответа, без уведомления → уведомление отправлено, запись создана
- Тест: игрок с ответом → уведомление не отправлено
- Тест: игрок уже получал уведомление (есть запись в `notifications_log`) → повторного нет
- Тест: прошло меньше `ReminderDelay` → ни одному игроку уведомление не отправлено, проверяем именно game.CurrentTaskPublishedAt, а не время ответа/присоединения игрока

### Обновление `docs/architecture.md`

Добавить раздел **«Паттерн финализации тасок: Router + TaskFinalizer»**:
- Диаграмма: `scheduler → FinalizeRouter → [TextFinalizer | PredictionsFinalizer | ...]`
- Объяснение выбора паттерна: добавление нового типа summary = новый файл без изменения router
- Контракт `TaskFinalizer`: ответственность за отправку сообщения и сохранение `task_result`
- Таблица: `summary.type` → реализующий класс → этап реализации
- Описание notifier: отдельный сервис, работает независимо от bot

### Результат этапа

Проверяемые TC: TC-26, TC-27, TC-28, TC-29

---

## Этап 7 — Сабтаска voting_collage (Таска 2)

**Цель:** голосование по категориям с эксклюзивным локом; генерация коллажа через OpenAI.

### 7.1 `internal/usecase/task/subtask/voting_collage.go`

Зависимости: `LockManager`, `SubtaskProgressRepository`, `TaskResponseRepository`, `PlayerStateRepository`, `media.Storage`, `openai.Client`, `tele.Bot`, `config.Messages`, `config.Timings`

`HandleRequestAnswer(ctx, game, player, task) error`:
1. `lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)`; если `false` → `subtask_locked` (удалить через 10s)
2. `subtaskProgressRepo.Get` — если нет, создать (question_index=0, answers_data={})
3. `playerStateRepo.Upsert(awaiting_answer, task.ID)`
4. Отправить категорию `task.Subtask.Categories[progress.QuestionIndex]`:
   - `media.GetPhoto(category.MediaFile)` + `category.HeaderText` + `CategoryKeyboard(task, idx)`

`HandleCategoryChoice(ctx, game, player, task, categoryID, optionID string) error`:
1. Проверить `lockManager` — если лок не принадлежит player → `subtask_locked` (удалить через 10s)
2. Добавить выбор в `progress.AnswersData[categoryID] = optionID`
3. `subtaskProgressRepo.Upsert(progress)`
4. `progress.QuestionIndex++`
5. Если остались категории → отправить следующую
6. Если все категории пройдены:
   - `taskResponseRepo.Create(answered, response_data=progress.AnswersData)`
   - `subtaskProgressRepo.Delete`
   - `lockManager.Release`
   - `playerStateRepo.SetIdle`
   - Отправить случайное `followup` из task YAML

### 7.2 Реализация `CollageFinalizer` (из этапа 6, файл `finalize/collage.go`)

`Finalize(ctx, game, task, responses) error`:
1. Собрать голоса: для каждого `response.ResponseData` (JSON `{categoryID: optionID}`) — подсчитать по каждой категории
2. Победитель категории = опция с max голосов; при равенстве — первый по порядку в YAML
3. Сохранить в `task_results.ResultData`: `{"drink": "smoothie", "music": "tina_karol", ...}`
4. Сформировать prompt для OpenAI: перечислить победившие варианты с метками
5. Отправить `summary.SendingText`
6. `openaiClient.GenerateCollage(ctx, prompt)` → `imageBytes`
7. Отправить изображение как `tele.Photo` + `summary.ReadyText`
8. Опционально: отправить PDF-версию высокого качества с `summary.HqText`

### 7.3 Обновление `callback.go`

```
"task02:choice:{categoryID}:{optionID}" → votingCollage.HandleCategoryChoice
```

### 7.4 Обновление `keyboard/factory.go`

`CategoryKeyboard(task *config.Task, catIdx int) *tele.ReplyMarkup` — кнопки вариантов текущей категории

### Unit-тесты этапа 7

**`usecase/task/subtask/voting_collage_test.go`**
- Тест: `HandleRequestAnswer` — лок свободен → захвачен, прогресс создан, первая категория отправлена
- Тест: `HandleRequestAnswer` — лок занят → `subtask_locked`, прогресс не создан
- Тест: `HandleCategoryChoice` — промежуточный выбор → прогресс обновлён, следующая категория отправлена
- Тест: `HandleCategoryChoice` — последняя категория → response создан, прогресс удалён, лок освобождён, `followup` отправлен
- Тест: `HandleCategoryChoice` — лок принадлежит другому игроку → ранний выход

**`usecase/task/finalize/collage_test.go`**
- Тест: подсчёт голосов — 3 игрока выбрали `smoothie`, 1 — `cappuccino` → победитель `smoothie`
- Тест: ничья по голосам → победитель — первый по порядку в YAML
- Тест: нет ответов → покрывается тестом `router_test.go` (na_answers)
- Тест: `openaiClient.GenerateCollage` вызван с промптом, содержащим победившие варианты
- Тест: `task_result` создан с корректным JSON

### Обновление `docs/architecture.md`

Добавить раздел **«Сабтаска voting_collage»**:
- Диаграмма flow: request_answer → lock → show category → choice → next category / finish
- Объяснение эксклюзивного лока: почему нельзя отвечать параллельно
- Описание `subtask_progress`: промежуточное состояние между вопросами
- Описание алгоритма подсчёта голосов и формирования OpenAI prompt

### Результат этапа

Проверяемые TC: TC-20, TC-21

---

## Этап 8 — Сабтаска who_is_who (Таска 4)

**Цель:** последовательные вопросы с выбором участника, подведение итогов.

### 8.1 `internal/usecase/task/subtask/who_is_who.go`

`HandleRequestAnswer(ctx, game, player, task) error`:
1. `lockManager.TryAcquire` → если занят → `subtask_locked` (удалить через 10s)
2. `subtaskProgressRepo.Get` или создать (question_index=0)
3. `playerStateRepo.Upsert(awaiting_answer, task.ID)`
4. `playerRepo.GetAllByGame(game.ID)` — список участников
5. Отправить `task.Questions[0].Text` + `PlayerSelectionKeyboard(players, "q1")`

`HandlePlayerChoice(ctx, game, player, task, questionID string, chosenTelegramUserID int64) error`:
1. Проверить лок
2. `progress.AnswersData[questionID] = chosenTelegramUserID`
3. `subtaskProgressRepo.Upsert`
4. `progress.QuestionIndex++`
5. Если остались вопросы → `PlayerSelectionKeyboard(players, nextQuestionID)`
6. Если все вопросы пройдены:
   - `taskResponseRepo.Create(answered, response_data=progress.AnswersData)`
   - `subtaskProgressRepo.Delete`
   - `lockManager.Release`
   - `playerStateRepo.SetIdle`
   - Отправить подтверждение

### 8.2 Реализация `WhoIsWhoFinalizer` (из этапа 6, файл `finalize/who_is_who.go`)

`Finalize(ctx, game, task, responses) error`:
1. Собрать голоса: `map[questionID]map[telegramUserID]int`
2. Для каждого вопроса: победитель = игрок с max голосами
3. Сохранить в `task_results.ResultData`: `{"q1": telegramUserID, "q2": telegramUserID, ...}`
4. Сформировать итоговый текст: для каждого вопроса — `вопрос → @упоминание победителя`
5. Отправить `task.Summary.HeaderText` + итоговый текст

### 8.3 Обновление `callback.go`

```
"task04:player:{questionID}:{chosenTelegramUserID}" → whoIsWho.HandlePlayerChoice
```

### 8.4 Обновление `keyboard/factory.go`

`PlayerSelectionKeyboard(players []*entity.Player, questionID string) *tele.ReplyMarkup`

### Unit-тесты этапа 8

**`usecase/task/subtask/who_is_who_test.go`**
- Тест: `HandleRequestAnswer` — лок свободен → прогресс создан, первый вопрос с кнопками игроков отправлен
- Тест: `HandleRequestAnswer` — лок занят → `subtask_locked`
- Тест: `HandlePlayerChoice` — промежуточный ответ → прогресс обновлён, следующий вопрос отправлен
- Тест: `HandlePlayerChoice` — последний ответ → response создан, лок освобождён
- Тест: кнопки содержат всех игроков игры (3 игрока → 3 кнопки)

**`usecase/task/finalize/who_is_who_test.go`**
- Тест: 3 игрока выбрали одного и того же для q1 → он победитель
- Тест: ничья → побеждает первый по order в YAML
- Тест: `task_result` содержит корректный JSON с `telegramUserID` победителей
- Тест: итоговое сообщение содержит `header_text` и упоминания победителей

### Обновление `docs/architecture.md`

Добавить раздел **«Сабтаска who_is_who»**:
- Flow диаграмма
- Структура `response_data` в `task_responses`
- Алгоритм подсчёта голосов при финализации

### Результат этапа

Проверяемые TC: TC-22

---

## Этап 9 — Таска 10: Telegram Poll + ветвление

**Цель:** нативный Telegram-опрос, определение победителя, ветвление на нужную сабтаску.

### 9.1 Новая миграция

`010_add_active_poll_id.up.sql`: `ALTER TABLE games ADD COLUMN active_poll_id VARCHAR(64)`. Обновить `GameRepository.GetByActivePollID`.

### 9.2 Обновление `task/publish.go` для `poll_then_task`

1. Отправить GIF + текст
2. `bot.SendPoll(chat, task.Poll.Title, options, closeDate=now+PollDuration)`
3. `gameRepo.SetActivePollID(game.ID, poll.ID)`

### 9.3 `internal/delivery/bot/handler/poll_answer.go`

Обработчик `tele.Poll` (закрытие опроса):
1. `gameRepo.GetByActivePollID(poll.ID)` — найти игру
2. Определить победителя: опция с max `VoterCount`; ничья — первая в списке; все 0 — первая
3. `taskResultRepo.Create({"winning_option": optionID})`
4. `gameRepo.SetActivePollID(game.ID, "")` — очистить
5. Найти опцию в task YAML по `optionID`:
   - `result_type == "question_answer"` → опубликовать `prepared_text` как таску (`task/publish`)
   - `result_type == "meme_voiceover"` → инициировать meme flow (этап 10)

### 9.4 `internal/usecase/task/subtask/poll.go`

`PublishPollTask(ctx, game, task, winningOption *config.PollOption) error`:
- Для `question_answer`: отправить `winningOption.PreparedText` + `TaskKeyboard(task.ID)`
- Для `meme_voiceover`: отправить анонс, зарегистрировать ожидание (details в этапе 10)

### Unit-тесты этапа 9

**`usecase/task/subtask/poll_test.go`**
- Тест: определение победителя — опция с 3 голосами побеждает над опцией с 1
- Тест: ничья (2:2) → победитель — первая по порядку в YAML
- Тест: все 0 голосов → победитель — первая опция
- Тест: при `result_type=question_answer` → `prepared_text` отправлен в чат с кнопками
- Тест: `active_poll_id` очищается после закрытия опроса

### Обновление `docs/architecture.md`

Добавить раздел **«Таска 10: Poll и ветвление»**:
- Схема ветвления: poll closed → determine winner → question_answer / meme_voiceover
- Хранение `active_poll_id` в таблице `games` — почему так, а не отдельная таблица
- Алгоритм определения победителя при ничьей

### Результат этапа

Проверяемые TC: TC-23

---

## Этап 10 — Сабтаска meme_voiceover (Таска 10b)

**Цель:** последовательная озвучка мемов с эксклюзивным локом.

### 10.1 `internal/usecase/task/subtask/meme_voiceover.go`

Зависимости такие же как у `voting_collage`, плюс список `memeFiles` из `task.Poll.Options[memes].MemeFiles`.

`HandleRequestAnswer(ctx, game, player, task) error`:
1. `lockManager.TryAcquire` → если занят → `subtask_locked` (удалить через 10s)
2. `subtaskProgressRepo.Get` или создать (question_index=0)
3. `playerStateRepo.Upsert(awaiting_answer, task.ID+":meme")`
4. Отправить первый мем: `media.GetAnimation(memeFiles[0])` без сопровождающего текста

`HandleAnswer(ctx, game, player, task, msg *tele.Message) error`:
1. `subtaskProgressRepo.Get` — получить текущий прогресс
2. Проверить что `lockManager` — лок принадлежит player
3. Сохранить `progress.AnswersData["meme_N"] = msg.Text`
4. `progress.QuestionIndex++`
5. Если остались мемы → отправить следующий GIF (НЕ удалять предыдущие)
6. Если все мемы озвучены:
   - `taskResponseRepo.Create(answered)`
   - `subtaskProgressRepo.Delete`
   - `lockManager.Release`
   - `playerStateRepo.SetIdle`
   - Отправить финальное сообщение благодарности

### 10.2 Обновление `handler/message.go`

При `state.TaskID` содержащем `":meme"` суффикс → `memeVoiceover.HandleAnswer`.

### Unit-тесты этапа 10

**`usecase/task/subtask/meme_voiceover_test.go`**
- Тест: `HandleRequestAnswer` — лок свободен → первый мем отправлен
- Тест: `HandleRequestAnswer` — лок занят → `subtask_locked`
- Тест: `HandleAnswer` — промежуточный → следующий мем отправлен, сообщения НЕ удаляются
- Тест: `HandleAnswer` — последний мем → response создан, лок освобождён, финальное сообщение
- Тест: ответ не-текстовым сообщением (фото) → принимается (озвучка может быть любым типом)

### Обновление `docs/architecture.md`

Добавить раздел **«Сабтаска meme_voiceover»**:
- Отличие от других сабтасок: ответы не удаляются, остаются в чате как публичная озвучка
- Структура `subtask_progress.answers_data` для мемов
- Почему `state.TaskID` включает суффикс `":meme"` для маршрутизации

### Результат этапа

Проверяемые TC: TC-24

---

## Этап 11 — Таска 12: admin_only + финал игры

**Цель:** последовательные вопросы только для админа, OpenAI коллаж, финальные сообщения.

### 11.1 Обновление `task/publish.go` для `admin_only`

1. Отправить `task.Messages[0]` (GIF + текст)
2. Через `TaskInfoInterval` отправить `task.Messages[1]` (текст + кнопки `[Хочу відповісти]` `[Пропустити]`)

### 11.2 `internal/usecase/task/subtask/admin_only.go`

`HandleRequestAnswer(ctx, game, player, task) error`:
1. Проверить `player.TelegramUserID == game.AdminUserID`; если нет → `task12_only_admin` (удалить через 10s)
2. `subtaskProgressRepo.Get` или создать (question_index=0)
3. `playerStateRepo.Upsert(awaiting_answer, task.ID+":admin")`
4. Отправить `task.Questions[0].Text` + `Task12QuestionKeyboard(task.Questions[0])`
5. Сохранить message_id отправленного вопроса (в `subtask_progress.answers_data` как метаданные)

`HandleButtonPress(ctx, game, player, task, questionID string) error`:
1. Отправить случайное `task12_awaiting_answer` (не удалять — юзер должен видеть)
2. Ждём текстового ответа (state уже `awaiting_answer`)

`HandleAnswer(ctx, game, player, task, msg *tele.Message) error`:
1. Получить `progress`, `currentQuestionID = task.Questions[progress.QuestionIndex].ID`
2. Сохранить `progress.AnswersData[currentQuestionID] = msg.Text`
3. Удалить сообщение-вопрос (его message_id сохранён в прогрессе)
4. Оставить ответ юзера в чате
5. Отправить случайное `task12_reply`
6. `progress.QuestionIndex++`
7. Если остались вопросы → отправить следующий
8. Если все вопросы пройдены → вызвать `completeAdminTask(ctx, game, task, progress.AnswersData)`

`completeAdminTask(ctx, game, task, answers map[string]string) error`:
1. Рендерить `task.OpenAI.PromptTemplate` с `answers` через `formatter.RenderTemplate`
2. Отправить `task.Summary.SendingText`
3. `openaiClient.GenerateCollage(ctx, prompt)` → imageBytes
4. Отправить изображение + `task.Summary.ReadyText`
5. `taskResponseRepo.Create(answered, {"answers": answers})`
6. `subtaskProgressRepo.Delete`
7. `playerStateRepo.SetIdle`

### 11.3 Реализация `OpenAICollageFinalizer` (файл `finalize/openai_collage.go`)

`Finalize(ctx, game, task, responses) error`:
- Для task_12 `task_result` уже создан в `admin_only.completeAdminTask`
- Этот финализатор только: `taskResultRepo.GetByTask` → проверить что коллаж готов
- Финальные сообщения игры — через `FinalizeRouter.finishGame` (уже реализован в этапе 6)

### 11.4 Обновление `callback.go`

```
"task12:question:{questionID}" → adminOnly.HandleButtonPress
```

### 11.5 Обновление `keyboard/factory.go`

`Task12QuestionKeyboard(question *config.TaskQuestion) *tele.ReplyMarkup` — одна кнопка с `question.ButtonLabel`

### Unit-тесты этапа 11

**`usecase/task/subtask/admin_only_test.go`**
- Тест: `HandleRequestAnswer` — не-админ → `task12_only_admin`, прогресс не создан
- Тест: `HandleRequestAnswer` — админ → первый вопрос + кнопка отправлены
- Тест: `HandleButtonPress` — `task12_awaiting_answer` отправлен
- Тест: `HandleAnswer` — промежуточный → следующий вопрос отправлен, предыдущий удалён
- Тест: `HandleAnswer` — последний ответ → `openaiClient.GenerateCollage` вызван с корректным промптом
- Тест: `HandleAnswer` — последний ответ → response создан с `{"answers": {...}}`

**`usecase/task/finalize/openai_collage_test.go`**
- Тест: `OpenAICollageFinalizer.SupportedSummaryType()` возвращает `"openai_collage"`
- Тест: `Finalize` проверяет наличие `task_result` → если нет → логирует ошибку

### Обновление `docs/architecture.md`

Добавить раздел **«Таска 12: admin_only и финал игры»**:
- Почему только один игрок (админ) отвечает
- Схема взаимодействия: кнопка → `awaiting_answer` → текстовый ответ → следующий вопрос
- Как ответы из `subtask_progress` попадают в OpenAI prompt через Go-шаблон
- Финальный flow: `finishGame` → final_message_1 → final_message_2 → реферальная ссылка

### Результат этапа

Проверяемые TC: TC-25, TC-30

---

## Этап 12 — Тестовые команды, Docker, интеграционные тесты

**Цель:** готовность к деплою: тест-режим, контейнеры, полное покрытие.

### 12.1 Тестовые команды (`TEST_MODE=true`)

В `cmd/bot/main.go` регистрировать только если `cfg.TestMode`:
- `/test_task_N` → `task/publish.Publish` для таски N (установить `game.CurrentTaskOrder = N-1` перед вызовом)
- `/test_finalize_N` → `FinalizeRouter.Finalize` для таски N
- `/test_notify` → `notification.SendReminders`
- `/test_state` → JSON-дамп: game, players, player_states, последние 5 task_responses
- `/test_reset` → `gameRepo.Delete(game.ID)` — CASCADE удалит всё

### 12.2 Finalize Dockerfiles

Multi-stage `Dockerfile` для каждого из `cmd/bot`, `cmd/notifier`, `cmd/scheduler`:
- Stage `builder`: `golang:1.22-alpine`, `go build -o /app/service ./cmd/{service}`
- Stage `runtime`: `alpine:latest`, копирует бинарник + `content/` + `assets/`

### 12.3 `docker-compose.yml` (финальная версия)

```yaml
services:
  mysql:
    image: mysql:8.0
    healthcheck: {test: mysqladmin ping, interval: 5s, retries: 10}
    volumes: ["${DB_PATH}:/var/lib/mysql"]
    environment: [MYSQL_DATABASE, MYSQL_USER, MYSQL_PASSWORD, MYSQL_ROOT_PASSWORD]

  bot:
    build: {context: ., dockerfile: cmd/bot/Dockerfile}
    depends_on: {mysql: {condition: service_healthy}}
    env_file: .env
    restart: unless-stopped

  notifier:
    build: {context: ., dockerfile: cmd/notifier/Dockerfile}
    depends_on: {mysql: {condition: service_healthy}}
    env_file: .env
    restart: unless-stopped

  scheduler:
    build: {context: ., dockerfile: cmd/scheduler/Dockerfile}
    depends_on: {mysql: {condition: service_healthy}}
    env_file: .env
    restart: unless-stopped
```

### 12.4 `docker-compose.test.yml`

Отдельный `mysql_test` (порт 3307, БД `gamebot_test`) для `make test-integration`.

### 12.5 Интеграционные тесты

В `_test/integration/`:

**`game_lifecycle_test.go`**
- TestCreateAndJoin: создать игру, присоединить 3 игроков, проверить БД
- TestStartGame: запустить игру, проверить status=active и current_task_order=1

**`task_flow_test.go`**
- TestAnswerTask: опубликовать таску, ответить, проверить task_responses
- TestSkipTask: пропустить 3 таски, убедиться что 4-й пропуск отклонён
- TestFinalizeText: финализировать таску с `summary.type=text`, проверить task_results

Тесты используют реальную тестовую БД (`docker-compose.test.yml`), применяют миграции в `TestMain`.

### Обновление `docs/architecture.md`

Добавить раздел **«Деплой и тестирование»**:
- Архитектура Docker Compose: три независимых сервиса + MySQL
- Схема: что происходит при падении notifier (bot продолжает работу)
- Стратегия тестирования: unit (gomock) + integration (реальная БД)
- Таблица: какие тест-кейсы покрываются unit, какие — интеграционными
- Описание тестовых команд: как использовать при разработке и QA

### Результат этапа

Проверяемые TC: TC-31, TC-32, весь чеклист перед деплоем

---

## Сводная таблица этапов

| Этап | Название | Unit-тесты | TC покрываются |
|------|----------|------------|----------------|
| 1 | Фундамент | config, logger | — |
| 2 | Доменный слой | — | — |
| 3 | Инфраструктура | formatter, lock manager | — |
| 4 | Управление игрой | create, join, leave, start | TC-01–09 |
| 5 | question_answer | publish, request_answer, answer, skip | TC-10–19 |
| 6 | Finalize Router + нотификатор | router, text, predictions, send_reminder | TC-26–29 |
| 7 | voting_collage | voting_collage, collage finalizer | TC-20–21 |
| 8 | who_is_who | who_is_who, who_is_who finalizer | TC-22 |
| 9 | Poll + ветвление | poll | TC-23 |
| 10 | meme_voiceover | meme_voiceover | TC-24 |
| 11 | admin_only + финал | admin_only, openai_collage finalizer | TC-25, TC-30 |
| 12 | Docker + интеграция | integration tests | TC-31–32 |

---

## Полный список файлов

```
go.mod / go.sum
.env.example
Makefile
docker-compose.yml
docker-compose.test.yml

cmd/bot/main.go
cmd/bot/Dockerfile
cmd/notifier/main.go
cmd/notifier/Dockerfile
cmd/scheduler/main.go
cmd/scheduler/Dockerfile

internal/config/config.go
internal/config/timings.go        + timings_test.go
internal/config/messages.go       + messages_test.go
internal/config/task.go

internal/domain/entity/game.go
internal/domain/entity/player.go
internal/domain/entity/player_state.go
internal/domain/entity/task_response.go
internal/domain/entity/task_lock.go
internal/domain/entity/subtask_progress.go
internal/domain/entity/task_result.go
internal/domain/entity/notification.go

internal/domain/repository/game.go
internal/domain/repository/player.go
internal/domain/repository/player_state.go
internal/domain/repository/task_response.go
internal/domain/repository/task_lock.go
internal/domain/repository/subtask_progress.go
internal/domain/repository/task_result.go
internal/domain/repository/notification.go

internal/infrastructure/mysql/repository/game.go
internal/infrastructure/mysql/repository/player.go
internal/infrastructure/mysql/repository/player_state.go
internal/infrastructure/mysql/repository/task_response.go
internal/infrastructure/mysql/repository/task_lock.go
internal/infrastructure/mysql/repository/subtask_progress.go
internal/infrastructure/mysql/repository/task_result.go
internal/infrastructure/mysql/repository/notification.go
internal/infrastructure/media/local.go
internal/infrastructure/telegram/client.go
internal/infrastructure/openai/client.go

pkg/logger/logger.go              + logger_test.go
pkg/formatter/telegram.go         + telegram_test.go
pkg/lock/manager.go               + manager_test.go

internal/usecase/game/create.go              + create_test.go
internal/usecase/game/join.go                + join_test.go
internal/usecase/game/leave.go               + leave_test.go
internal/usecase/game/start.go               + start_test.go

internal/usecase/task/publish.go             + publish_test.go
internal/usecase/task/request_answer.go      + request_answer_test.go
internal/usecase/task/answer.go              + answer_test.go
internal/usecase/task/skip.go                + skip_test.go

internal/usecase/task/finalize/interface.go
internal/usecase/task/finalize/router.go     + router_test.go
internal/usecase/task/finalize/text.go       + text_test.go
internal/usecase/task/finalize/predictions.go + predictions_test.go
internal/usecase/task/finalize/who_is_who.go  + who_is_who_test.go
internal/usecase/task/finalize/collage.go     + collage_test.go
internal/usecase/task/finalize/openai_collage.go + openai_collage_test.go
internal/usecase/task/finalize/game_end.go

internal/usecase/task/subtask/voting_collage.go  + voting_collage_test.go
internal/usecase/task/subtask/who_is_who.go       + who_is_who_test.go
internal/usecase/task/subtask/poll.go             + poll_test.go
internal/usecase/task/subtask/meme_voiceover.go   + meme_voiceover_test.go
internal/usecase/task/subtask/admin_only.go        + admin_only_test.go

internal/usecase/notification/send_reminder.go + send_reminder_test.go

internal/delivery/bot/handler/chat_member.go
internal/delivery/bot/handler/callback.go
internal/delivery/bot/handler/message.go
internal/delivery/bot/handler/poll_answer.go
internal/delivery/bot/middleware/player_check.go
internal/delivery/bot/middleware/recover.go
internal/delivery/bot/keyboard/factory.go

migrations/001_create_games.up.sql / .down.sql
migrations/002_create_players.up.sql / .down.sql
migrations/003_create_player_states.up.sql / .down.sql
migrations/004_create_task_responses.up.sql / .down.sql
migrations/005_create_task_locks.up.sql / .down.sql
migrations/006_create_subtask_progress.up.sql / .down.sql
migrations/007_create_task_results.up.sql / .down.sql
migrations/008_create_notifications_log.up.sql / .down.sql
migrations/009_create_referral_clicks.up.sql / .down.sql
migrations/010_add_active_poll_id.up.sql / .down.sql

docs/architecture.md

_test/integration/game_lifecycle_test.go
_test/integration/task_flow_test.go
```
