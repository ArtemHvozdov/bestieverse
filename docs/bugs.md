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


## Bug #14 [FIXED]
**Симптом:** Когда юзер отвечает на таску и бот отправляет сообщение-реакцию, что ответ на таску принят, всегда указано в тексте, что ответ принят на таску 1, хотя отвечает юзер на таску 3

**Причина:** В `content/messages.yaml` строка `answer_accepted` содержала хардкоднутый номер `#1`: `"{{.Mention}} дякую! Твою відповідь на завдання #1 прийнято ✅"`. В `answer.go` шаблон рендерился со структурой `struct{ Mention string }` — поле для номера таски вообще не передавалось.

**Исправление:**
1. В `content/messages.yaml`: заменено `#1` на `#{{.TaskNum}}`.
2. В `internal/usecase/task/answer.go`: структура, передаваемая в `formatter.RenderTemplate`, расширена полем `TaskNum int`.
3. В `internal/usecase/task/sender.go`: добавлен хелпер `taskOrderFromID(taskID string) int`, который извлекает порядковый номер из ID вида `"task_03"` → `3`.


## Bug #15 [FIXED]
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

**Причина:** В `buildPlayerSelectionKeyboard` (`internal/usecase/task/subtask/who_is_who.go`) каждая кнопка добавлялась в отдельную строку через `kbd.Row(btn)` — отсюда вертикальный столбик.

**Исправление:** Функция переписана с итерацией по шагу 2: каждые два игрока объединяются в один `kbd.Row(btn, btn2)`. Если количество нечётное, последний игрок идёт в отдельную строку `kbd.Row(btn)` — Telegram сам растягивает одиночную кнопку на всю ширину. Та же логика добавлена в `PlayerSelectionKeyboard` в `internal/delivery/bot/keyboard/factory.go` для консистентности.


## Bug #15 [FIXED]
**Симптом:** В таске 4 при подведении итогов не правильный формат текста. Итог присылался двумя отдельными сообщениями, а вопрос и @username разделялись стрелкой `→` без пустых строк между вопросами.

**Причина:** `WhoIsWhoFinalizer.Finalize` в `internal/usecase/task/finalize/who_is_who.go` отправлял два отдельных сообщения: сначала `task.Summary.HeaderText`, затем строки результатов, объединённые через `\n` с разделителем `→` между вопросом и упоминанием.

**Исправление:** Объединено в одно сообщение: заголовок + `\n` + строки результатов через `\n\n` (пустая строка между каждым вопросом). Разделитель `→` удалён — вопрос и @mention стоят рядом через пробел. Обновлены тесты в `who_is_who_test.go`: ожидание 2 сообщений заменено на 1.


## Bug #16 [FIXED]
**Симптом:** При подведении итогов таски 8 после текстового сообщения должен отправляться pdf-файл со списком ресурсных занятий. Файл должен называться "Твій список ресурсних занять.pdf".

**Причина:** `TextFinalizer` отправлял только текст и не имел механизма для отправки вложений. В `TaskSummary` не было полей для указания пути к файлу.

**Исправление:**
1. В `internal/config/task.go` в структуру `TaskSummary` добавлены поля `AttachmentFile string` (путь относительно `MEDIA_PATH`) и `AttachmentName string` (имя файла, отображаемое в Telegram).
2. В `TextFinalizer` (`internal/usecase/task/finalize/text.go`) добавлена зависимость `media.Storage`. После отправки текста, если `task.Summary.AttachmentFile` задан, получает документ через `storage.GetFile`, переопределяет `doc.FileName = task.Summary.AttachmentName` и отправляет файл.
3. В `content/tasks/task_08.yaml` добавлены поля:
   ```yaml
   attachment_file: "task_08/resource_activities.pdf"
   attachment_name: "Твій список ресурсних занять.pdf"
   ```
4. Создана директория `assets/media/task_08/`. **PDF-файл нужно положить туда с именем `resource_activities.pdf`**.
5. Вызовы `NewTextFinalizer` обновлены в `cmd/bot/main.go` и `cmd/scheduler/main.go` — добавлен аргумент `mediaStorage`.
6. Обновлены тесты в `text_test.go` — добавлен `noopMedia{}` как третий аргумент.


## Bug #17 [FIXED]
**Симптом:** В таске 10 нет подведения итогов голосования. Просто ничего не происходит

**Причина (финальная):** Telegram не отправляет `UpdatePoll` события через long polling при автоматическом закрытии опроса по `close_date`. Дополнительно: дефолтный `POLL_DURATION=24h` превышает максимум Telegram (600 секунд), поэтому `close_date` не устанавливался корректно. Первоначальный фикс (удаление `taskResultRepo.Create` из `PollHandler`) был необходим, но недостаточен — `OnPoll` всё равно не срабатывал.

