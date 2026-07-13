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


## Task #4 [DONE]
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


## Task #5 [DONE]
**Description:** В таске 12 в конце, когда подводятся итоги, генерируется коллаж через API Open AI. Иногда могут возникать проблемы именно с генерацией коллажа. В таком случае ошибки отображаются в логах и уже немного погуглив я могу дойти до возможной проблемы. Наверное, нужно как-то сообщать игрокам о том, что возникла проблема с генерацией коллажа и его не будет. И как в ходе работы бота понять какая именно ошибка и в чём именно? тут ещё нужно видеть ответы юзеров, которые потом добавляются в промпт. В боте есть техподдержка, обычный телеграмм аккаует пользователя, с которым можно связаться.
Подумай как можно реализовать обработку ошибок и понятие в чем именно проблема возникла и добавь в какое-то поле в этой таске. Если возможно опиши несколько вариантов и их плюсы и минусы

**Variants:**

### Контекст
Генерация коллажа происходит в `internal/usecase/task/subtask/admin_only.go:268` (`completeAdminTask` → `h.openai.GenerateCollage(ctx, prompt)`). Сейчас при ошибке функция возвращает обёрнутый `err` наверх и пишет ERROR-лог без структурированных полей. Игроки в чате ничего не видят — бот молча зависает после `task.Summary.SendingText` ("генерую коллаж..."). Финализатор `OpenAICollageFinalizer` при отсутствии `task_result` сам падает с ошибкой и игра не завершается.

### Какие ошибки могут возникать
- **OpenAI API** (`*openai.APIError` из `sashabaranov/go-openai` содержит `HTTPStatusCode`, `Type`, `Code`, `Message`):
  - `400 invalid_request_error` — некорректный промпт (слишком длинный, запрещённые символы)
  - `400 content_policy_violation` — модерация отклонила (нецензурные ответы юзеров)
  - `401 invalid_api_key` — конфигурация
  - `429 rate_limit_exceeded` / `insufficient_quota` — квоты исчерпаны
  - `500/502/503 server_error` — временный сбой OpenAI
- **Сетевые ошибки**: `context.DeadlineExceeded`, `context.Canceled`, `net.Error` (timeout, dns failure)
- **Декодирование**: невалидный base64 в ответе (редко)
- **Telegram-сторона**: отправка фото может упасть отдельно (файл слишком большой, чат недоступен)

---

### Вариант A (минимальный): одно общее сообщение игрокам + структурированный лог
**Что делаем:**
1. В `completeAdminTask` при ошибке `GenerateCollage`:
   - Отправляем в чат одно сообщение типа: `"😔 На жаль, не вдалося згенерувати колаж. Зверніться до техпідтримки: <a href='{{SupportURL}}'>...</a>"` (новый ключ в `messages.yaml`, например `task12_collage_error`).
   - Логируем ERROR со всеми полями: `game.ID`, `chat`, `task.ID`, `prompt` (полный), `answers` (map целиком), `error.Error()`.
2. В `OpenAICollageFinalizer.Finalize`: если `task_result` отсутствует — всё равно вызвать `finishGame` (или новый `finishGameWithError`), чтобы статусом игры завершить и не блокировать поток.

**Плюсы:**
- Минимальные изменения: ~30 строк, без новых интеграций.
- Игроки получают понятную обратную связь.
- Лог содержит всё для диагностики (прометей-стиль через zerolog → `grep` по `game=` или `task=`).

**Минусы:**
- Разработчик узнаёт об ошибке только если **сам** мониторит логи. При проде это значит "когда юзер напишет в поддержку".
- Один шаблон сообщения для всех типов ошибок — игроку непонятно, временная проблема или фундаментальная.
- `prompt` в логе может быть длинным (несколько сотен символов) — лог-файл раздувается.

---

### Вариант B: категоризация ошибок + специфичные сообщения для игроков + структурированный лог
**Что делаем:**
1. Создаём хелпер `internal/infrastructure/openai/errors.go` с функцией `Classify(err error) ErrorCategory`:
   - `CategoryContentPolicy` — модерация
   - `CategoryRateLimit` — квоты/throttling (можно повторить позже)
   - `CategoryConfig` — auth/api_key (показать "проблема на нашей стороне")
   - `CategoryNetwork` — timeout/canceled (можно повторить)
   - `CategoryServer` — 5xx (временно)
   - `CategoryUnknown` — fallback
   Через `errors.As(&apiErr)` и проверку `apiErr.HTTPStatusCode` + `apiErr.Code`.
