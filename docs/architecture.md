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

Дополнительно при `TEST_MODE=true` команда `make docker-restart` автоматически удаляет директорию `DB_PATH` (bind mount с данными MySQL) перед перезапуском контейнеров. Это обеспечивает запуск с чистой, пустой базой данных при каждом перезапуске в режиме разработки. Таблицы воссоздаются сервисом `migrate` при старте.

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

В `cmd/bot/main.go` теперь подключён реальный `task.Publisher`; заглушка `nil` удалена в Stage 5.

---

## Жизненный цикл таски

### Схема состояний

```
Таска опубликована
       │
       ▼
 [Хочу відповісти]  ←──── player state: idle
       │
       ▼
 player state: awaiting_answer
       │
  (входящее сообщение)
       │
       ▼
 task_response: answered   ──► player state: idle
       │
       ▼
       └─── (scheduler) TaskFinalizeOffset → финализация
```

```
 [Пропустити]   (SkipCount < 3)
       │
       ▼
 task_response: skipped   (player state не меняется)
```

### Схема состояний игрока

```
idle  ──[Хочу відповісти]──►  awaiting_answer
 ▲                                   │
 └──────[Answer / Skip]──────────────┘
```

### Компоненты Stage 5

| Файл | Назначение |
|---|---|
| `usecase/task/publish.go` | Публикует следующую таску; обновляет `game.current_task_order` |
| `usecase/task/request_answer.go` | Кнопка «Хочу відповісти» → переводит игрока в `awaiting_answer` |
| `usecase/task/answer.go` | Входящее сообщение → сохраняет ответ, возвращает в `idle` |
| `usecase/task/skip.go` | Кнопка «Пропустити» → лимит 3 пропуска на игру |
| `delivery/bot/handler/message.go` | Маршрутизирует входящие сообщения группы к Answerer |
| `cmd/scheduler/main.go` | Тикер каждую минуту: проверяет когда публиковать следующую таску |

### Маршрутизация callback_data

| Endpoint | Данные (`c.Data()`) | Хэндлер |
|---|---|---|
| `\ftask:request` | taskID | `OnTaskRequestAnswer` |
| `\ftask:skip` | taskID | `OnTaskSkip` |

`kbd.Data(label, unique, payload)` — telebot v3 передаёт `payload` как `c.Data()`, `unique` используется для роутинга.

### Message handler

`OnMessage` игнорирует сообщения если:
- Нет активной игры в чате
- Отправитель не является игроком
- Игрок не в состоянии `awaiting_answer`

Для таски типа `question_answer` любой медиаконтент принимается как ответ.

### Scheduler

Тикер запускается каждую минуту и для каждой активной игры:
1. `CurrentTaskOrder == 0` → сразу публикует первую таску
2. `CurrentTaskPublishedAt + TaskPublishInterval ≤ now` → публикует следующую таску (если она существует)

Финализация тасок добавляется в Stage 6.

---

## Паттерн финализации тасок: Router + TaskFinalizer

### Диаграмма

```
scheduler
    └─► FinalizeRouter.Finalize(game, task)
            ├─ GetAllByTask → []responses
            ├─ len==0 → send na_answers
            └─ finalizers[task.Summary.Type].Finalize(...)
                    ├─ TextFinalizer         (summary.type = "text")
                    ├─ PredictionsFinalizer  (summary.type = "predictions")
                    ├─ WhoIsWhoFinalizer     (summary.type = "who_is_who_results", Stage 8)
                    ├─ CollageFinalizer      (summary.type = "collage", Stage 7)
                    └─ OpenAICollageFinalizer(summary.type = "openai_collage", Stage 11)
            └─ last task? → finishGame
```

### Почему Router + интерфейс

Добавление нового `summary.type` = создание нового файла, реализующего `TaskFinalizer`. Роутер не меняется. Регистрация — через `NewFinalizeRouter(...finalizers)`.

### Контракт TaskFinalizer

```go
type TaskFinalizer interface {
    Finalize(ctx, game, task, responses) error
    SupportedSummaryType() string
}
```

Каждый финализатор **сам** отправляет итоговое сообщение и сохраняет `task_result` (если нужно).

### Таблица summary.type → финализатор

| `summary.type`       | Финализатор              | Этап |
|----------------------|--------------------------|------|
| `text`               | `TextFinalizer`          | 6    |
| `predictions`        | `PredictionsFinalizer`   | 6    |
| `who_is_who_results` | `WhoIsWhoFinalizer`      | 8    |
| `collage`            | `CollageFinalizer`       | 7    |
| `openai_collage`     | `OpenAICollageFinalizer` | 11   |