**Исправление (финальное):** Полностью переработан механизм закрытия опроса:
1. Убран `CloseUnixdate` из опроса при публикации — опрос теперь открыт бессрочно.
2. Добавлен столбец `poll_message_id BIGINT` в таблицу `games` (миграция `010`).
3. `Publisher` сохраняет `poll_message_id` в БД вместе с `active_poll_id`.
4. Добавлен метод `PollHandler.ForceClosed` — явно вызывает `bot.StopPoll`, получает результаты голосования, определяет победителя, публикует follow-up задание, очищает `active_poll_id` и `poll_message_id`.
5. Шедулер вызывает `ForceClosed` после истечения `PollDuration`. Пока опрос активен (`active_poll_id != ""`), финализация пропускается.
6. `SetActivePollID` заменён на `SetActivePoll(ctx, id, pollID, msgID)` — атомарно обновляет оба поля.
7. Дефолт `POLL_DURATION` изменён с `24h` на `6h` (должен быть меньше `TASK_FINALIZE_OFFSET=23h`).
8. Добавлен `ClaimActivePoll` — атомарный `UPDATE ... WHERE active_poll_id = ?` — устраняет race condition двойной публикации: `ForceClosed` (шедулер) и `HandlePollClosed` (`OnPoll`, срабатывает от `StopPoll`) могут одновременно определить победителя; только тот, кто выиграл UPDATE, публикует follow-up.

**Изменённые файлы:** `migrations/010_*`, `internal/domain/entity/game.go`, `internal/domain/repository/game.go`, `internal/domain/repository/mocks/mock_game.go`, `internal/infrastructure/mysql/repository/game.go`, `internal/usecase/task/publish.go`, `internal/usecase/task/subtask/sender.go`, `internal/usecase/task/subtask/poll.go`, `internal/usecase/task/subtask/poll_test.go`, `internal/usecase/task/subtask/voting_collage_test.go`, `internal/config/timings.go`, `cmd/bot/main.go`, `cmd/scheduler/main.go`.


## Bug #18 [FIXED]
**Симптом:** В таске 10, когда побеждает в опросе вариант с озвучкой мемов, некоректно работает процес таски с озвучкой мемов. В папке assets/media/task_10/ 25 файлов-мемов. По 5 мемов для каждого игрока. То есть, когда первый юзер нажал на кнопку Озвучити меми, последовательно должны отправляться первые 5 файлов-мемов. Когда второй юзер нажал, должны отправиться 6-10 мемы и т.д. Сейчас для первого юзера отправилось только 4 мема-файла, для второго юзера только 3 файла и дальше не отправляется ничего и при этом отправляются файлы мемы, которые должны отправляться для первого юзера.

**Причина:** В `HandleRequestAnswer` всегда отправлялся первый мем (`memeFiles[0]`) для любого игрока, а условие завершения в `HandleAnswer` было `progress.QuestionIndex < len(memeFiles)` (то есть все 25 мемов). Отсутствовал механизм слотовой раздачи мемов — каждый игрок должен получать только свою порцию (`len(memeFiles) / totalPlayers`) из своего начального индекса.

**Исправление:**
1. В `MemeVoiceoverHandler` добавлен `playerRepo repository.PlayerRepository`.
2. В `HandleRequestAnswer` при создании прогресса теперь вычисляется слот игрока:
   - `totalPlayers = len(playerRepo.GetAllByGame(...))`
   - `answeredCount = taskResponseRepo.CountAnsweredByTask(...)` — сколько игроков уже завершили
   - `memesPerPlayer = len(memeFiles) / totalPlayers`
   - `startIndex = answeredCount * memesPerPlayer`
   - В `progress.AnswersData` сохраняются метаданные `_start` и `_per`
   - Первый отправляемый мем: `memeFiles[startIndex]`
3. В `HandleAnswer` считываются `_start` и `_per` из progress, следующий мем отправляется по индексу `startIndex + progress.QuestionIndex`, финализация происходит когда `QuestionIndex >= memesPerPlayer`.
4. `NewMemeVoiceoverHandler` в `cmd/bot/main.go` обновлён — добавлен аргумент `playerRepo`.
5. Обновлены все тесты в `meme_voiceover_test.go`, добавлен тест `TestMemeHandleRequestAnswer_SecondPlayer_GetsCorrectSlot`.


## Bug #19 [FIXED]
**Симптом:** В таске 12 не нужно чтобы нотифаер проверял ответили ли юзеры на таску или нет.

**Причина:** Нотификатор вызывал `GetUnnotifiedPlayers` для всех тасок подряд. Для `admin_only` (task_12) обычные игроки никогда не создают `task_response`, поэтому все они всегда попадали в список "не ответивших" и получали напоминания.

**Исправление:** В `send_reminder.go` в методе `remindGame` добавлена ранняя проверка `if task.Type == "admin_only" { return nil }` — перед вызовом `GetUnnotifiedPlayers`. Добавлен тест `TestSendReminders_AdminOnlyTask_NoReminder`.


## Bug #20 [FIXED]
**Симптом:** В 12 таске сообщения-реакции, когда админ нажимает на кнопку Подилитися мрією и сообщения-реакции, когда админ написал ответ после нажатия кнопки Подилитися мрією должны удаляться.

**Причина:** `HandleButtonPress` отправлял сообщение `Task12AwaitingAnswer` без удаления (комментарий "does not delete it so the admin can read it"). `HandleAnswer` отправлял `Task12Reply` без удаления.

**Исправление:** В `HandleButtonPress` добавлен `deleteAfter` для сообщения `Task12AwaitingAnswer`. В `HandleAnswer` заменён `h.sender.Send` на захват возвращённого сообщения + `deleteAfter`. Обновлены тесты: `TestAdminHandleButtonPress` теперь проверяет `deleted == 1` (с `time.Sleep(5ms)`), тесты `HandleAnswer` обновлены до `deleted == 2` (синхронное удаление вопроса + асинхронное удаление reply).


