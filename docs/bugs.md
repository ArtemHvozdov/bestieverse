# Bugs

## Bug #1 — Callback кнопки не работают [FIXED]

**Симптом:** При нажатии на любую inline-кнопку с callback data (кроме URL-кнопок) в Telegram появляется спиннер «Загрузка...», который через несколько секунд исчезает. Ничего не происходит, в логах нет ни одной записи.

**Причина:** Telebot v3 парсит поле `callback_data` входящего callback query регулярным выражением:

```
^\f([-\w]+)(\|(.+))?$
```

Символьный класс `[-\w]+` включает только буквы, цифры, символы `_` и `-`. Символ `:` в него не входит. Все уникальные идентификаторы кнопок в проекте использовали двоеточие как разделитель (`game:join`, `game:leave`, `task:request`, `task02:choice` и т.д.). Из-за этого regex не давал совпадения, telebot не находил зарегистрированный хэндлер и передавал управление в `OnCallback` — который не был зарегистрирован. Обработчики не вызывались, ошибки не логировались.

Баг затрагивал **все** data-кнопки в проекте без исключения.

**Исправление:** Заменено двоеточие на нижнее подчёркивание во всех unique-именах кнопок и соответствующих регистрациях хэндлеров. Изменены файлы:

| Файл | Что изменено |
|---|---|
| `internal/delivery/bot/keyboard/factory.go` | Все фабричные функции: `game:join` → `game_join`, `game:leave` → `game_leave`, и т.д. |
| `cmd/bot/main.go` | Все `bot.Handle("\f...")` регистрации |
| `internal/usecase/task/publish.go` | `task:request` → `task_request`, `task:skip` → `task_skip` |
| `internal/usecase/task/subtask/poll.go` | `task:request`, `task:skip`, `task10:meme_request` |
| `internal/usecase/task/subtask/voting_collage.go` | `task02:choice` → `task02_choice` |
| `internal/usecase/task/subtask/who_is_who.go` | `task04:player` → `task04_player` |
| `internal/usecase/task/subtask/admin_only.go` | `task12:question` → `task12_question` |

**Правило на будущее:** В telebot v3 unique-имя кнопки (`kbd.Data(label, unique, ...)`) может содержать только символы `[a-zA-Z0-9_-]`. Двоеточие и другие спецсимволы использовать нельзя.


## Bug #2 [FIXED]
**Симптом:** Когда бота добавили в чат и он отправил сообщение с кнопками "Приєднатися до гри", "Техпідтримка", "Вийти з гри" и сообщение для с кнопкой для старти игры, которую должен нажимать только админ, если обычный юзер (не админ) ещё не присоединиться к игре и нажмёт на кнопку "Почати гру", бот отправит сообщение о том, что этот юзер ещё в игре и сначала ему нужно присоединиться к игре в начале. В таком сценарии бот должен отправить сообщение о том, что начинать игру может только админ.
Когда обычный юзер уже присоединился к игре и нажимает на кнопу Почати гру, бот отправляет сообщение о том, что начинать игру может только админ.

**Причина:** Маршрут `game_start` использовал тот же middleware `PlayerCheck`, что и все остальные кнопки. Если пользователь ещё не вступил в игру, middleware перехватывал запрос и отправлял «ти ще не в грі» до того, как обработчик `Starter.Start` мог проверить, является ли он админом.

**Исправление:** Добавлен новый middleware `PlayerCheckForStart` в `internal/delivery/bot/middleware/player_check.go`. В отличие от `PlayerCheck`, он никогда не блокирует запрос на "не в игре". Если отправитель не найден среди игроков, в контекст помещается минимальный `entity.Player` с `TelegramUserID`, `Username`, `FirstName` из сообщения — этого достаточно, чтобы `Starter.Start` выполнил проверку на админа и отправил корректное сообщение `StartOnlyAdmin`. В `cmd/bot/main.go` маршрут `\fgame_start` переключён на `pcStart` вместо `pc`.


## Bug #3 [FIXED]
**Симптом:** Когда бот отправляет сообщение и тегает какого-то юзера в чате. юзернейм используется с нижним подчеркиванием. Нижнего подчеркивания не должно быть, просто @username.