2. В `messages.yaml` — отдельные ключи (`task12_collage_error_content_policy`, `task12_collage_error_rate_limit`, `task12_collage_error_generic`). Все с упоминанием поддержки.
3. Логировать с полем `category` — упрощает фильтрацию.

**Плюсы:**
- Игроки получают релевантный месседж: при `content_policy` — "ответы не прошли модерацию", при `rate_limit` — "сервис перегружен, ми поки розбираємось".
- В логах появляется `category=content_policy` → быстро видно паттерны.
- Заодно открывает возможность для retry в варианте D.

**Минусы:**
- Больше кода (~80–100 строк).
- OpenAI иногда возвращает ошибку без чёткого `Code` — придётся fallback на `Unknown`.
- Нужно обновлять categorizer при обновлении модели/SDK.

---

### Вариант C: уведомление поддержки в DM + общее сообщение игрокам
**Что делаем:**
1. Расширяем конфиг: добавляем `SUPPORT_TELEGRAM_CHAT_ID` (числовой ID, не URL). Поддержка должна один раз написать боту в личку, чтобы бот получил право слать сообщения.
2. В `completeAdminTask` при ошибке:
   - В чат игры: общий месседж со ссылкой на поддержку (как в варианте A).
   - В DM поддержки: подробный отчёт:
     ```
     ❌ Помилка task_12 collage
     Game: {{game.ID}} | Chat: {{chat.Name}} ({{chat.ID}})
     Admin: @{{admin.username}} ({{admin.tg_id}})
     Error: {{err.Type}} {{err.Code}} — {{err.Message}}
     Prompt (rendered):
     <pre>{{prompt}}</pre>
     Answers:
     <pre>{{answers as JSON}}</pre>
     ```
     отправляется как HTML-сообщение в чат поддержки.

**Плюсы:**
- Разработчик узнаёт о проблеме **в момент возникновения**, с полным контекстом (промпт, ответы, чат). Не нужно лезть в логи и грепать.
- Можно отвечать на инциденты быстро: связаться с админом игры, попросить переформулировать или вручную перегенерировать.
- Заодно — лёгкий путь добавить /support-команду в DM для других ошибок.

**Минусы:**
- Требует, чтобы поддержка предварительно «активировала» бота в DM (один раз) — это операционный шаг.
- Если бот словит rate limit на отправку в Telegram (после нескольких подряд ошибок) — отчёт может не дойти. Нужно дублировать в лог как safety net.
- Чувствительные данные (имена игроков, их ответы) попадают в DM конкретного человека — комплаенс-нюанс.

---

### Вариант D (рекомендуемый): A + B + C + retry с backoff
**Что делаем:**
1. **Retry-обёртка** вокруг `GenerateCollage` для категорий `RateLimit`, `Network`, `Server`: 2 повторных попытки с экспоненциальным backoff (например, 5s, 15s). Для `ContentPolicy` и `Config` — без повторов.
2. **Категоризация** (Вариант B) — определяет сообщение для игроков и поведение retry.
3. **Сообщение в чат** (Вариант A) — через `messages.yaml`, шаблон зависит от категории.
4. **DM поддержки** (Вариант C) — финальный отчёт, если все retry провалились, или для категорий `ContentPolicy`/`Config` сразу.
5. **Структурированный лог** на каждом шаге: `attempt=1/3`, `category=...`, `wait=15s`, `final_error=...`.
6. **Финализация игры**: даже при провале коллажа `OpenAICollageFinalizer` должен пропустить проверку `task_result` (или создать `task_result` с `{"image_generated": false, "error": "..."}`), чтобы `finishGame` отработал и игра не висела в `active` навечно.

**Плюсы:**
- Самый устойчивый: половина ошибок (rate limit, timeout, 5xx) скроется за retry и игроки даже не заметят.
- Разработчик получает контекстный DM-отчёт только когда ошибка реально требует внимания.
- Игроки получают релевантное сообщение, игра завершается чисто.