## Bug #21 [FIXED]
**Симптом:** В 12 таске не генерируется коллаж.
Вот логи из консоли бота:
```
bot-1  | 2026-05-16 15:17:43 INF admin_only: first question sent chat="(-1002617613395|Test 3)" task=task_12 user="( 385672319 | green_delfin)"
bot-1  | 2026/05/16 15:18:55 48016495 subtask.admin_only.completeAdminTask: generate collage: openai.GenerateCollage: create image: error, status code: 400, status: 400 Bad Request, message: Unknown parameter: 'response_format'.
```

**Причина:** Модель `gpt-image-1` не поддерживает параметр `response_format` — она всегда возвращает изображение в формате `b64_json` и отклоняет запросы с этим параметром. В `openai/client.go` поле `ResponseFormat: openai.CreateImageResponseFormatB64JSON` явно передавалось в запрос, что приводило к ошибке 400.

**Исправление:** Убран `ResponseFormat` из `openai.ImageRequest` в `internal/infrastructure/openai/client.go`. Декодирование `resp.Data[0].B64JSON` оставлено без изменений — `gpt-image-1` возвращает данные именно в этом поле.


## Bug #22 [FIXED]
**Симптом:** В таске 10, когда побеждает вариант с озвучкой мемов: первый юзер получает все 5 мемов и ответ фиксируется. Когда второй юзер нажимает «Хочу озвучити», ему отправляется только 2-3 мема, после чего ничего не происходит — ни следующий мем, ни запись TaskResponse. В логах после отправки последнего мема нет ни одной записи об обработке следующего сообщения.

**Причина:**

Телебот v3 в методе `handleMedia` (`update.go`) маршрутизирует сообщения по типу медиа через switch-case. Порядок проверок:

```go
case m.Animation != nil:
    fired = b.handle(OnAnimation, c)   // ПЕРЕД Document
case m.Document != nil:
    fired = b.handle(OnDocument, c)
```

В Telegram, когда пользователь отправляет GIF-анимацию (через кнопку GIF или пересылает анимацию), у сообщения одновременно заполнены поля `animation` И `document`. Телебот попадает в `case m.Animation != nil` и вызывает `b.handle(OnAnimation, c)`. Поскольку `tele.OnAnimation` не был зарегистрирован в `main.go`, метод возвращал `false`. Затем телебот пробовал `OnMedia` (тоже не зарегистрирован), возвращал `false`, и весь `handleMedia` возвращал `false`. В итоге `ProcessUpdate` **полностью игнорировал сообщение** — ни один хендлер не вызывался, логов не было.

Пользователь при «озвучке» мемов мог отправить анимацию/GIF в качестве своей реакции. Именно такое сообщение выпадало из маршрутизации.

Дополнительно (из предыдущей итерации диагностики):
- `INSERT IGNORE` в `Acquire` не обновлял `expires_at` при повторных вызовах → потенциальная истечение лока.
- Молчаливый `return nil` в `HandleAnswer` при `!lockHolder` без лог-записи.

**Исправление:**

1. В `cmd/bot/main.go` добавлены недостающие регистрации хендлеров:
   ```go
   bot.Handle(tele.OnAnimation, messageHandler.OnMessage)
   bot.Handle(tele.OnSticker, messageHandler.OnMessage)
   ```
   Теперь анимации и стикеры корректно маршрутизируются в `OnMessage`, и далее — в `memeVoiceoverHandler.HandleAnswer`.

2. В `internal/infrastructure/mysql/repository/task_lock.go` метод `Acquire` использует `ON DUPLICATE KEY UPDATE` вместо `INSERT IGNORE` — лок обновляет `expires_at` при каждом вызове `TryAcquire` тем же игроком.

3. В `internal/usecase/task/subtask/meme_voiceover.go` в `HandleAnswer` добавлено WARN-сообщение при `!lockHolder`.

4. В `internal/usecase/task/request_answer.go` добавлена защита от перезаписи state игрока, находящегося в эксклюзивной сабтаске: если `state.TaskID` заканчивается на `:meme` или `:admin`, нажатие «Хочу відповісти» на другую таску игнорируется.

**Изменённые файлы:** `cmd/bot/main.go`, `internal/infrastructure/mysql/repository/task_lock.go`, `internal/usecase/task/subtask/meme_voiceover.go`, `internal/usecase/task/request_answer.go`, `internal/usecase/task/request_answer_test.go`.

**UPDATE (финальный фикс):** Корневая причина — flood control Telegram. `bot.Send(chat, animation)` в `sendMeme` каждый раз загружал GIF-байты с диска через multipart (поле `tele.File` содержало только `FileLocal`, без `FileID`). Telegram считает каждый такой upload против per-chat лимита, и через ~7-10 GIF подряд возвращает `429 retry after N`. Старый код ошибку не обрабатывал — она возвращалась из usecase, и для пользователя «ничего не происходило» (Telegram показывал зависший спиннер). Референсный код в `docs/reference/meme.go` "случайно" не упирался в лимит за счёт `time.AfterFunc(2*time.Second, ...)` между мемами — это полу-решение, которое легко ломалось на быстром тестировании.

Сделано два изменения:

1. **Кэш `file_id` в `media.Storage`**: после успешного `Send` сохраняем `msg.Animation.FileID` через новый метод `Storage.CacheFileID(name, fileID)`. Следующий вызов `GetAnimation(name)` возвращает объект с заполненным `File.FileID` — telebot тогда не делает multipart upload, а просто шлёт `file_id` в запросе. Telegram такие запросы практически не лимитирует. После первой прогонки все 25 мемов кэшируются и больше не аплоадятся повторно.