**Причина:** Функция `Mention` в `pkg/formatter/telegram.go` всегда оборачивала имя пользователя в HTML-тег `<a href="tg://user?id=...">@username</a>`. Telegram рендерит `<a>` с эффектом подчёркивания (underline), что визуально даёт лишнее «нижнее подчёркивание» под текстом.

**Исправление:** Для пользователей с username функция теперь возвращает просто `@username` (plain text). Telegram автоматически делает `@username` кликабельным меншеном во всех режимах разбора, включая HTML. Для пользователей без username (анонимных) сохранена HTML-ссылка `<a href="tg://user?id=...">FirstName</a>`, т.к. без username другого способа создать кликабельный таг нет. Обновлён тест `TestMention_WithUsername`.


## Bug #4 [FIXED]
**Симптом:** После старта игры, когда публикуются Приветственные сообщения и первая таска, не отображаются медиафайлы корректно. Просто иконка файла, надпись file и размер. Можно файл загрузить, открыть локально файл, но отображается не гифка, а какой-то код. Скорее всего такая проблема будет с публикацией всех медифайлов.

**Причина:** В telebot v3 метод `Animation.MediaFile()` копирует `a.FileName` → `a.File.fileName`. Это поле используется как имя файла в заголовке `Content-Disposition` multipart-запроса. Telebot сам оставил комментарий в исходниках: *"file_name is required, without it animation sends as a document"*. В нашем `LocalStorage.GetAnimation` поле `FileName` не устанавливалось — `Content-Disposition` получал пустое имя файла. Telegram не мог определить тип файла по расширению и сохранял загрузку как Document (иконка файла, надпись "file", размер).

**Исправление:** В `internal/infrastructure/media/local.go` в методах `GetAnimation` и `GetFile` добавлено явное задание `FileName: filepath.Base(path)`. Теперь multipart-запрос содержит правильный заголовок `Content-Disposition: form-data; name="animation"; filename="task_01.gif"`, Telegram корректно определяет тип и отображает GIF как встроенную анимацию.


## Bug #5 [FIXED]
**Симптом:** Когда игра началась и опубликована первая таска, юзер отвечает на неё и, когда бот отправлявет сообщение о том, что ответ на таску приняти, бот не тегает юзера. Сообщение выглядит вот так - {{.Mention}} дякую! Твою відповідь на завдання #1 прийнято ✅. Вместо {{.Mention}} должен быть указан @username

**Причина:** В `answer.go` сообщение `msgs.AnswerAccepted` отправлялось напрямую через `sender.Send` без прогона через `formatter.RenderTemplate`. Go-шаблон `{{.Mention}}` оставался нерендеренным и появлялся в чате как литеральный текст. Та же проблема присутствовала во всех сообщениях `skip.go` (`AlreadyAnswered`, `AlreadySkipped`, `SkipNoRemaining`, `SkipWithRemaining2`, `SkipWithRemaining1`, `SkipLast`) — они тоже содержат `{{.Mention}}` и тоже отправлялись без рендеринга.

**Исправление:** В `answer.go` перед отправкой теперь строится mention (`formatter.Mention(...)`) и вызывается `formatter.RenderTemplate(a.msgs.AnswerAccepted, struct{ Mention string }{...})`. В `skip.go` та же пара вызовов добавлена для всех шести сообщений со статусом пропуска.


## Bug #6 [FIXED]
**Симптом:** Когда юзер ответил на таску бот написал, что ответ принят, если этот же юзер нажмёт повторно кнопку Хочу відповісти, то ничего не происходит, вверху окна телеграмма надпись "Загрузка...", через несколько секунд она исчезает. Такое же поведение, если юзер нажал на кнопку "Пропустити", бот написал сообщение о пропуске и тегнул юзера, но при повторных нажатия на кнопки Хочу відпоісти или Пропустити также ничего не происодит и исчесзающая через несколько секунд надпись "Загрузка...".
Вот логи из консоли бота:

```
bot-1  | 2026-05-09 06:19:48 INF bot started
bot-1  | 2026-05-09 06:21:50 INF player joined chat=-1002617613395 user=6598439879 username=Jay_jayss
bot-1  | 2026-05-09 06:22:26 INF player left chat=-1002617613395 user=6598439879
bot-1  | 2026-05-09 06:56:21 INF game started chat=-1002617613395 game=1
bot-1  | 2026-05-09 06:56:21 INF task published chat=-1002617613395 game=1 task=task_01
bot-1  | 2026-05-09 06:57:23 INF awaiting answer chat=-1002617613395 task=task_01 user=385672319
bot-1  | 2026-05-09 06:57:32 INF task answered chat=-1002617613395 game=1 task=task_01 user=385672319
bot-1  | 2026/05/09 06:57:38 48015989 task.RequestAnswer: get response: mysql/task_response.GetByPlayerAndTask: sql: Scan error on column index 5, name "response_data": unsupported Scan, storing driver.Value type <nil> into type *json.RawMessage
bot-1  | 2026/05/09 06:57:46 48015990 task.Skip: get response: mysql/task_response.GetByPlayerAndTask: sql: Scan error on column index 5, name "response_data": unsupported Scan, storing driver.Value type <nil> into type *json.RawMessage
bot-1  | 2026-05-09 06:59:53 INF game created admin=green_delfin admin_id=385672319 chat=-5117034843
bot-1  | 2026-05-09 07:00:05 INF game started chat=-5117034843 game=2
bot-1  | 2026-05-09 07:00:06 INF task published chat=-5117034843 game=2 task=task_01
bot-1  | 2026-05-09 07:00:10 INF task skipped chat=-5117034843 skip_count=1 task=task_01 user=385672319
bot-1  | 2026/05/09 07:00:15 48016002 task.Skip: get response: mysql/task_response.GetByPlayerAndTask: sql: Scan error on column index 5, name "response_data": unsupported Scan, storing driver.Value type <nil> into type *json.RawMessage
bot-1  | 2026/05/09 07:00:16 48016003 task.RequestAnswer: get response: mysql/task_response.GetByPlayerAndTask: sql: Scan error on column index 5, name "response_data": unsupported Scan, storing driver.Value type <nil> into type *json.RawMessage
```

**Причина:** `question_answer`-таска сохраняет ответ без `ResponseData` (только статус), поэтому в БД колонка `response_data = NULL`. При повторном нажатии "Хочу відповісти" или "Пропустити" `GetByPlayerAndTask` пытается `Scan` это NULL напрямую в `json.RawMessage` (`[]byte`), но `database/sql` не умеет класть `nil` в непустой `[]byte` — выбрасывает ошибку. Ошибка возвращалась из usecase, callback не вызывал `c.Respond()`, Telegram показывал бесконечный спиннер. Та же проблема присутствовала в `subtask_progress.GetByPlayerAndTask` для поля `answers_data`.

**Исправление:** В `internal/infrastructure/mysql/repository/helpers.go` добавлен хелпер `scanNullJSON(sql.NullString) json.RawMessage`, который возвращает `nil` для NULL-значений. В `GetByPlayerAndTask` и `GetAllByTask` в `task_response.go`, а также в `Get` в `subtask_progress.go` — сканирование JSON-колонок переведено на `sql.NullString` с последующим вызовом `scanNullJSON`.

## Bug #7 [FIXED]
**Симптом:** Когда игра уже началась и опубликована первая таска, если юзер до старта игры не присоединился к игре, после этого он уже не может присоединиться. Юзер нажимает кнопки Хочу відповісти или Пропустити, бот отправляет сообщение в чат, о том что юзер ещё не в игре и ему нужно присоединиться к игре нажав на кнопку в начале игры. Юзер нажимает на кнопку Приеднатися до гри, вверху окна телеграмма появляется надпись "Загрузка...", через несколько секунд она исчезает и ничего не происходит

