## Task #1 [DONE]
**Descriotion:** При запуске проекта через Docker compose создаётся База Данных(далее - БД). Если контейнеры остановить или очистить(посмотри на команды в @Dockerfilae) и перезапустить контейнеры, то старая БД остаётся. Сделай так чтобы при запуске проекта при параметре TEST_MODE=true из файла .env рри перезапуске контейнеров БД удалялась и создавалась новая и пустая. После реализации решения допиши в Task #1 как можно проверить что база данных пустая(то есть при перезапуске БД пересоздалась). Опиши поле Solution и внеси изменение в файл документации, если нужно  @docs/architecture.md

**Solution:**
Изменён таргет `docker-restart` в `Makefile`. При `TEST_MODE=true` (из `.env`) команда:
1. Останавливает и удаляет контейнеры и named volumes (`docker compose down -v`)
2. Проверяет переменную `TEST_MODE`: если `true` — удаляет директорию `$(DB_PATH)` (bind mount с данными MySQL) и создаёт пустую
3. Пересобирает и поднимает контейнеры; сервис `migrate` автоматически применяет миграции, создавая пустые таблицы

**Как проверить, что БД пустая (пересоздалась):**
1. Убедись, что в `.env` стоит `TEST_MODE=true`
2. Выполни `make docker-restart`
3. Дождись запуска контейнеров (`docker compose ps` — все сервисы `Up`)
4. Выполни проверку количества строк в таблице игр:
   ```
   docker exec bestieverse-mysql-1 mysql -u${DB_USER} -p${DB_PASSWORD} ${DB_NAME} -e "SELECT COUNT(*) as games_count FROM games;"
   ```
   Результат должен быть `0`.
5. Либо подключись напрямую и выполни `SHOW TABLES;` — таблицы будут созданы миграциями, но все пустые.


## Task #2 [DONE]
**Description:** Во время логирования, когда выводиться информациия о юзере, я хочу видеть в логах информацию в видее user=(user_telegram_id | telegram_username). Если это по каким-то причинам не возможно, тогда хочу видеть в логах информацию в виде user_id=user_telegram_id username=telegram_username. Если оба варианта возвожмны, кратко проанализуй как лучше будет вариант, опиши мне плюсы и миунс каждого из них и аргументируй их. Дождись от меня ответа какой вариант я выберу и после ответа приступай к реализации

**Solution:**
Выбран Вариант A. Изменена функция `WithUser` в `pkg/logger/logger.go`:
- Новый формат: `user=(987654321|@username)`
- Если `username` пустой: `user=(987654321)`
Обновлены тесты в `pkg/logger/logger_test.go`, добавлен тест `TestWithUser_EmptyUsername`.
Обновлена документация в `docs/architecture.md`.


## Task #3 [DONE]
**Description:** Во время логирования, когда выводиться информациия о чате, я хочу видеть в логах информацию в видее chat=( chat_telegram_id | chat_name ).

**Solution:**
Изменена функция `WithChat` в `pkg/logger/logger.go` — добавлен второй параметр `chatName string`:
- Новый формат: `chat=(123456789|Group Name)`
- Если `chatName` пустой: `chat=(123456789)`
Добавлена вспомогательная функция `ChatValue(chatID int64, chatName string) string` аналогично уже существующей `UserValue`.
Обновлены тесты в `pkg/logger/logger_test.go`:
- `TestWithChat_AddsFieldToOutput` — проверяет формат с именем чата
- `TestWithChat_EmptyChatName` — проверяет формат без имени
Также исправлен некорректный тест `TestWithUser_EmptyUsername` из Task #2 (ожидал `"( 987654321 | )"` вместо `"(987654321)"`).
Обновлена документация в `docs/architecture.md`.


## Task #4
**Description:** Во время логирования я хочу видеть в логах консоли больше информации, каждое действие юзера, его состояние, состояние игры, текущую таску и так далее. Продумай какие параметры и при каких действиях можно логировать для подробного мониторинга и напиши это в виде плана в поле Plan Task #4 в файле @docs/feature_tasks
**Plan:**

### Анализ текущего состояния

**Уже логируется (INFO):**
- `game created` (chat, admin)
- `player joined` / `player left` (chat, user)
- `game started` / `game finished` (chat, game)
- `task published` (chat, game, task)
- `awaiting answer` (chat, user, task)
- `task answered` (chat, game, user, task)
- `task skipped` (chat, user, task, skip_count)
- `task finalized: no answers` (chat, game, task)
- `reminder sent` (chat, game, user)
- Ошибки во всех слоях (ERROR/WARN)