2. **Ретрай на `tele.FloodError` в `sendMeme`**: если первичный аплоад всё-таки упирается в лимит, читаем `RetryAfter`, спим `RetryAfter+1` сек и пробуем один раз ещё. Это страховка для самого первого прогона с холодным кэшем.

**Изменённые файлы:** `internal/infrastructure/media/local.go` (новый метод `CacheFileID`, `sync.Map` для кэша, ветка возврата cached `File{FileID:...}` в `GetFile`/`GetPhoto`/`GetAnimation`), `internal/usecase/task/subtask/meme_voiceover.go` (FloodError-aware retry + `CacheFileID` после успешного Send). Обновлены тест-стабы: `voting_collage_test.go::testMedia`, `helpers_test.go::noopMedia`, `publish_test.go::stubMedia`, `start_test.go::stubMedia` (no-op `CacheFileID`). Также подчищён неконсистентный expectation `Upsert` в `TestMemeHandleAnswer_LastMeme_FinalizesAndSendsDoneMessage` — finalize-ветка делает `Delete`, не `Upsert`.

---

**Предыдущая итерация (не дала результата):** ранее ты уже фиксил эту таску, но все попытки не дали ожидаемого результата. Для второго юзера публикуется только 2 мема. Далее в последующих тестах такая проблема возникла и для первого юзера, который отвечает на мемы. В логах бота появилась ошибка:
```
bot-1  | 2026/05/23 09:43:05 48016961 subtask.meme_voiceover.HandleAnswer: send next meme: send animation task_10/meme_02.gif: telegram: retry after 15 (429)
```

После этого я перезапустил бота. Возможнро эта ошибка говорит о том, что гифки публикуются слишком часто и это какое-то ограничение самого телеграмма. Далее я уже стал отвечать на мемы не так быстро как до этого. До этого было опубликовался мем, сразу сообщение и так далее. В этот раз я протестировал уже чуть медленее где-то 20-30 секунд пауза была между отправкой ботом мема-гифки и ответом юзером. В этот раз для второго юзера отправилось уже 4 мема и отправка прикратилась и в логах бота появилась похожая ошибка:
```
bot-1  | 2026-05-23 09:54:59 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-23 09:54:59 INF bot started
bot-1  | 2026-05-23 09:55:42 INF game created admin="( 385672319 | green_delfin)" chat="(-1002617613395|Test 3)"
bot-1  | 2026-05-23 09:55:46 INF player joined chat="(-1002617613395|Test 3)" user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:56:03 INF poll sent, storing poll_id and poll_message_id chat="(-1002617613395|Test 3)" game=1 poll_id=5228682324178114157 poll_message_id=11393 task=task_10
bot-1  | 2026-05-23 09:56:03 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_10
bot-1  | 2026-05-23 09:56:03 INF test: task published chat="(-1002617613395|Test 3)" task=10
bot-1  | 2026-05-23 09:57:15 INF processing closed poll poll_id=5228682324178114157
bot-1  | 2026-05-23 09:57:21 INF meme_voiceover: lock acquired, first meme sent chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:57:37 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 09:57:37 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:57:37 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=1 memes_per_player=5 question_index=1 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:57:56 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 09:57:56 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:57:57 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=2 memes_per_player=5 question_index=2 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:07 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 09:58:07 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:08 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=3 memes_per_player=5 question_index=3 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:13 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 09:58:13 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:13 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=4 memes_per_player=5 question_index=4 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:21 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 09:58:21 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:21 INF meme_voiceover: all memes voiced chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 09:58:41 INF meme_voiceover: lock acquired, first meme sent chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:09 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 09:59:09 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:14 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=6 memes_per_player=5 question_index=1 task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:29 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 09:59:29 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:29 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=7 memes_per_player=5 question_index=2 task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:38 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 09:59:38 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:38 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=8 memes_per_player=5 question_index=3 task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 09:59:46 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 09:59:46 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026/05/23 09:59:46 48016992 subtask.meme_voiceover.HandleAnswer: send next meme: send animation task_10/meme_10.gif: telegram: retry after 78 (429)
```

Подобным быстрым способом я тестировал эту таску на другом коде проекта, который сйчас и переписываю. Таких проблем не встречал с мемами. Я добавил код, который исползовал ранее на для такой таски. Он находится в @docs/reference/meme.go. Обработку таски с мемами вызывает функция handleSubTask13() в этом же файле. Изучи этот пример, возможно что-то тебе будет полезно из этого.