**Причина:** В `join.go` стояла проверка `game.Status != entity.GamePending` — при активной игре usecase молча возвращал `nil`, не отправляя никакого сообщения и не отвечая на callback. Telegram показывал бесконечный спиннер. При этом бот сам подсказывал юзеру нажать кнопку "Приєднатися" — UX-противоречие.

**Исправление:** Условие изменено на `game.Status == entity.GameFinished` — теперь присоединение разрешено и в статусе `pending`, и в статусе `active`. Только завершённая игра блокирует вступление. Юзер, нажавший join во время активной игры, получает стандартное приветственное сообщение и добавляется как игрок с состоянием `idle`.


## Bug #8 [FIXED]
**Симптом:** Первая таска, в игре 2 юзера, один ответил на таску, бот отправил сообщение о, том что ответ на таску принят. Уведомления для второго юзера о том, что от него нет ответа было отправлено через 2 минуты после публикации таски, хотя ожидалось через 1. Также не было подведения итогов таски. Следующая таска опубликовалась через 4 минуты, ожидалось через 3.
TEST_MODE=true
TEST_TASK_PUBLISH_INTERVAL=3m
TEST_TASK_FINALIZE_OFFSET=2m
TEST_REMINDER_DELAY=1m
TEST_POLL_DURATION=1m

**Причина:** Два независимых бага:

1. **Отсутствие идемпотентности в `FinalizeRouter`**: Scheduler вызывал `Finalize` на каждом тике после того, как `finalizeTime` наступал. Если `TaskFinalizeOffset` и `TaskPublishInterval` попадали в разные тики, task_01 финализировался дважды: первый вызов отправлял предсказания и создавал `task_result`, второй — отправлял дубликаты сообщений и падал с ошибкой дублирующего ключа (`UNIQUE KEY uq_result`). Ошибка логировалась, но scheduler продолжал работу и публиковал следующую таску.

2. **Смещение времени из-за фиксированного 1-минутного тикера**: Scheduler и Notifier тикали раз в минуту от старта процесса, а не от момента публикации таски. Если таска публиковалась через 15 секунд после старта сервиса, следующая проверка происходила через ~45 секунд после публикации. Это давало смещение до 60 секунд для всех событий (напоминание, финализация, публикация следующей таски).

**Исправление:**

1. В `FinalizeRouter.Finalize` добавлена идемпотентная проверка в начале метода: `taskResultRepo.GetByTask`. Если `task_result` уже существует — вызов возвращает `nil` без побочных эффектов. В структуру `FinalizeRouter` добавлен `taskResultRepo repository.TaskResultRepository`, конструктор обновлён. Обновлены все места вызова (`cmd/bot/main.go`, `cmd/scheduler/main.go`). Добавлен тест `TestRouter_AlreadyFinalized_Skips`.

2. Ticker scheduler уменьшен с 1 минуты до 15 секунд (максимальное смещение теперь ≤15 с вместо ≤60 с). Дополнительно scheduler выполняет первый `tick()` сразу при старте, до первого тика тикера — это позволяет отработать события, которые должны были наступить во время downtime. Ticker notifier также уменьшен до 15 секунд.

Изменённые файлы: `internal/usecase/task/finalize/router.go`, `internal/usecase/task/finalize/router_test.go`, `cmd/scheduler/main.go`, `cmd/notifier/main.go`, `cmd/bot/main.go`.


## Bug #9 [FIXED]
**Симптом:** Вторая таска была опубликована без изображения(гифки), только текст. В логах не было записи о том, что вторая таска опубликована:
```
bot-1  | 2026-05-09 10:29:34 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-09 10:29:34 INF bot started
bot-1  | 2026-05-09 10:30:08 INF game created admin=green_delfin admin_id=385672319 chat=-1002617613395
bot-1  | 2026-05-09 10:30:37 INF player joined chat=-1002617613395 user=6598439879 username=Jay_jayss
bot-1  | 2026-05-09 10:30:48 INF game started chat=-1002617613395 game=1
bot-1  | 2026-05-09 10:30:49 INF task published chat=-1002617613395 game=1 task=task_01
bot-1  | 2026-05-09 10:30:56 INF awaiting answer chat=-1002617613395 task=task_01 user=385672319
bot-1  | 2026-05-09 10:30:59 INF task answered chat=-1002617613395 game=1 task=task_01 user=385672319
bot-1  | 2026-05-09 10:36:34 INF voting_collage: lock acquired, first category sent chat=-1002617613395 task=task_02 user=385672319
bot-1  | 2026-05-09 10:37:11 INF voting_collage: all categories answered chat=-1002617613395 task=task_02 user=385672319
```