### Завершение игры (`finishGame`)

Вызывается роутером после финализации последней таски (`TaskByOrder(task.Order+1) == nil`):
1. `gameRepo.SetFinished(game.ID)`
2. Отправить `game.final_message_1` (GIF `game/final.gif`)
3. `time.Sleep(TaskInfoInterval)`
4. Отправить `game.final_message_2`

### Scheduler (обновлён в Stage 6)

Тикер запускается каждую минуту и обрабатывает каждую активную игру **в отдельной горутине** (`sync.WaitGroup`):
1. `CurrentTaskOrder == 0` → публикует первую таску
2. `CurrentTaskPublishedAt + TaskFinalizeOffset ≤ now` → финализирует текущую таску
3. `CurrentTaskPublishedAt + TaskPublishInterval ≤ now` → публикует следующую таску

---

## Нотификатор (`cmd/notifier`)

Независимый сервис; падение не влияет на бот. Тикер каждую минуту.

### `usecase/notification.ReminderSender`

```
SendReminders():
    GetAllActive games
    for each game:
        if time.Since(game.CurrentTaskPublishedAt) < ReminderDelay → skip
        GetUnnotifiedPlayers(game.ID, task.ID)
        for each player:
            Send reminder with {{.Mention}}
            Create NotificationLog
```

Ключевая деталь: проверка `ReminderDelay` выполняется по `game.CurrentTaskPublishedAt`, а **не** по времени присоединения игрока или его последнего ответа. Один игрок получает максимум одно напоминание на таску (гарантируется через `notifications_log`).

---

## Таска 10: Poll и ветвление

### Схема ветвления

```
scheduler → task/publish (poll_then_task)
    │  sends animation + text
    └─ sends tele.Poll (anonymous, close_date = now + PollDuration)
         │  stores poll.ID in games.active_poll_id
         │
         │  ... players vote ...
         │
    tele.OnPoll fires (poll is_closed=true)
         │
    delivery/handler/poll_answer.go → subtask.PollHandler.HandlePollClosed
         │
         ├─ GetByActivePollID(poll.ID) → finds game
         ├─ determineWinner(poll.Options, task.Poll.Options)
         ├─ taskResultRepo.Create({"winning_option": optionID})
         ├─ SetActivePollID(game.ID, "") — очищаем
         │
         └─ publishFollowUp:
              ├─ result_type="question_answer" → sends PreparedText + task keyboard
              └─ result_type="meme_voiceover"  → stub (полная реализация в Stage 10)
```

### Хранение `active_poll_id`

Поле `active_poll_id VARCHAR(64)` хранится прямо в таблице `games` (а не в отдельной таблице). Причина: в один момент времени у одной игры может быть максимум один активный опрос, а структура отдельной таблицы была бы избыточной. Наличие индекса `INDEX idx_active_poll (active_poll_id)` делает `GetByActivePollID` эффективным.

### Алгоритм определения победителя

Функция `determineWinner(pollResults []tele.PollOption, configOptions []config.PollOption)`:

1. Итерация по опциям в порядке YAML
2. Победитель = опция с наибольшим `VoterCount`
3. При **ничьей** — побеждает первая по порядку в YAML (стабильный порядок)
4. При **всех 0 голосов** — то же правило: первая опция (индекс 0)

Это гарантирует детерминированный результат без случайности.

---

## Таска 12: admin_only и финал игры

### Почему только один игрок (админ) отвечает

Таска 12 (`admin_only`) — финальная задача игры. Только один человек (создатель игры, `game.admin_user_id`) агрегирует мнения команды и отвечает от её имени. Бот проверяет `player.TelegramUserID == game.AdminUserID` при каждом нажатии «Хочу відповісти»; остальным отправляется сообщение `task12_only_admin` с автоудалением.

### Схема взаимодействия

```
Admin нажимает "Хочу відповісти"
    │
    ├─ HandleRequestAnswer
    │   ├─ Проверка admin
    │   ├─ Проверка existing response
    │   ├─ Создать/загрузить subtask_progress
    │   ├─ playerStateRepo.Upsert(awaiting_answer, "task_12:admin")
    │   └─ Отправить Questions[0].Text + Task12QuestionKeyboard
    │
Admin нажимает кнопку (ButtonLabel)  ← callback "\ftask12:question"
    │
    └─ HandleButtonPress
        └─ Отправить task12_awaiting_answer (не удалять)
    │
Admin отправляет текст  ← message handler (суффикс ":admin")
    │
    └─ HandleAnswer
        ├─ Загрузить progress
        ├─ Сохранить answers[questionID] = msg.Text
        ├─ Удалить сообщение-вопрос (сохранённый q_msg_id)
        ├─ Оставить ответ в чате
        ├─ Отправить task12_reply
        │
        ├─ Если остались вопросы → Следующий вопрос + обновить q_msg_id
        │
        └─ Если все вопросы → completeAdminTask
```