**UPDATE:** Перезапустил бот, всё равно "упёрся" в лимиты флуда от телеграм для второго юзера:
```
bot-1  | 2026-05-23 14:12:09 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-05-23 14:12:09 INF bot started
bot-1  | 2026-05-23 14:13:54 INF game created admin="( 385672319 | green_delfin)" chat="(-1002617613395|Test 3)"
bot-1  | 2026-05-23 14:14:05 INF player joined chat="(-1002617613395|Test 3)" user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 14:14:21 INF game started chat="(-1002617613395|Test 3)" game=1
bot-1  | 2026-05-23 14:14:21 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_01
bot-1  | 2026-05-23 14:14:33 INF poll sent, storing poll_id and poll_message_id chat="(-1002617613395|Test 3)" game=1 poll_id=5228682324178114296 poll_message_id=11435 task=task_10
bot-1  | 2026-05-23 14:14:33 INF task published chat="(-1002617613395|Test 3)" game=1 task=task_10
bot-1  | 2026-05-23 14:14:33 INF test: task published chat="(-1002617613395|Test 3)" task=10
bot-1  | 2026-05-23 14:15:39 INF processing closed poll poll_id=5228682324178114296
bot-1  | 2026-05-23 14:15:39 INF poll closed, winner determined chat="(-1002617613395|Test 3)" game=1 task=task_10 winner=memes
bot-1  | 2026-05-23 14:15:50 INF meme_voiceover: lock acquired, first meme sent chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:53 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 14:15:53 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:53 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=1 memes_per_player=5 question_index=1 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:56 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 14:15:56 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:56 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=2 memes_per_player=5 question_index=2 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:59 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 14:15:59 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:15:59 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=3 memes_per_player=5 question_index=3 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:16:02 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 14:16:02 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:16:03 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=4 memes_per_player=5 question_index=4 task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:16:06 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=385672319
bot-1  | 2026-05-23 14:16:06 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:16:06 INF meme_voiceover: all memes voiced chat="(-1002617613395|Test 3)" task=task_10 user="( 385672319 | green_delfin)"
bot-1  | 2026-05-23 14:16:20 INF meme_voiceover: lock acquired, first meme sent chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 14:16:23 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 14:16:23 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 14:16:24 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=6 memes_per_player=5 question_index=1 task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 14:16:27 INF OnMessage: received chat_id=-1002617613395 msg_type=text sender_id=6598439879
bot-1  | 2026-05-23 14:16:27 INF meme_voiceover.HandleAnswer: called chat="(-1002617613395|Test 3)" task=task_10 user="( 6598439879 | Jay_jayss)"
bot-1  | 2026-05-23 14:16:27 WRN meme_voiceover: flood limit hit, sleeping before retry meme=task_10/meme_08.gif retry_after=173
bot-1  | 2026-05-23 14:19:21 INF meme_voiceover: sent next meme chat="(-1002617613395|Test 3)" meme_index=7 memes_per_player=5 question_index=2 task=task_10 user="( 6598439879 | Jay_jayss)"
```

Почитал немного про эти флуд лимиты. Пришёл пока к 3 вариантам решения этой проблемы:
1. Прогрев кэша при старте (радикальное решение) — при запуске бота отправить все мемы в отдельный технический чат, сохранить file_id. После этого все игры идут без upload'ов вообще.
2. Задержка между мемами — добавить time.Sleep(3–4 * time.Second) между отправками. Грубо, но надёжно укладывается в лимит 20 msg/min. Минус — замедляет первого игрока тоже.
3. Комбо: задержка только для cold cache — если file_id не закэширован, спать 3 сек перед отправкой; если закэширован — слать без задержки. После первого прогона задержки исчезают.

Проанализируй эти 3 варианта, какой вариант можно выбрать для решения этой проблемы?

**UPDATE (итоговый фикс):** Выбран вариант 3 с усилением — **persistent JSON-кэш + cold-cache delay**. Реализация:

1. **Персистентный JSON-кэш `file_id` в `media.LocalStorage`**: на старте читает `MEDIA_CACHE_PATH` (по умолчанию `./data/media_cache.json`); после каждого `CacheFileID` атомарно перезаписывает файл (write-tmp + rename) под mutex. После первой реальной прогонки task_10 все 25 file_id сохраняются и переживают рестарт контейнера. Если файл-кэша нет или нечитаем — стартуем с пустым кэшем, не падаем.

2. **Cold-cache delay в `sendMeme`**: если у возвращённого `Animation` нет `FileID` (то есть будет multipart upload с диска) — `time.Sleep(timings.MemeColdCacheDelay)` перед отправкой. Для cached-сендов задержки нет. Значение настраивается через `MEME_COLD_CACHE_DELAY` (default 3s prod, 3s test), идёт через `config.Timings`, поэтому в unit-тестах с zero-Timings sleep автоматически no-op.

3. **Volume mount для `./data`** в `docker-compose.yml` для сервисов `bot` и `scheduler` — без этого JSON-кэш не переживёт `docker compose build/down`.

**Поведение:**
- Первый прогон (cold cache, любой первый игрок): аплоад каждого мема предварён 3-сек паузой → ~15 сек на 5 мемов одного игрока, лимит Telegram не нарушается, все file_id попадают в кэш и пишутся на диск.
- Все последующие прогоны (warm cache): `GetAnimation` возвращает `Animation{File:{FileID:...}}`, telebot не делает upload, cold-cache-delay не срабатывает → отправка мгновенная, никакого 429.
- Если 3 секунды каким-то образом всё же не хватит на холодном старте — сохраняется существующий ретрай на `tele.FloodError` (sleep `RetryAfter+1` + одна повторная попытка) как safety net.

**Изменённые файлы:**
- `internal/infrastructure/media/local.go` — JSON-кэш (load на старте, persist через tmp+rename под mutex), `cacheFile string` в конструкторе.
- `internal/usecase/task/subtask/meme_voiceover.go::sendMeme` — cold-cache check + `time.Sleep(h.timings.MemeColdCacheDelay)` перед upload-ом из disk (nil-safe для тестов).
- `internal/config/timings.go` — поле `MemeColdCacheDelay`, env-vars `MEME_COLD_CACHE_DELAY` / `TEST_MEME_COLD_CACHE_DELAY` (default 3s).
- `internal/config/config.go` — `MediaConfig.CachePath`, env `MEDIA_CACHE_PATH` (default `./data/media_cache.json`).
- `cmd/bot/main.go`, `cmd/scheduler/main.go` — передают `cfg.Media.CachePath` в `NewLocalStorage`.
- `docker-compose.yml` — volume `./data:/app/data` для `bot` и `scheduler`.
- `.env.example` — новые переменные `MEDIA_CACHE_PATH`, `MEME_COLD_CACHE_DELAY`, `TEST_MEME_COLD_CACHE_DELAY`.