**Минусы:**
- Самый объёмный: ~200–250 строк кода + тесты на categorizer и retry.
- Удлиняет максимальное время ожидания (5s + 15s = 20s доп. ожидания) — нужно сообщить игрокам "ще трохи, генеруємо..." промежуточным сообщением, иначе подумают что бот завис.
- DM поддержки усложняет деплой (нужен `SUPPORT_TELEGRAM_CHAT_ID`, инициализация при первом запуске).

---

### Сравнение

| Критерий | A | B | C | D |
|---|---|---|---|---|
| Игроки видят ошибку | Общее | Конкретное | Общее | Конкретное |
| Разработчик узнаёт сразу | Нет | Нет | Да | Да |
| Контекст для диагностики (промпт+ответы) | В логе | В логе | В DM | В DM + лог |
| Устойчивость к временным сбоям | Нет | Нет | Нет | Да (retry) |
| Объём кода | ~30 | ~100 | ~150 | ~250 |
| Доп. инфраструктура | — | — | DM-чат поддержки | DM-чат поддержки |

### Рекомендация
**Вариант D** для прода, но можно делать инкрементально: начать с **A** (быстрый win, игроки не остаются в темноте), затем добавить **B** (категоризация), затем **C** (DM) при первом серьёзном инциденте. Retry (часть D) — критичная часть для устойчивости, его стоит включить даже в первой итерации, потому что rate-limit и кратковременные 5xx у OpenAI встречаются регулярно.




## Task#6: изменить поведение сообщений-вопросов в таске 12 (`admin_only`) [DONE]

## 1. Контекст

Таска 12 (`admin_only`) — финальная задача игры. Отвечает только админ, последовательно, вопрос за вопросом. Логика живёт в `internal/usecase/task/subtask/admin_only.go`. Промежуточное состояние (ответы + `q_msg_id`) хранится в `subtask_progress.answers_data` как JSON:

```json
{ "answers": { "city": "...", "concert": "..." }, "q_msg_id": 12345 }
```

Каждый вопрос отправляется как сообщение с inline-клавиатурой (`Task12QuestionKeyboard`). Сейчас после ответа админа сообщение-вопрос **удаляется** по сохранённому `q_msg_id`, и в чате остаётся только следующий вопрос.

## 2. Что нужно изменить (кратко)

| | Было | Стало |
|---|---|---|
| Сообщение-вопрос после ответа админа | Удаляется (по `q_msg_id`) | **НЕ удаляется**, остаётся в чате вместе со своей клавиатурой |
| Повторное нажатие на кнопку уже отвеченного вопроса | Не обрабатывалось (сообщение уже было удалено) | Бот отправляет уведомление «ответ на этот вопрос уже есть» и **удаляет это уведомление через интервал из `.env`** |

**Больше ничего в таске 12 (и во всём проекте) менять нельзя.** Остальной flow — постановка вопросов, сохранение ответов, `task12_reply`, переход к следующему вопросу, `completeAdminTask`, генерация OpenAI-коллажа, финализация и завершение игры — остаётся строго как есть.

## 3. Детальное поведение

### 3.1. `HandleAnswer` — убрать удаление вопроса

В обработчике входящего текста админа (`HandleAnswer`, суффикс состояния `:admin`) сейчас есть шаг «удалить сообщение-вопрос по сохранённому `q_msg_id`». **Удалить только этот шаг.**

Оставить без изменений:
- загрузку `subtask_progress`;
- сохранение `answers[questionID] = msg.Text`;
- то, что ответ админа остаётся в чате;
- отправку `task12_reply`;
- переход к следующему вопросу (отправка нового вопроса + обновление `q_msg_id`);
- вызов `completeAdminTask`, когда все вопросы отвечены.

> `q_msg_id` продолжает записываться как раньше (он всё ещё указывает на текущий активный вопрос). Не удалять это поле и не менять его семантику — минимизируем диффы. Убирается **только** сам вызов удаления сообщения.

### 3.2. `HandleButtonPress` — guard на уже отвеченный вопрос

Так как сообщения-вопросы теперь остаются в чате, у каждого из них остаётся живая кнопка. Нужно, чтобы при нажатии на кнопку вопроса `HandleButtonPress` различал два случая:

1. **Вопрос ещё не отвечен** (это текущий активный вопрос) → поведение как сейчас: отправить `task12_awaiting_answer`, состояние игрока не менять (оно уже `awaiting_answer`).
2. **Вопрос уже отвечен** (его `questionID` присутствует в `progress.answers`) → отправить сообщение `task12_already_answered` в чат и **запланировать его автоудаление** через новый интервал (см. §4). При этом:
   - НЕ менять `subtask_progress`;
   - НЕ менять состояние игрока (`player_states`);
   - НЕ трогать существующие ответы;
   - вызвать `c.Respond()` (убрать «часики» на callback-кнопке).

Алгоритм `HandleButtonPress` (новый):

```
HandleButtonPress:
    questionID := распарсить из callback payload   // см. §5 — критично
    progress   := subtaskProgressRepo.Get(...)     // загрузить прогресс
    answers    := progress.Answers

    if _, ok := answers[questionID]; ok {
        // вопрос уже отвечен
        msg := cfg.Messages.Task12AlreadyAnswered   // рандом из массива вариантов
        sent := bot.Send(chat, msg, formatter.ParseMode)
        go scheduleDelete(sent, cfg.Timings.Task12AlreadyAnsweredDeleteDelay)
        c.Respond()
        return
    }

    // вопрос ещё не отвечен — прежнее поведение
    bot.Send(chat, cfg.Messages.Task12AwaitingAnswer, formatter.ParseMode)
    c.Respond()
    return
```

`scheduleDelete` — переиспользовать уже существующий в проекте паттерн автоудаления временных сообщений (тот же, что применяется для `task12_only_admin` / `DeleteMessageDelay`): горутина со `time.Sleep(delay)` → `bot.Delete(msg)`, с логированием ошибки удаления, но без паники. **Не** вводить новый механизм, если такой хелпер уже есть — найти и переиспользовать.

## 4. Конфигурация: новый интервал

Значение времени автоудаления сообщения «ответ уже есть» задаётся отдельной переменной окружения (требование заказчика).

### 4.1. `internal/config/timings.go`
Добавить поле в структуру `Timings`:

```go
// Через сколько удалять уведомление "ответ на вопрос уже есть" (таска 12)
Task12AlreadyAnsweredDeleteDelay time.Duration
```

Загрузить его в `loadTimings` по правилам проекта: prod-значение из `TASK12_ALREADY_ANSWERED_DELETE_DELAY`, а при `TEST_MODE=true` — из `TEST_TASK12_ALREADY_ANSWERED_DELETE_DELAY` (как это сделано для остальных интервалов).

### 4.2. `.env` и `.env.example`
Добавить:

```env
TASK12_ALREADY_ANSWERED_DELETE_DELAY=10s
# при TEST_MODE=true берётся отсюда:
TEST_TASK12_ALREADY_ANSWERED_DELETE_DELAY=5s
```

> Значения `10s` / `5s` — разумный дефолт по аналогии с `DELETE_MESSAGE_DELAY`. Финальные значения владелец проекта может поправить в `.env`.

> Не переиспользовать `DELETE_MESSAGE_DELAY` для этого сценария — заказчик явно попросил отдельную переменную окружения, чтобы интервал настраивался независимо.

## 5. Callback: пробросить `questionID` в payload (проверить обязательно!)

Чтобы `HandleButtonPress` знал, какой именно вопрос нажат, callback кнопки вопроса (`\ftask12:question`) **должен нести `questionID` в payload**.

Claude Code обязан сначала проверить текущую реализацию `Task12QuestionKeyboard` в `internal/delivery/bot/keyboard/factory.go` и роутинг callback в `internal/delivery/bot/handler/callback.go`:

- **Если payload уже содержит `questionID`** (`kbd.Data(label, "task12:question", questionID)`) — просто использовать его в `HandleButtonPress`, ничего в фабрике/роутинге не меняя.
- **Если payload пустой или не содержит `questionID`** — доработать:
  - `Task12QuestionKeyboard(questionID string)` должна класть `questionID` в payload (`kbd.Data(label, "task12:question", questionID)`);
  - обновить точки вызова фабрики (в `admin_only.go`, где отправляется вопрос) так, чтобы передавать `questionID` текущего вопроса;
  - в хендлере callback распарсить `questionID` из `c.Data()` и передать в `HandleButtonPress`.