Далее все таски публикуются, но в логах информации об этом.

**Причина:** Два независимых наблюдения:

1. **Опечатка в `task_02.yaml`**: поле `media_file` содержало `"tasks/tasks_02.gif"` (лишняя `s`), тогда как реальный файл называется `task_02.gif`. `LocalStorage.GetAnimation` выбрасывал ошибку "file not found", `publish.go` падал в fallback-ветку и отправлял только текст с клавиатурой без анимации.

2. **«Нет логов» — ожидаемое поведение**: таска 1 публикуется ботом (`Starter.Start → publisher.Publish`), поэтому лог `task published` появляется в `bot-1`. Таски 2+ публикует `scheduler`, поэтому их логи идут в `scheduler-1`. Если пользователь смотрел только `bot-1` — логов планировщика он не видел.

**Исправление:** Опечатка исправлена: `"tasks/tasks_02.gif"` → `"tasks/task_02.gif"` в `content/tasks/task_02.yaml`. Все остальные `media_file`-пути проверены — опечаток не найдено.


## Bug #11 [FIXED]
**Симптом:** На второй таске публикуются изображения и варианты выбора, когда юзер нажимает на кнопку то есть делает какой-то выбор, публикуется следуюшей изображение с вариантами выбора. Предыдущие сообшение с кнопками не удаляется. Должно удаляться. То есть первое изображение и кнопки выбора публикуются, юзер нажал на кнопку то есть сделал выбор, это сообщение бот удалил и отправил следующее сообщение со следующим изображеним и кнопками выбора

**Причина:** `HandleCategoryChoice` (task_02) та `HandlePlayerChoice` (task_04) не отримували посилання на попереднє повідомлення і не викликали `sender.Delete`. Повідомлення з фото + кнопками залишалося в чаті після натискання.

**Исправление:** Додано параметр `prevMsg *tele.Message` до `HandleCategoryChoice` та `HandlePlayerChoice`. Після успішної перевірки лока (лок належить цьому гравцю) викликається `h.sender.Delete(prevMsg)` перед відправкою наступної категорії/питання. В callback handler (`OnTask02Choice`, `OnTask04PlayerChoice`) передається `c.Message()` — це саме те повідомлення, що містить натиснуту кнопку. Якщо лок належить іншому гравцю — видалення не відбувається. Оновлено тести обох сабтасок.


## Bug #12 [FIXED]
**Симптом:** Первая таска при старте игры опубликовалась 2 раза. В чате было 2 юзера, в игре только 1 - админ. Админ нажал Почати гру, игра началась, и бот опубликовал 2 раза первую таску. При этом в консоли логах бота логируется только одна публикается таски:
```
bot-1  | 2026-05-16 11:17:57 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-16 11:17:57 INF bot started
bot-1  | 2026-05-16 11:26:46 INF game created admin="( 385672319 | green_delfin)" chat="(-1002617613395|Test 3)"
bot-1  | 2026-05-16 11:27:43 INF game started chat="(-1002617613395|Test 3)" game=1
bot-1  | 2026-05-16 11:27:44 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_01
bot-1  | 2026-05-16 11:28:06 INF awaiting answer chat="(-1002617613395|Test 3)" task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:28:09 INF task answered chat="(-1002617613395|Test 3)" game=1 task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:28:34 INF player joined chat="(-1002617613395|Test 3)" user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:28:41 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_01 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:32:17 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:33:12 INF voting_collage: all categories answered chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:33:19 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:36:19 INF awaiting answer chat="(-1002617613395|Test 3)" task=task_03 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:36:25 INF task answered chat="(-1002617613395|Test 3)" game=1 task=task_03 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:36:34 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_03 user="( 385672319 | green_delfin)"
```