**UPDATE (финальный фикс):** 3-секундная задержка не устранила 429 полностью, поскольку задержка была per-сессия одного игрока. Когда игрок 2 начинал сразу после игрока 1, его первые загрузки шли без паузы относительно последней загрузки игрока 1 — возникал burst, который всё равно пробивал лимит Telegram.

**Исправление:**

1. **Глобальный rate limiter в `MemeVoiceoverHandler`** (`coldUploadMu sync.Mutex` + `lastColdUpload time.Time`). Метод `waitColdGap()` вычисляет время, прошедшее с последней cold-загрузки по всему хендлеру (singleton), и спит только оставшийся интервал. Первая cold-загрузка всегда проходит мгновенно. Это устраняет burst на стыке сессий игроков.

2. **Дефолт `MEME_COLD_CACHE_DELAY` поднят с 3s до 20s** (prod). Telegram блокирует примерно после 8 загрузок в скользящем окне ~190 сек. При 20s между загрузками 10 мемов укладываются в ~180 сек — ниже порога. Тестовый дефолт (`TEST_MEME_COLD_CACHE_DELAY`) остался 3s. В продакшне натуральная задержка игроков (они пишут озвучку ≥20 сек) означает, что rate limiter практически не добавляет лишнего ожидания.

3. **`LocalStorage.CacheSize() int`** — метод для диагностики; вызывается при старте в `cmd/bot/main.go` и `cmd/scheduler/main.go` и логирует количество загруженных `file_id`.

**Изменённые файлы:** `internal/usecase/task/subtask/meme_voiceover.go`, `internal/infrastructure/media/local.go`, `internal/config/timings.go`, `cmd/bot/main.go`, `cmd/scheduler/main.go`, `.env.example`.


## Bug #23 [FIXED]
**Симптом:** Тестовые команды отдельных тасок реализуют только публикацию тасок, но остальная логика не работает. По вызову команды должна начинаться игра с нужной таски и работала вся логика и далее таски публиковались по порядку с указанной до конца игры.

**Причина:** `OnTestTask` публиковал таску не проверяя и не обновляя статус игры. Если игра была в статусе `pending`, шедулер игнорировал её (`GetAllActive` возвращает только `active`), а `OnMessage` отклонял сообщения игроков (`game.Status != entity.GameActive`). В результате после публикации таски через тестовую команду ни кнопки, ни ответы игроков не работали.

**Исправление:** В `OnTestTask` (`internal/delivery/bot/handler/test_commands.go`) перед публикацией добавлена проверка статуса игры. Если статус не `active`, вызывается `gameRepo.UpdateStatus(ctx, game.ID, entity.GameActive)`. После этого шедулер подхватывает игру на следующем тике и продолжает публиковать таски по порядку.


## Bug #24 [FIXED]
**Симптом:** Когда при перезапуске бота БД не удаляется, бот удаляется из чата и заново добавляется в чат — бот отправляет приветственные сообщения с кнопками (Приєднатися до гри, Техпідтримка, Вийти з гри, Почати гру), хотя игра уже создана и БД активна. Если нажимать на кнопки, ничего не происходит — только «Загрузка...», которая через несколько секунд исчезает.

**Причина:** В `OnMyChatMember` (`internal/delivery/bot/handler/chat_member.go`) результат `creator.Create` проверялся только на ошибку. Если игра уже существует, `Creator.Create` возвращает `nil, nil` (идемпотентно), но сообщения с кнопками отправлялись всегда — независимо от того, была ли создана новая игра. Старые кнопки вели к сломанному UX: игра была `active` или `finished`, а новые кнопки предполагали `pending`.

**Исправление:** В `OnMyChatMember` добавлена проверка возвращённой игры: если `game == nil` (игра уже существует), хэндлер логирует WARN и возвращает `nil` без отправки сообщений. Сообщения с кнопками отправляются только при создании новой игры.

**Изменённые файлы:** `internal/delivery/bot/handler/chat_member.go`, `internal/delivery/bot/handler/test_commands.go`.


## Bug #25 [FIXED]
**Симптом:** В тасках, где есть лок ответов на таски, чтобы пока только один юзер мог отвечать на таску, например таска 2 или таска 10, в тестовом режиме не освобождается лок. Когда юзер начинает отвечать, создаётся лок, если он на несколько вопросов ответил и далее не отвеает, через 2 минуты пробовал другой юзер отвечать - бот отправил сообщение донт пуш зе хорсес, другая звёздочка отвечает. Если юзер просто нажал Хочу відповісти, создался лок и юзе вообще не отвеает, через 2 минуты также другой юзер не может отвечать. Тестирование было на таске 2 и таск 10 с мемами при параметре TEST_MODE=true.

**Причина:** Два независимых бага:

1. **Неправильная переменная в test-режиме**: в `loadTestTimings` таймаут лока читался из `SUBTASK_LOCK_TIMEOUT` (без `TEST_` префикса) — той же переменной, что и в prod-режиме. В `.env` задано `SUBTASK_LOCK_TIMEOUT=15m`, поэтому тест-режим тоже получал 15 минут. Дефолт `1m` в коде никогда не срабатывал, потому что переменная всегда была задана. Другой игрок пробовал через 2 минуты — лок ещё активен.