Это единственная допустимая правка вне `admin_only.go` / конфигов / текстов — и только если она реально требуется. Никакой другой логики роутинга не менять.

## 6. Тексты: `content/messages.yaml`

Добавить новый ключ с вариативным массивом (правило проекта: публичные сообщения берутся рандомно из массива), HTML-разметка Telegram:

```yaml
task12_already_answered:
  - "На це питання ти вже відповів ✅"
  - "Відповідь на це питання вже збережена ✅"
```

Подключить ключ в структуру `config.Messages` (`Task12AlreadyAnswered []string`) и в загрузчик, по образцу существующих сообщений таски 12 (`task12_only_admin`, `task12_awaiting_answer`, `task12_reply`).

> Текст на украинском — как и остальные пользовательские сообщения в проекте. При необходимости владелец скорректирует формулировки.

## 7. Обязательные правила из CLAUDE.md (соблюдать строго)

1. **Тексты — только в YAML.** Никаких строк-сообщений в Go. Использовать `cfg.Messages.Task12AlreadyAnswered`.
2. **Форматирование — только HTML через `pkg/formatter`.** ParseMode всегда `tele.ModeHTML`.
3. **Тайминги — только в `timings.go`.** Никаких `time.Sleep(10 * time.Second)` — использовать `cfg.Timings.Task12AlreadyAnsweredDeleteDelay`.
4. **Архитектурные границы:** правки только в `usecase/task/subtask/admin_only.go`, `config/*`, `content/messages.yaml`, `.env(.example)` и (при необходимости, см. §5) в `keyboard/factory.go` + `handler/callback.go`. `usecase` не импортирует `delivery`/`infrastructure` напрямую — только интерфейсы.
5. **Ошибки** оборачивать с контекстом: `fmt.Errorf("subtask/admin_only.HandleButtonPress: %w", err)`.
6. **Логирование** — только `pkg/logger`, с полями `chat` и `user`.

## 8. Крайние случаи

- Все вопросы отвечены (после `completeAdminTask`): все сообщения-вопросы остаются в чате с живыми кнопками. Нажатие на любую → `task12_already_answered` с автоудалением. Корректно (все `questionID` уже в `answers`).
- Быстрые повторные нажатия на кнопку уже отвеченного вопроса: каждое нажатие шлёт своё уведомление и планирует своё автоудаление независимо. Дедупликация не требуется.
- Нажатие на кнопку **текущего** (ещё не отвеченного) вопроса — прежнее поведение (`task12_awaiting_answer`), состояние игрока не меняется.
- Не-админ: guard `player.TelegramUserID == game.AdminUserID` и сообщение `task12_only_admin` — остаются без изменений и срабатывают раньше проверки на «уже отвечено».

## 9. Тесты

Обновить/добавить unit-тесты (`admin_only_test.go`, gomock) по правилам проекта:

- **Изменить** тест на `HandleAnswer`: убедиться, что удаления сообщения-вопроса больше **не происходит** (mockSender не должен получать `Delete` для `q_msg_id`), при этом ответ сохраняется, `task12_reply` отправляется, следующий вопрос публикуется.
- **Добавить** тест на `HandleButtonPress`, случай «вопрос уже отвечен»: `questionID` есть в `progress.answers` → отправляется `task12_already_answered`, планируется удаление, `subtask_progress` и `player_states` не изменяются.
- **Добавить/сохранить** тест на `HandleButtonPress`, случай «вопрос ещё не отвечен»: отправляется `task12_awaiting_answer`, состояние не меняется.
- Если менялась фабрика/роутинг (§5) — покрыть парсинг `questionID` из callback payload.

Запуск: `make test`. Линт: `make lint`.

## 10. Критерии приёмки