**Причина:** Гонка данных между ботом и планировщиком при старте игры. В `start.go` последовательность такая:
1. `gameRepo.UpdateStatus(active)` — игра уже `active`, `current_task_order = 0`
2. Отправка стартовой анимации + `time.Sleep(TaskInfoInterval)` + отправка второго сообщения (~1 сек)
3. `publisher.Publish(game)` → `UpdateCurrentTask(order=1)`

Scheduler тикает каждые 15 секунд (и сразу при старте). Если тик попадал в окно между шагами 1 и 3, он видел игру в статусе `active` с `CurrentTaskOrder == 0` и выполнял тот же `publisher.Publish`. Оба вызова работали с объектом `game.CurrentTaskOrder = 0`, оба вычисляли `nextOrder = 1` и оба отправляли первую таску в чат. В логах `bot-1` публикация появляется только одна, потому что scheduler пишет в `scheduler-1`.

**Исправление:** Удалена ветка `CurrentTaskOrder == 0` из `processGame` в `cmd/scheduler/main.go`. Первая таска — исключительная ответственность бота (`Starter.Start → Publisher.Publish`). Scheduler обрабатывает только игры с `CurrentTaskOrder > 0` и установленным `CurrentTaskPublishedAt`, то есть только последующие таски. Условия объединены: `if g.CurrentTaskOrder == 0 || g.CurrentTaskPublishedAt == nil { return }`.

## Bug #13 [FIXED]
**Симптом:** Итоги таски 3 подводяться бесконечно каждые 15 секунд. Это происходит и, если никто не ответил на таску и если ответы есть на таску.

Логи бота и шедулера, когда есть ответы на таску:
```
bot-1  | 2026-05-16 11:17:57 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-16 11:17:57 INF bot started
bot-1  | 2026-05-16 11:26:46 INF game created admin="( 385672319 | green_delfin)" chat="(-1002617613395|Test 3)"
bot-1  | 2026-05-16 11:27:43 INF game started chat="(-1002617613395|Test 3)" game=1
bot-1  | 2026-05-16 11:27:44 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_01
bot-1  | 2026-05-16 11:28:06 INF awaiting answer chat="(-1002617613395|Test 3)" task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:28:09 INF task answered chat="(-1002617613395|Test 3)" game=1 task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:28:34 INF player joined chat="(-1002617613395|Test 3)" user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:28:41 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_01 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:32:17 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:33:12 INF voting_collage: all categories answered chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:33:19 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 11:36:19 INF awaiting answer chat="(-1002617613395|Test 3)" task=task_03 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:36:25 INF task answered chat="(-1002617613395|Test 3)" game=1 task=task_03 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 11:36:34 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_03 user="( 385672319 | green_delfin)"


scheduler-1  | 2026-05-16 11:34:14 INF collage finalized chat="(-1002617613395|Test 3)" game=1 task=task_02
scheduler-1  | 2026-05-16 11:34:14 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_02
scheduler-1  | 2026-05-16 11:36:13 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:38:27 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:38:42 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:38:57 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:39:12 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:39:27 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:39:42 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 11:39:57 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_03

```