### Как ответы попадают в OpenAI prompt через Go-шаблон

Ответы хранятся в `subtask_progress.answers_data` как `{"answers": {"city": "...", ...}, "q_msg_id": N}`. При завершении `completeAdminTask` рендерит `task.OpenAI.PromptTemplate` через `formatter.RenderTemplate` с данными:

```go
struct{ Answers map[string]string }{ Answers: answers }
```

В YAML-шаблоне используется синтаксис Go text/template:
```
{{index .Answers "city"}}
{{index .Answers "concert"}}
```

### Финальный flow

```
completeAdminTask завершена
    │
FinalizeRouter.Finalize (scheduler)
    │
    ├─ OpenAICollageFinalizer.Finalize
    │   └─ taskResultRepo.GetByTask → проверяет что коллаж уже сгенерирован
    │
    └─ r.finishGame (task.Order+1 == nil — нет следующей таски)
        ├─ gameRepo.SetFinished
        ├─ Отправить game.final_message_1 (GIF)
        ├─ time.Sleep(TaskInfoInterval)
        └─ Отправить game.final_message_2 + реферальная ссылка
```

---

## Деплой и тестирование

### Архитектура Docker Compose

Три независимых сервиса + MySQL:

```
services:
  mysql      ← image: mysql:8.0, healthcheck: mysqladmin ping
  bot        ← cmd/bot/Dockerfile,       depends_on: mysql (healthy)
  notifier   ← cmd/notifier/Dockerfile,  depends_on: mysql (healthy)
  scheduler  ← cmd/scheduler/Dockerfile, depends_on: mysql (healthy)
```

Каждый Dockerfile использует **multi-stage build**:
- `builder` (`golang:1.22-alpine`): компилирует бинарник
- `runtime` (`alpine:latest`): копирует бинарник + `content/` + `assets/`

### Независимость сервисов

Падение `notifier` не влияет на `bot` или `scheduler` — каждый сервис работает в отдельном контейнере и взаимодействует только через MySQL.

```
bot падает      → notifier и scheduler продолжают работу
notifier падает → напоминания не отправляются, игра продолжается
scheduler падает→ таски не публикуются и не финализируются; при рестарте scheduler
                  сразу проверяет все активные игры и выполняет пропущенные события
```

### Тестовые команды (TEST_MODE=true)

Регистрируются только если `cfg.TestMode == true`:

| Команда | Действие |
|---------|----------|
| `/test_task_N` | Устанавливает `current_task_order = N-1`, вызывает `publisher.Publish` |
| `/test_finalize_N` | Вызывает `FinalizeRouter.Finalize` для таски N немедленно |
| `/test_notify` | Вызывает `ReminderSender.SendReminders` немедленно |
| `/test_state` | Отправляет JSON-дамп: game + players + player_states + последние ответы |
| `/test_reset` | Удаляет игру через `gameRepo.Delete` — CASCADE удаляет все связанные записи |

### Стратегия тестирования

| Уровень | Инструменты | Покрытие |
|---------|-------------|----------|
| Unit | `testify` + `gomock` | Все usecase-и |
| Integration | Реальная тестовая БД (docker-compose.test.yml, порт 3307) | Жизненный цикл игры, таски |

**Запуск unit-тестов:**
```
make test              # go test ./... -count=1 -race
```

**Запуск интеграционных тестов:**
```
make test-integration  # поднимает mysql_test, запускает _test/integration/..., гасит
```

Интеграционные тесты используют build tag `//go:build integration` — они исключены из `make test` и запускаются только через `make test-integration`.

### Таблица тест-кейсов

| TC | Тип | Описание |
|----|-----|----------|
| TestCreateAndJoin | Integration | Создание игры + присоединение 3 игроков, проверка БД |
| TestStartGame | Integration | Запуск игры: status=active, появляется в GetAllActive |
| TestAnswerTask | Integration | Публикация ответа на таску, CountAnsweredByTask=1 |
| TestSkipTask | Integration | 3 пропуска, skip_count=3 в БД |
| TestFinalizeText | Integration | Создание task_result с type=text, проверка GetByTask |

`task_result` создаётся в `completeAdminTask` (поле `image_generated: true`), а не в `OpenAICollageFinalizer.Finalize`. Финализатор только верифицирует его наличие, что защищает от ситуации, когда scheduler пытается финализировать до того, как админ ответил.