1. После ответа админа сообщение-вопрос остаётся в чате вместе с клавиатурой (не удаляется).
2. Повторное нажатие на кнопку уже отвеченного вопроса → в чат приходит уведомление, которое автоматически удаляется через `TASK12_ALREADY_ANSWERED_DELETE_DELAY` (в тест-режиме — через `TEST_TASK12_ALREADY_ANSWERED_DELETE_DELAY`).
3. Нажатие на кнопку текущего (неотвеченного) вопроса работает как раньше.
4. Ни один другой сценарий таски 12 и остального проекта не изменился (проверяется существующими тестами — они должны остаться зелёными).
5. Соблюдены все правила CLAUDE.md (тексты в YAML, HTML через formatter, тайминги в timings.go, границы слоёв, обёртка ошибок, логгер).
6. `make test` и `make lint` проходят.

## 11. Ручная проверка (TEST_MODE=true)

1. `make docker-up`, добавить бота в тестовый чат, `Розпочати гру`.
2. `/test_task_12` — опубликовать таску 12.
3. Как админ: «Хочу відповісти» → ответить на первый вопрос.
4. Убедиться, что сообщение первого вопроса **осталось** в чате.
5. Нажать кнопку **первого** (уже отвеченного) вопроса → появляется уведомление «ответ уже есть», которое исчезает через `DELETE_MESSAGE_DELAY` (в тест-режиме — `TEST_MODE`-эквивалент этой же переменной).
6. Ответить на все вопросы → убедиться, что генерация коллажа и финал игры отрабатывают как прежде.

**Solution:**
Реализовано в `internal/usecase/task/subtask/admin_only.go`:
1. `HandleAnswer` — убран шаг удаления сообщения-вопроса по `pd.QuestionMsgID`. Само поле `q_msg_id` по-прежнему сохраняется в прогрессе без изменений.
2. `HandleButtonPress` — теперь использует `ctx` и `player` (ранее игнорировались), загружает `subtask_progress` и проверяет, есть ли `questionID` среди уже сохранённых ответов:
   - Если да — отправляет случайный текст из `Task12AlreadyAnswered` и планирует автоудаление.
   - Если нет (включая случай, когда прогресса ещё нет) — прежнее поведение: `Task12AwaitingAnswer`.
   Ни `subtask_progress`, ни `player_states` в ветке "уже отвечено" не изменяются.
3. Callback `task12_question` уже передавал `questionID` в payload (`kbd.Data(q.ButtonLabel, "task12_question", q.ID)` в `buildTask12QuestionKeyboard`), роутинг в `callback.go` (`OnTask12Question`) уже пробрасывал `c.Data()` в `HandleButtonPress` — правки в `keyboard/factory.go` и `handler/callback.go` не потребовались.

**Отличие от исходного плана задачи:** отдельная переменная окружения `TASK12_ALREADY_ANSWERED_DELETE_DELAY` не добавлялась — по явному указанию владельца проекта автоудаление уведомления "ответ уже есть" использует уже существующий `cfg.Timings.DeleteMessageDelay`, без новых переменных в `.env`/`.env.example`.

Новый текстовый ключ `task12_already_answered` добавлен в `content/messages.yaml` и в `config.Messages.Task12AlreadyAnswered`.

Тесты (`admin_only_test.go`) обновлены/добавлены:
- `TestAdminHandleAnswer_IntermediateQuestion_SendsNext` / `..._LastQuestion_...` — проверяют, что вопрос больше не удаляется.
- `TestAdminHandleButtonPress_NotAnsweredYet_SendsAwaitingAnswer`, `..._NoProgressYet_...`, `..._AlreadyAnswered_SendsNotice` — покрывают обе ветки `HandleButtonPress` и неизменность `subtask_progress`/`player_states` при уже отвеченном вопросе.

**Фикс после ревью:** т.к. сообщение-вопрос больше не удаляется, его inline-кнопка остаётся видимой всем участникам чата, а не только админу. `HandleButtonPress` изначально не проверял `player.TelegramUserID == game.AdminUserID` (этой проверки не было и до Task#6, но раньше она была не нужна — вопрос быстро удалялся). Добавлена проверка в начале `HandleButtonPress`: не-админ получает `task12_only_admin` (с автоудалением), как и в `HandleRequestAnswer`; только после этой проверки идёт логика "уже отвечено / ещё не отвечено". Добавлен тест `TestAdminHandleButtonPress_NonAdmin_SendsDismissal`.

`go build ./...`, `go vet ./...` и `go test ./...` проходят.