Логи бота и шедулера, когда нет ответов на таску:
```
bot-1  | 2026-05-16 11:57:33 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-16 11:57:33 INF bot started
bot-1  | 2026-05-16 12:14:27 INF game created admin="( 385672319 | green_delfin)" chat="(-1002617613395|Test 3)"
bot-1  | 2026-05-16 12:14:47 INF game started chat="(-1002617613395|Test 3)" game=1
bot-1  | 2026-05-16 12:14:48 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_01
bot-1  | 2026-05-16 12:15:00 INF awaiting answer chat="(-1002617613395|Test 3)" task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 12:15:03 INF task answered chat="(-1002617613395|Test 3)" game=1 task=task_01 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 12:15:13 INF player joined chat="(-1002617613395|Test 3)" user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 12:15:20 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_01 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 12:19:01 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 12:19:49 INF voting_collage: all categories answered chat="(-1002617613395|Test 3)" task=task_02 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-16 12:19:58 INF voting_collage: lock acquired, first category sent chat="(-1002617613395|Test 3)" task=task_02 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 12:20:35 INF voting_collage: all categories answered chat="(-1002617613395|Test 3)" task=task_02 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 12:23:10 INF task skipped chat="(-1002617613395|Test 3)" skip_count=1 task=task_03 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-16 12:23:20 INF task skipped chat="(-1002617613395|Test 3)" skip_count=2 task=task_03 user="( 6598439879 | Jay_jayss)"


scheduler-1  | 2026-05-16 11:57:33 INF scheduler started
scheduler-1  | 2026-05-16 12:16:49 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_01
scheduler-1  | 2026-05-16 12:18:49 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_02
scheduler-1  | 2026-05-16 12:21:05 INF collage finalized chat="(-1002617613395|Test 3)" game=1 task=task_02
scheduler-1  | 2026-05-16 12:21:05 INF task finalized chat="(-1002617613395|Test 3)" game=1 task=task_02
scheduler-1  | 2026-05-16 12:23:04 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:25:19 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:25:34 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:25:49 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:26:04 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:26:19 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
scheduler-1  | 2026-05-16 12:26:34 INF task finalized: no answers chat="(-1002617613395|Test 3)" game=1 task=task_03
```

**Причина:** Два финализатора не создавали запись в `task_results`, из-за чего идемпотентная проверка в `FinalizeRouter.Finalize` (`taskResultRepo.GetByTask`) всегда возвращала `nil` и финализация повторялась на каждом тике:

1. **`TextFinalizer`**: отправлял текст итогов, но никогда не вызывал `taskResultRepo.Create`. Все таски с `summary.type: text` (task_03, task_05, task_06, task_07, task_08, task_09, task_11) попадали в бесконечный цикл финализации.

2. **Ветка "нет ответов" в `FinalizeRouter`**: при `len(responses) == 0` роутер отправлял сообщение о том, что ответов нет, и возвращал `nil` без создания `task_result`. Аналогичная бесконечная финализация для любой таски, на которую никто не ответил.

**Исправление:**

1. В `TextFinalizer` (`internal/usecase/task/finalize/text.go`): добавлена зависимость `taskResultRepo repository.TaskResultRepository`. После отправки итогового текста вызывается `taskResultRepo.Create` с `{"type": "text"}`. Конструктор `NewTextFinalizer` обновлён — принимает `taskResultRepo` первым аргументом.

2. В `FinalizeRouter.Finalize` (`internal/usecase/task/finalize/router.go`): в ветке `len(responses) == 0` после отправки `na_answers`-сообщения добавлен вызов `taskResultRepo.Create` с `{"type": "no_answers"}`. Также добавлен вызов `finishGame` для случая, когда таска без ответов является последней.

3. Вызовы `NewTextFinalizer` обновлены в `cmd/scheduler/main.go` и `cmd/bot/main.go`.

4. Тест `TestTextFinalizer_SendsSummaryText` обновлён: теперь проверяет вызов `taskResultRepo.Create`. Тест `TestRouter_NoResponses_SendsNaAnswers` обновлён аналогично.

**Изменённые файлы:** `internal/usecase/task/finalize/text.go`, `internal/usecase/task/finalize/router.go`, `internal/usecase/task/finalize/text_test.go`, `internal/usecase/task/finalize/router_test.go`, `cmd/scheduler/main.go`, `cmd/bot/main.go`.


## Bug #14
**Симптом:** Когда юзер отвечает на таску и бот отправляет сообщение-реакцию, что ответ на таску принят, всегда указано в тексте, что ответ принят на таску 1, хотя отвечает юзер на таску 3


## Bug #15
**Симптом:** В таске 4 порядок расположения кнопок выбора юзера не правильный. Кнопки должны быть расположены по 2 кнопки в линию, а не в столбик как сейчас. То есть если в игре 2 юзера, 2 кнопки в 1 линию, если 4 игрока, то по 2 кнопки в каждую линию, то есть 2 линии по 2 кнопки. Если не четное количество юзеров, то 2 кнопки в 1 линию и 1 кнопка в ноовой строке, растянутая на всю ширину клавиатуры. Если не ошибаюсь, то телеграм сам автоматически растягивает нечетную кнопку на всю ширинку клавиатуры, но проверь это точно.