2. **Нет периодической очистки локов**: `ReleaseExpired` вызывался только внутри `TryAcquire`. Если второй игрок не нажимал «Хочу відповісти», истёкший лок оставался в БД навсегда — до следующей попытки захвата.

**Исправление:**

1. В `internal/config/timings.go` в функции `loadTestTimings` переменная лока изменена: `parseDurationEnv("SUBTASK_LOCK_TIMEOUT", "1m")` → `parseDurationEnv("TEST_SUBTASK_LOCK_TIMEOUT", "2m")`. Теперь тест-режим использует отдельную переменную с коротким таймаутом, не затрагивая prod-значение.

2. В `cmd/scheduler/main.go` добавлена функция `releaseExpiredLocks` и её вызов в начале каждого тика `tick()`. Создан `taskLockRepo` и передаётся в `tick` как аргумент. Теперь истёкшие локи очищаются каждые 15 секунд независимо от действий других игроков.

3. В `.env.example` добавлена переменная `TEST_SUBTASK_LOCK_TIMEOUT=2m`.

4. В `internal/config/timings_test.go` добавлены:
   - Проверка `TEST_SUBTASK_LOCK_TIMEOUT` в `TestLoadTestTimings_UsesTestEnvVars`
   - Новый тест `TestLoadTestTimings_LockTimeout_NotAffectedByProdVar` — проверяет, что prod-переменная `SUBTASK_LOCK_TIMEOUT=15m` не влияет на test-режим.

**Изменённые файлы:** `internal/config/timings.go`, `internal/config/timings_test.go`, `cmd/scheduler/main.go`, `.env.example`.


## Bug #26 [FIXED]
**Симптом:** В таске 2, когда юзер дал ответы на все вопросы, бот отправляет вариативное сообщение юзера, что как только все юзеры ответят на вопросы, бот создаст коллаж. Сейчас это сообщение не удаляется. Нужно чтобы удалялось через 10 секунд. Время удаленя сообщений прописано в .env. файле.

**Причина:** В `HandleCategoryChoice` (`internal/usecase/task/subtask/voting_collage.go`) при финализации всех категорий followup-сообщение отправлялось через `h.sender.Send(...)` без сохранения возвращённого `*tele.Message`. Без ссылки на сообщение вызвать `deleteAfter` было невозможно.

**Исправление:** Захватываем возвращённое сообщение: `if msg, err := h.sender.Send(...); err == nil && msg != nil { deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay) }`. Теперь followup удаляется через `DeleteMessageDelay` (10 секунд в prod). Тест `TestHandleCategryChoice_LastCategory_FinalizesAndSendsFollowup` не требовал изменений — он уже проверял `len(sender.sent) == 1`, но не проверял удаление followup; логика удаления тестируется через `DeleteMessageDelay: time.Millisecond` в `testTimings()`.


## Bug #27 [FIXED]
**Симптом:** В таске 2, когда подводяться итоги, отправляется 3 сообщения:
1. "Чекайте-чекайте, меджик у процесі … 🧚"
2. "Готово! Ловіть колаж із відповідей, які набрали найбільшу кількість голосів. Схоже на те, що подобається вашій гьорлз бенд? 💅" + изображение сгенерированного коллажа.
3. "Cупер висока якість для моїх aesthetic girls 🎀"

Нужно оставить только первые 2 сообщения. Коллаж для 3 сообщения не нужен.

**Причина:** В `CollageFinalizer.Finalize` (`internal/usecase/task/finalize/collage.go`) после отправки фото-коллажа с подписью `ReadyText` отдельным блоком создавался и отправлялся `*tele.Document` с тем же файлом коллажа и подписью `HqText`.

**Исправление:** Удалён блок создания и отправки документа (`doc := &tele.Document{...}` + `f.sender.Send(chat, doc, ...)`). Теперь финализатор отправляет ровно 2 сообщения: `PendingText` + `*tele.Photo` с `ReadyText`. Обновлён тест `TestCollageFinalizer_MessagesSent`: ожидание изменено с 3 сообщений на 2. Поле `HqText` в `config.TaskSummary` остаётся в структуре (его удаление потребовало бы изменения YAML и всех тестов), но больше не используется в `CollageFinalizer`.


## Bug #28 [FIXED]
**Симптом:** В таске 10, когда побеждает вариант озвучки мемов и запускается подтаска с озвучкой мемов, когда юзер нажимает на кнопку Хочу відповісти, бот должен отправлять вариативное сообщение о том, что ожидает ответа от юзера и через 10 секунд удалять это сообщение как и во всех тасках

**Причина:** В `HandleRequestAnswer` (`internal/usecase/task/subtask/meme_voiceover.go`) после захвата лока, установки прогресса и перевода игрока в состояние `awaiting_answer` — бот сразу отправлял первый мем, не отправляя вариативное `AwaitingAnswer`-сообщение. Все остальные обработчики тасок (см. `request_answer.go`) отправляют случайное сообщение из `msgs.AwaitingAnswer` с `{{.Mention}}` и автоудалением через `DeleteMessageDelay`.

