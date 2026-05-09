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


## Bug #6
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

## Bug #7
**Симптом:** Когда игра уже началась и опубликована первая таска, если юзер до старта игры не присоединился к игре, после этого он уже не может присоединиться. Юзер нажимает кнопки Хочу відповісти или Пропустити, бот отправляет сообщение в чат, о том что юзер ещё не в игре и ему нужно присоединиться к игре нажав на кнопку в начале игры. Юзер нажимает на кнопку Приеднатися до гри, вверху окна телеграмма появляется надпись "Загрузка...", через несколько секунд она исчезает и ничего не происходит


## Bug #8
**Симптом:** Первая таска, в игре 2 юзера, один ответил на таску, бот отправил сообщение о, том что ответ на таску принят. Уведомления для второго юзера о том, что от него нет ответа было отправлено через 2 минуты после публикации таски, хотя ожидалось через 1. Также не было подведения итогов таски. Следующая таска опубликовалась через 4 минуты, ожидалось через 3.
Временные параметры были такие указаны при запуске бота:
TEST_MODE=true
TEST_TASK_PUBLISH_INTERVAL=3m
TEST_TASK_FINALIZE_OFFSET=2m
TEST_REMINDER_DELAY=1m
TEST_POLL_DURATION=1m


## Bug #9
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

## Bug #10
**Симптом:** На второй таске когда публикуется изображение и варианты ответа в видео кнопок не такая расстановка кнопок под сообщением.
структура изображения:
 -------------
| var1 | var2 |
 -------------
| var3 | var4 |
 -------------

 Поррядок кнопок под этим сообщением:
 [  var1  ]
 [  var2  ]
 [  var3  ]
 [  var4  ]

 Порядок кнопок, который должен быть:
 [  var1  ] [  var2  ]
 [  var3  ] [  var4  ]


## Bug #11
**Симптом:** На второй таске публикуются изображения и варианты выбора, когда юзер нажимает на кнопку то есть делает какой-то выбор, публикуется следуюшей изображение с вариантами выбора. Предыдущие сообшение с кнопками не удаляется. Должно удаляться. То есть первое изображение и кнопки выбора публикуются, юзер нажал на кнопку то есть сделал выбор, это сообщение бот удалил и отправил следующее сообщение со следующим изображеним и кнопками выбора