Располжение кнопок сейчас:
[ username1 ]
[ username 2]

Должно быть таким:
[ username1 ] [ username2 ]
[ username3 ] [ username4 ]

или 

[ username1 ] [ username2 ]
[        username1        ]


## Bug #15
**Симптом:** В таске 4 при подведении итогов не правильный формат текста. Для примера будут приводить первые три вопроса.
Сейчас это два отдельных сообщения:
1. Круасанчики, ролі у вашому серіалі визначено 🍿
Усе збігається? 😏
2.  🍕 Хто з нас з’їсть останній шматок піци і вдаватиме, що він там ніколи не лежав? → @green_delfin
    🤯 Хто з нас забуде, навіщо зайшов у кімнату, і піде назад? → @green_delfin
    💤 Хто з нас може проспати навіть власний день народження? → @green_delfin

То есть идёт вопрос, после него смайлик или символ стрелки вправо и сразу @username и с новой строки следующий вопрос и в таком же формате ответ-юзернейм.

Должно быть:
Всё в одном сообщении.
Круасанчики, ролі у вашому серіалі визначено 🍿
Усе збігається? 😏
после этого текста перенос строки + пустая строкаю
Вопрос и @username-ответ в одной строке.
После этого перенос строкии + пустая строка.
Вопрос и @username-ответ в одной строке.

Пример:
Круасанчики, ролі у вашому серіалі визначено 🍿
Усе збігається? 😏

🍕 Хто з нас з’їсть останній шматок піци і вдаватиме, що він там ніколи не лежав? @green_delfin

🤯 Хто з нас забуде, навіщо зайшов у кімнату, і піде назад? @green_delfin

💤 Хто з нас може проспати навіть власний день народження? @Jay_jayss


## Bug #16
**Симптом:** При подведении итогов таски 8 после текстового сообщения должен отправлться pdf-файл со списком ресурсных занятий. Этот файл есть, куда и с каким именем его мне нужно хранить в папке проекта? где указать место располжение?
В сообщении отправки файл должен называться - Твій список ресурсних занять.pdf


## Bug #17
**Симптом:** В таске 10 нет подведения итогов голосования. Просто ничего не происходит


## Bug #18
**Симптом:** В таске 12 не нужно чтобы нотифаер проверял ответили ли юзеры на таску или нет.


## Bug #19
**Симптом:** В 12 таске сообщения-реакции, когда админ нажимает на кнопку Подилитися мрією и сообщения-реакции, когда админ написал ответ после нажатия кнопки Подилитися мрією должны удаляться.


## Bug #19
**Симптом:** В 12 таске не генерируется коллаж.
Вот логи из консоли бота:
```
bot-1  | 2026-05-16 15:17:43 INF admin_only: first question sent chat="(-1002617613395|Test 3)" task=task_12 user="( 385672319 | green_delfin)"
bot-1  | 2026/05/16 15:18:55 48016495 subtask.admin_only.completeAdminTask: generate collage: openai.GenerateCollage: create image: error, status code: 400, status: 400 Bad Request, message: Unknown parameter: 'response_format'.
```


## Bug #XX
**Симптом:** Тестировал удаления БД при разных режимах запуска или перезапуск проекта. Когда при перезапуске бота БД не удаляется, бот удаляется из чата и заново добавляется в чат, бот отправяется приветственные сообщения - сообщение с кнопками Присоединиться к игре, Техподдержка, Выйти из игры и сообщение с кнопкой старта для админа. Хотя игра уже создана и БД активна и не удалялась. Если на кнопки нажимать, то ничего не происходит просто "Загрузка..." в верху окна телеграмма и через несколько секунд удаляется. Я думаю при перезапуске бота не должны в реальности юзеры удалять бота из чата или и заново добавлять. Подумай, нужно ли как-то реагировать на такое поведение