**Исправление:** В `HandleRequestAnswer` после успешного `playerStateRepo.Upsert` добавлена отправка вариативного сообщения `config.Random(h.msgs.AwaitingAnswer)` через `formatter.RenderTemplate` с `{{.Mention}}` и `deleteAfter(h.sender, awaitingMsg, h.timings.DeleteMessageDelay)`. Обновлены тесты: в `testMemeMsgs()` добавлено поле `AwaitingAnswer`; тесты `TestMemeHandleRequestAnswer_LockFree_AcquiresAndSendsFirstMeme` и `TestMemeHandleRequestAnswer_SecondPlayer_GetsCorrectSlot` — ожидаемое `len(sender.sent)` изменено с `1` на `2` (awaiting-сообщение + первый мем).

**Изменённые файлы:** `internal/usecase/task/subtask/meme_voiceover.go`, `internal/usecase/task/subtask/meme_voiceover_test.go`.


## Bug #29 [FIXED]
**Симптом:** В таске 10, когда побеждает вариант озвучки мемов и запускается подтаска с озвучкой мемов, когда юзер озвучил все мемы, вместо вариативного сообщения бот должен отправлять стандартное для всех тасок "{{.Mention}} дякую! Твою відповідь на завдання #{{.TaskNum}} прийнято ✅" и удалять через 10 секунд как и во всех тасках. Так бот должен отвечать на любую победившую в голосовании сабтаску в таске 10, не важно это танец, пропеть песню или озвучка мемов.

**Причина:** В `HandleAnswer` (`internal/usecase/task/subtask/meme_voiceover.go`) при финализации (все мемы озвучены) отправлялось кастомное сообщение из `h.msgs.MemeVoiceoverDone` без auto-delete. По архитектурному паттерну проекта, при завершении ответа на любую таску должен отправляться стандартный `msgs.AnswerAccepted` с `{{.Mention}}` и `{{.TaskNum}}`, который автоматически удаляется через `DeleteMessageDelay` (см. `answer.go`).

**Исправление:** Блок отправки `MemeVoiceoverDone` заменён на рендеринг `h.msgs.AnswerAccepted` с данными `struct{ Mention string; TaskNum int }` где `TaskNum = task.Order`. Сообщение отправляется и автоудаляется через `deleteAfter`. В `testMemeMsgs()` добавлено поле `AnswerAccepted`. Тест `TestMemeHandleAnswer_LastMeme_FinalizesAndSendsDoneMessage` обновлён: добавлен `time.Sleep(5ms)` и проверка `sender.deleted == 1`.

**Изменённые файлы:** `internal/usecase/task/subtask/meme_voiceover.go`, `internal/usecase/task/subtask/meme_voiceover_test.go`.

## Bug #30 [FIXED]
**Симптом:** В таске 12, когда админ нажимет на кнопку Хочу відповісти, сейчас сразу появляются вопросы и кнопу Поділитися мрією. Нужно сделать чтобы после того как админ нажимет на кнопку Хочу відповісти, бот отправлял ещё одной текстовое сообщение без кнопок, а после него уже запускалась отправка вопросов с кнопокй, чтобы админ делился мрією. Наверное, нужно сделать какую-то врменную паузу между текстовым сообщением и вопросами с кнопками. Если считаешь это обоснованным, то значение это времени должно быть вынесено в отдельную .env переменную. Текст для этого нового сообщения, наверное, должен содержаться в файле @task_12.yaml? Если да, то дабавь там соответсвующее поле и короткий текст в качестве сообщения-заглушки. После реализации и теста этого функционала я сам заменю текст-заглушку на нужный.

**Причина:** В `HandleRequestAnswer` (`internal/usecase/task/subtask/admin_only.go`) после перевода игрока в состояние `awaiting_answer` сразу вызывался `sendQuestionMsg` — без какого-либо вводного сообщения и паузы.

**Исправление:**
1. В `internal/config/task.go` в структуру `Task` добавлено поле `RequestAnswerIntro string` с тегом `yaml:"request_answer_intro"`. Если поле не пустое, это сообщение отправляется в чат перед первым вопросом.
2. В `content/tasks/task_12.yaml` добавлено поле `request_answer_intro` с текстом-заглушкой (разработчик заменит на нужный текст).
3. В `internal/config/timings.go` добавлено поле `Task12IntroDelay time.Duration`. Prod-значение читается из `TASK12_INTRO_DELAY` (default 2s), тест-значение — из `TEST_TASK12_INTRO_DELAY` (default 500ms).
4. В `HandleRequestAnswer` после `playerStateRepo.Upsert` добавлен блок: если `task.RequestAnswerIntro != ""` — отправить intro-сообщение, затем `time.Sleep(h.timings.Task12IntroDelay)`, затем отправить первый вопрос с кнопкой.
5. В `.env.example` добавлены переменные `TASK12_INTRO_DELAY=2s` и `TEST_TASK12_INTRO_DELAY=500ms`.


## Bug #31
**Симптом::** В таске 12, если нет ответа от админа, то сейчас публикуется случайное вариативное сообщение-фидбек, если нет ответа. Ддя этой таски есть отдельное сообщение, я его добавил в файле @task_12.yaml в поле followup. Проверь, правильно я описал текст с учатоем форматирования. Если нет ответа от юзера, то бот должен отправлять именно это сообщение, а не случайное вариативное.


## Bug #32
**Симптом:** В таске 12, когда админ уже отвечает на вопросы нажимая на кнопку Поділитися мрією, после каждого ответа бот отправляет сообщение-фидбек. Когда юзер ответил на последний вопрос, это сообщение-фидбек отправлять не нужно. Нужно сразу перейти к логике генерации коллажа.