**Пробелы — что нужно добавить:**

---

### 1. Финализация тасок — добавить INFO-лог успешной финализации

**Файл:** `internal/usecase/task/finalize/router.go`

После успешного вызова `finalizer.Finalize(...)`:
```
INFO  task finalized   chat=(ID|Name) game=N task=task_NN summary_type=text
```
Поля: `chat`, `game` (uint64), `task` (string), `summary_type` (string)

Мотивация: сейчас лог `task finalized` есть только для случая "нет ответов" — успешная финализация не логируется совсем.

---

### 2. Сабтаска voting_collage — добавить INFO-логи нормального flow

**Файл:** `internal/usecase/task/subtask/voting_collage.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Лок захвачен | INFO | `voting_collage: lock acquired` | chat, user, task |
| Лок занят | INFO | `voting_collage: lock busy` | chat, user, task |
| Категория выбрана | INFO | `voting_collage: category chosen` | chat, user, task, category, option, progress (напр. `"2/3"`) |
| Сабтаска завершена | INFO | `voting_collage: completed` | chat, user, task |

---

### 3. Сабтаска who_is_who — добавить INFO-логи нормального flow

**Файл:** `internal/usecase/task/subtask/who_is_who.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Лок захвачен | INFO | `who_is_who: lock acquired` | chat, user, task |
| Лок занят | INFO | `who_is_who: lock busy` | chat, user, task |
| Ответ на вопрос записан | INFO | `who_is_who: answer recorded` | chat, user, task, question, chosen_user (telegram_id) |
| Сабтаска завершена | INFO | `who_is_who: completed` | chat, user, task |

---

### 4. Сабтаска meme_voiceover — добавить INFO-логи нормального flow

**Файл:** `internal/usecase/task/subtask/meme_voiceover.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Лок захвачен | INFO | `meme_voiceover: lock acquired` | chat, user, task |
| Лок занят | INFO | `meme_voiceover: lock busy` | chat, user, task |
| Мем озвучен (промежуточный) | INFO | `meme_voiceover: meme answered` | chat, user, task, meme_index (напр. `"2/5"`) |
| Сабтаска завершена | INFO | `meme_voiceover: completed` | chat, user, task |

---

### 5. Сабтаска admin_only — добавить INFO-логи нормального flow

**Файл:** `internal/usecase/task/subtask/admin_only.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Сабтаска начата (запрос ответа) | INFO | `admin_only: started` | chat, user, task |
| Ответ на вопрос записан | INFO | `admin_only: answer recorded` | chat, user, task, question |
| Запуск генерации OpenAI коллажа | INFO | `admin_only: generating collage` | chat, game, task |
| Сабтаска полностью завершена | INFO | `admin_only: completed` | chat, user, task |

---

### 6. Scheduler — добавить DEBUG-логи тиков и состояния игр

**Файл:** `cmd/scheduler/main.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Начало тика планировщика | DEBUG | `scheduler: tick` | games_count (int) |
| Игра ожидает следующего события | DEBUG | `scheduler: game idle` | game (uint64), next_finalize_in (duration), next_publish_in (duration) |

Мотивация: сейчас невозможно понять, обрабатывает ли планировщик игры или тихо молчит потому что условия не выполнены.

---

### 7. Нотификатор — добавить INFO-лог о количестве игроков для напоминания

**Файл:** `internal/usecase/notification/send_reminder.go`

| Событие | Уровень | Сообщение | Поля |
|---|---|---|---|
| Найдены игроки без ответа | INFO | `reminder: players pending` | chat, game, task, count (int) |

Мотивация: сейчас видно каждое отдельное напоминание, но не видно общего числа — при 0 игроках нотификатор тихо пропускает игру без какого-либо лога.

---

### Итоговая таблица изменений

| Файл | Новых логов | Уровень |
|---|---|---|
| `finalize/router.go` | 1 | INFO |
| `subtask/voting_collage.go` | 4 | INFO |
| `subtask/who_is_who.go` | 4 | INFO |
| `subtask/meme_voiceover.go` | 4 | INFO |
| `subtask/admin_only.go` | 4 | INFO |
| `cmd/scheduler/main.go` | 2 | DEBUG |
| `notification/send_reminder.go` | 1 | INFO |

**Итого: 20 новых записей лога** покрывают все пробелы в мониторинге нормального flow.