## Bug #1 [FIXED]
**Симптомы:** Кода юзер, который уже присоединился к игре нажимает на кнопу [Вийти з гри] бот отправляет сообщение с вопросом точно ли хочет выйти юзер из игры и с кнопками [Точно вийти][Назад в гру]. Это сообщение находиться в файле @content/messages.yaml по ключу leave_confirm. Это сообщение остается в чате постоянно до того времени, пока юзер, для которого это сообщение отправлялось, не нажмет одну из кнопок.
Нужно исправить чтобы бот удалял это сообщение через 10 секунд. В корне проекта есть файла .env где это значение времени указано в переменной DELETE_MESSAGE_DELAY.

**Причина:** В `Leaver.InitiateLeave` (`internal/usecase/game/leave.go`) сообщение `leave_confirm` отправлялось через `l.sender.Send(...)` без сохранения возвращённого `*tele.Message`. Без ссылки на сообщение запланировать его удаление через `deleteAfter` было невозможно — сообщение оставалось в чате навсегда, если юзер не нажимал ни одну из кнопок.

**Исправление:** Захватываем возвращённое сообщение: `msg, _ := l.sender.Send(chat, text, kb, formatter.ParseMode)`. Если `msg != nil`, вызывается `deleteAfter(l.sender, msg, l.timings.DeleteMessageDelay)` — тот же паттерн автоудаления, что используется во всех остальных временных сообщениях проекта (`DeleteMessageDelay`, 10 секунд в prod). Нажатие кнопок [Точно вийти]/[Назад в гру] до истечения таймера обрабатывается как раньше (`ConfirmLeave`/`CancelLeave`), повторное удаление уже удалённого сообщения в `CancelLeave` безвредно.

**Изменённые файлы:** `internal/usecase/game/leave.go`, `internal/usecase/game/leave_test.go` (добавлен тест `TestInitiateLeave_NonAdmin_ConfirmAutoDeletes`).



## Bug #2 [FIXED]
**Симптомы:** Когда мы фиксили баг #1, у нас еще появлялся баг, что сообение в файле @content/messages.yaml по ключу leave_confirm отправляется 2 раза ботом в один и тот же чат для одного и того же юзера. Ты изучил этот вопрос и объяснил это, какой-то внутренней особенностью то ли телеграм апи то библиотеки телебот. В качестве решения ты предложил изменить что-то в БД и перед каждым отправлением сообщения от бота скорее всего делается какой-то запрос и записывается в БД для предотвращения дублированния одних и тех же сообщений. Теперь у меня не запускается проект телеграм бота. Вот логи из консоли в момент запуска:

```
docker compose build
[+] Building 69.2s (33/33) FINISHED
 => [internal] load local bake definitions                                                                                                                                                                       0.0s
 => => reading from stdin 1.47kB                                                                                                                                                                                 0.0s
 => [scheduler internal] load build definition from Dockerfile                                                                                                                                                   0.1s
 => => transferring dockerfile: 352B                                                                                                                                                                             0.0s
 => [notifier internal] load build definition from Dockerfile                                                                                                                                                    0.1s
 => => transferring dockerfile: 348B                                                                                                                                                                             0.0s
 => [bot internal] load build definition from Dockerfile                                                                                                                                                         0.1s
 => => transferring dockerfile: 328B                                                                                                                                                                             0.0s
 => [notifier internal] load metadata for docker.io/library/golang:1.25-alpine                                                                                                                                   5.0s
 => [bot internal] load metadata for docker.io/library/alpine:latest                                                                                                                                             5.0s
 => [bot internal] load .dockerignore                                                                                                                                                                            0.0s
 => => transferring context: 2B                                                                                                                                                                                  0.0s
 => [notifier internal] load build context                                                                                                                                                                       9.4s
 => => transferring context: 349.14MB                                                                                                                                                                            9.4s
 => [notifier stage-1 1/5] FROM docker.io/library/alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6                                                                          1.4s
 => => resolve docker.io/library/alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6                                                                                           0.0s
 => => sha256:a9986cd6f37dbddae7862a6d4be71683472e7c2ea708e87db14f8a6393c00f00 4.19MB / 4.19MB                                                                                                                   0.4s
 => => extracting sha256:a9986cd6f37dbddae7862a6d4be71683472e7c2ea708e87db14f8a6393c00f00                                                                                                                        0.7s
 => [notifier builder 1/6] FROM docker.io/library/golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59                                                                     9.9s
 => => resolve docker.io/library/golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59                                                                                      0.0s
 => => sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1 32B / 32B                                                                                                                         0.2s
 => => sha256:de28e276f45c8f7d8e5fbdd7de92cc7e1d57c1b8ae22ad1361234efe0c4da67d 127B / 127B                                                                                                                       0.3s
 => => sha256:e0d20033b9f3681906c8edebff516f7ce729922aa6e424090ca51802b1626da6 57.74MB / 57.74MB                                                                                                                 1.9s
 => => sha256:b974b8cd186c1939e0d085cf5166c6acd3ded905003f4dbff6ee1f93ea0f158b 247.50kB / 247.50kB                                                                                                               0.9s
 => => extracting sha256:b974b8cd186c1939e0d085cf5166c6acd3ded905003f4dbff6ee1f93ea0f158b                                                                                                                        0.4s
 => => extracting sha256:e0d20033b9f3681906c8edebff516f7ce729922aa6e424090ca51802b1626da6                                                                                                                        7.5s
 => => extracting sha256:de28e276f45c8f7d8e5fbdd7de92cc7e1d57c1b8ae22ad1361234efe0c4da67d                                                                                                                        0.0s
 => => extracting sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1                                                                                                                        0.0s
 => [bot stage-1 2/5] WORKDIR /app                                                                                                                                                                               0.4s
 => [bot builder 2/6] WORKDIR /app                                                                                                                                                                               0.7s
 => [scheduler builder 3/6] COPY go.mod go.sum ./                                                                                                                                                                0.0s
 => [bot builder 4/6] RUN go mod download                                                                                                                                                                        3.7s
 => [scheduler builder 5/6] COPY . .                                                                                                                                                                             1.0s
 => [notifier builder 6/6] RUN CGO_ENABLED=0 GOOS=linux go build -o /app/notifier ./cmd/notifier                                                                                                                38.4s
 => [bot builder 6/6] RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot ./cmd/bot                                                                                                                               38.5s
 => [scheduler builder 6/6] RUN CGO_ENABLED=0 GOOS=linux go build -o /app/scheduler ./cmd/scheduler                                                                                                             38.5s
 => [notifier stage-1 3/5] COPY --from=builder /app/notifier .                                                                                                                                                   0.2s
 => [bot stage-1 3/5] COPY --from=builder /app/bot .                                                                                                                                                             0.1s
 => [scheduler stage-1 3/5] COPY --from=builder /app/scheduler .                                                                                                                                                 0.1s
 => [notifier stage-1 4/5] COPY content/ ./content/                                                                                                                                                              0.1s
 => [bot stage-1 4/5] COPY content/ ./content/                                                                                                                                                                   0.1s
 => [scheduler stage-1 4/5] COPY content/ ./content/                                                                                                                                                             0.1s
 => [notifier stage-1 5/5] COPY assets/ ./assets/                                                                                                                                                                0.6s
 => [bot stage-1 5/5] COPY assets/ ./assets/                                                                                                                                                                     0.6s
 => [scheduler stage-1 5/5] COPY assets/ ./assets/                                                                                                                                                               0.6s
 => [bot] exporting to image                                                                                                                                                                                     8.3s
 => => exporting layers                                                                                                                                                                                          5.8s
 => => exporting manifest sha256:888d3611a60bd7c0b188d84116cc36985c925b1aad4a20c1d664b85c6b9df893                                                                                                                0.1s
 => => exporting config sha256:bd3d600d82c2199bbca6f45b18c48bfe77f5134db194b9fcbdcd8557dc3c499f                                                                                                                  0.0s
 => => exporting attestation manifest sha256:13712de3ad6268a6e78845d974b16d7875d3ee65146f241f9d8f5301d7d122ad                                                                                                    0.1s
 => => exporting manifest list sha256:d0d714238faf18441377fc4ebef9f4a0bc763ab8b4741ac6a3477d93c84b76ad                                                                                                           0.0s
 => => naming to docker.io/library/bestieverse-bot:latest                                                                                                                                                        0.0s
 => => unpacking to docker.io/library/bestieverse-bot:latest                                                                                                                                                     2.0s
 => [scheduler] exporting to image                                                                                                                                                                               8.3s
 => => exporting layers                                                                                                                                                                                          6.1s
 => => exporting manifest sha256:14612711545002e269d8e1714b3177c4558f4fb67f99ba1b0096e16dc5c69a9e                                                                                                                0.0s
 => => exporting config sha256:69036775b4fecdc48bfad4a54442b2b166f040c0d0268dbfac3d7122cfe86476                                                                                                                  0.0s
 => => exporting attestation manifest sha256:b711ae393dd7553b6d81001a8bae98233daf324b2e464fba8267b37f69fa64f5                                                                                                    0.1s
 => => exporting manifest list sha256:2014eec016a4ed659beaf3283c6ccb64284c24a35cd58a6730adae3de27648bf                                                                                                           0.0s
 => => naming to docker.io/library/bestieverse-scheduler:latest                                                                                                                                                  0.0s
 => => unpacking to docker.io/library/bestieverse-scheduler:latest                                                                                                                                               1.9s
 => [notifier] exporting to image                                                                                                                                                                                8.3s
 => => exporting layers                                                                                                                                                                                          6.1s
 => => exporting manifest sha256:8782f09cd97aa2882f6d585c37ad95915f8a5f7ebdc2a65d3dfeda160df018ac                                                                                                                0.0s
 => => exporting config sha256:4f2025d061adf11acce6c4b090d2c39174429b83a2f5484e8783b8dc4c656cb0                                                                                                                  0.0s
 => => exporting attestation manifest sha256:6e1cc7782e7c51f175ea3d672f5251f40d4be8a7db3c61344eccd92d7de36bc7                                                                                                    0.1s
 => => exporting manifest list sha256:d906638956cae66bdc0e0c8e0141f1edd79e7c1376abc2576f9c1602c46540ca                                                                                                           0.0s
 => => naming to docker.io/library/bestieverse-notifier:latest                                                                                                                                                   0.0s
 => => unpacking to docker.io/library/bestieverse-notifier:latest                                                                                                                                                1.9s
 => [bot] resolving provenance for metadata file                                                                                                                                                                 0.1s
 => [notifier] resolving provenance for metadata file                                                                                                                                                            0.1s
 => [scheduler] resolving provenance for metadata file                                                                                                                                                           0.0s
[+] Building 3/3
 ✔ bestieverse-notifier   Built                                                                                                                                                                                  0.0s
 ✔ bestieverse-scheduler  Built                                                                                                                                                                                  0.0s
 ✔ bestieverse-bot        Built                                                                                                                                                                                  0.0s
docker compose up -d
[+] Running 6/6
 ✔ Network bestieverse_default        Created                                                                                                                                                                    0.0s
 ✔ Container bestieverse-mysql-1      Healthy                                                                                                                                                                   17.8s
 ✔ Container bestieverse-scheduler-1  Started                                                                                                                                                                   18.5s
 ✔ Container bestieverse-notifier-1   Started                                                                                                                                                                   18.2s
 ✔ Container bestieverse-migrate-1    Started                                                                                                                                                                   18.6s
 ✔ Container bestieverse-bot-1        Started                                                                                                                                                                   18.4s
docker compose logs -f bot
bot-1  | 2026-10-19 15:19:21 INF media cache loaded cached_file_ids=0
bot-1  | 2026-10-19 15:19:21 WRN TEST_MODE enabled: test commands registered
bot-1  | 2026-10-19 15:19:21 INF bot started
bot-1  | 2026-10-19 15:19:21 ERR dedup: cleanup failed error="mysql/update_dedup.DeleteOlderThan: Error 1146 (42S02): Table 'gamebot.processed_updates' doesn't exist"
```

Найди причину, опиши первоначальную проблему и давай откатим изменения назад.

**Первоначальная проблема (дубликат `leave_confirm`):** объяснение про особенность телеграм/телебота было верным и задокументировано в `docs/architecture.md` (раздел «Дедупликация Telegram-апдейтов», теперь удалён вместе с откатом): `getUpdates` (long polling) гарантирует доставку **at-least-once**, а не exactly-once — апдейт считается подтверждённым Telegram-сервером только когда бот делает *следующий* `getUpdates`-вызов со сдвинутым офсетом. `tele.LongPoller` хранит этот офсет только в памяти процесса и не персистит его. Если процесс убивается (например, рестарт контейнера) сразу после обработки апдейта, но до следующего `getUpdates`-вызова — Telegram повторно доставляет тот же апдейт уже свежезапущенному процессу. Хендлеры (в частности `Leaver.InitiateLeave`) не идемпотентны — просто безусловно шлют сообщение, — поэтому повтор апдейта дублирует и побочный эффект (второе `leave_confirm`).

**Причина падения проекта (почему не стартует сейчас):** предложенное решение — таблица `processed_updates` + middleware `Dedup`, захватывающий `update_id` через `INSERT IGNORE` — само по себе рабочее, но было подключено с гонкой при старте инфраструктуры. В `docker-compose.yml` сервисы `bot` / `notifier` / `scheduler` объявлены как `depends_on: mysql: condition: service_healthy` — то есть ждут готовности **только MySQL**, а не завершения сервиса `migrate`. `bot` и `migrate` стартуют практически одновременно (в логах оба `Started` с разницей в доли секунды). При этом в `cmd/bot/main.go` сразу после `bot.Use(botmw.Dedup(...))` запускалась горутина `go botmw.RunDedupCleanup(...)`, которая выполняет `cleanup()` **синхронно, немедленно при старте**, ещё до первого тика тикера. Если на этот момент `migrate` не успел применить миграцию `011_create_processed_updates` (а остальные 10 миграций уже были накатаны в прошлых запусках — пересоздавать приходилось только новую), запрос `DeleteOlderThan` падает с `Table 'gamebot.processed_updates' doesn't exist`. То же самое рано или поздно ударило бы и по самому middleware `Dedup` (`repo.TryClaim`) при более медленном `migrate` — просто `TryClaim` там спроектирован с fail-open (`err != nil` → обработать апдейт как обычно), а `RunDedupCleanup` ошибку просто логирует и не ретраит, из-за чего в логе она более заметна первой.

**Исправление (откат):** по просьбе разработчика фича дедупликации апдейтов полностью откачена (а не допилена через `depends_on: migrate` + healthcheck/condition `service_completed_successfully`, что было бы альтернативным, но более инвазивным для docker-compose.yml решением):
1. Удалены файлы: `internal/delivery/bot/middleware/dedup.go`, `internal/delivery/bot/middleware/dedup_test.go`, `internal/domain/repository/update_dedup.go`, `internal/domain/repository/mocks/mock_update_dedup.go`, `internal/infrastructure/mysql/repository/update_dedup.go`, `migrations/011_create_processed_updates.{up,down}.sql`.
2. В `cmd/bot/main.go` убраны: `updateDedupRepo := mysqlrepo.NewUpdateDedupRepo(db)`, `bot.Use(botmw.Dedup(...))`, горутина `go botmw.RunDedupCleanup(...)` и ставший неиспользуемым импорт `"context"`.
3. В `docs/architecture.md` убран раздел «Дедупликация Telegram-апдейтов» и упоминания `Dedup` в middleware-цепочке и в списке миграций (строка `011_create_processed_updates` в перечне файлов миграций).
4. Правки Bug #1 (автоудаление `leave_confirm` в `internal/usecase/game/leave.go` + тест) не трогались — это рабочий, не связанный с дедупликацией фикс.
5. `go build ./...` и `go test ./...` проходят чисто.

**На будущее:** если к дедупликации апдейтов вернутся, обязательно нужно сначала убрать гонку старта — например, добавить сервису `bot` настоящую зависимость от `migrate` (`depends_on: migrate: condition: service_completed_successfully` в `docker-compose.yml`), а не только от `mysql`, и/или не выполнять `cleanup()` синхронно сразу при старте горутины.


## Bug #3 [FIXED]
**Симптомы:** Когда игрок пропускает задания,у него есть возможность пропустить только три задания. На каждый пропуск бот отправляет сообщение с информацией, что осталас одна попытка, осталось две попытки и эта попытка была последней. Тексты этих сообщений находяться в файле @content/messages.yaml по ключам skip_with_remaining_2, skip_with_remaining_1 и skip_last. Сейчас эти сообщения исчезающие. Удаляются, кажется, через 10 секунд. Нужно исправить чтобы эти сообщения не удалялись. Чтобы оставались постоянно в чате. По завершению исправления нужно обновить документацию и файл CLAUDE.md, если об этом моменте там упоминается.

**Причина:** В `Skipper.Skip` (`internal/usecase/task/skip.go`) после отправки сообщения об остатке пропусков (`skip_with_remaining_2` / `skip_with_remaining_1` / `skip_last`) безусловно вызывался `deleteAfter(..., DeleteMessageDelay)` — тот же паттерн, что и для «ошибочных» сообщений, из-за чего они пропадали через 10 секунд.

**Исправление:** Убран вызов `deleteAfter` для этих трёх сообщений — результат `Send` теперь не используется, сообщения остаются в чате постоянно. Автоудаление сохранено для `already_answered`, `already_skipped` и `skip_no_remaining`. Обновлены `PLAN.md` (п. 5.4) и `docs/architecture.md` (раздел «Жизненный цикл таски»); в `CLAUDE.md` это поведение не упоминается — правок не требовалось.

**Изменённые файлы:** `internal/usecase/task/skip.go`, `internal/usecase/task/skip_test.go` (добавлен `TestSkip_RemainingMessages_NotAutoDeleted`), `PLAN.md`, `docs/architecture.md`, `docs/bugs_1.md`.


## Bug #4 [FIXED]
**Симптомы:** В задании 4, когда в голосовании набирается одинаковое количество голосов (юзеров) и, когда бот отправляет сообщение с подведением итогов и теганием юзеров, которые набрали больше всего голосов, бот тегает только админа игры. Нужно изменить этот моент, если есть пункт, где набирается одинаковое количество за нескольких юзеров, нужно тегнуть их всех в сообщении подведения итогов.

То есть, набирает наибольшее количество голосов юзер1 - бот тегает юзера1, набирает наибольшее количество голосов юзер1 и юзер3, бот тегает юзера1 и юзера3.

Исправления этого момента, не должно "ломать" никакую другую логику.

**Причина:** В `WhoIsWhoFinalizer.Finalize` (`internal/usecase/task/finalize/who_is_who.go`) победитель по вопросу выбирался как один игрок: цикл по `allPlayers` с условием `votes > maxVotes` (строгое «больше») оставлял первого игрока с максимумом. При ничьей это всегда первый в списке игроков — то есть админ (он создаётся первым).

**Исправление:** Победители теперь собираются списком: при `n > maxVotes` список сбрасывается, при `n == maxVotes` игрок добавляется. Игроки без голосов пропускаются. В итоговом сообщении все победители вопроса тегаются через `, `. Порядок — как порядок игроков в игре. `task_results.ResultData` теперь хранит `{"q1": [uid, ...]}` вместо `{"q1": uid}` (читателей этого поля в коде нет). Остальная логика (подсчёт голосов, заголовок, сабтаска) не менялась.

**Изменённые файлы:** `internal/usecase/task/finalize/who_is_who.go`, `internal/usecase/task/finalize/who_is_who_test.go` (тест ничьей заменён на `TestWhoIsWhoFinalizer_Tie_AllLeadersMentioned`), `PLAN.md`, `docs/bugs_1.md`.


## Bug #5 [FIXED]
**Симптомы:** Изучи всю информацию, которая касается задания 10. Нужно провести масштабное обновление этого задания. В этом задании есь три подзадания:
1. Станцювати “Abracadabra” Леді Гаги
2. 🙉 Заспівати шлягер Фібі Буфе “Smelly Cat”
3. 🙊 Озвучити меми 🎤

Нужно убрать подтаску с озвучкой мемов и всю логику, которая с ней связана. Остается только первые две подтаски Станцювати “Abracadabra” Леді Гаги и 🙉 Заспівати шлягер Фібі Буфе “Smelly Cat”. Из текста самого задания 10 в файле @content/tasks/task_10.yaml ничего не удаляй. Изображения мемов также не удаляй. Просто скажи, что с этими файлами нужно сделать.
Фикс этого бага не должен затрагивать логику, которая не относится к заданию 10 и не должен ничего ломать.

**Причина:** Задание 10 (`poll_then_task`) содержало третий вариант голосования `memes` (`result_type: meme_voiceover`) и всю цепочку под него: `MemeVoiceoverHandler`, кнопку/роут `task10_meme_request`, маршрутизацию сообщений по суффиксу `:meme`, ветку в `PollHandler.publishFollowUp`, тексты, поля конфига и тайминг `MEME_COLD_CACHE_DELAY`.

**Исправление:**
1. Удалены `internal/usecase/task/subtask/meme_voiceover.go` и `meme_voiceover_test.go`.
2. `cmd/bot/main.go`: убраны создание `MemeVoiceoverHandler`, его передача в `CallbackHandler`/`MessageHandler` и роут `\ftask10_meme_request`.
3. `handler/callback.go`: удалён `OnTask10MemeRequest`; `handler/message.go`: удалена ветка `:meme`; `keyboard/factory.go`: удалён `MemeVoiceoverKeyboard`.
4. `subtask/poll.go`: убран `case "meme_voiceover"` (неизвестный `result_type` по-прежнему даёт ошибку). `task/request_answer.go`: guard теперь проверяет только `:admin`.
5. Конфиг: удалены `MemeVoiceoverAnnounce/Done` (`messages.go` + `messages.yaml`), `PollOption.MemeFiles/MemesPerPlayer`, `Timings.MemeColdCacheDelay` и `MEME_COLD_CACHE_DELAY`/`TEST_MEME_COLD_CACHE_DELAY` (`.env.example`).
6. `content/tasks/task_10.yaml`: текст задания и две оставшиеся опции не тронуты; опция `memes` **закомментирована** (не удалена), чтобы в опросе было ровно два варианта.
7. Обновлены `CLAUDE.md`, `docs/architecture.md`, `PLAN.md`, `poll_test.go`. `go build`, `go vet`, `go test ./...` проходят.

**Файлы мемов (`assets/media/task_10/meme_*.gif`, 20–25 шт.):** не удалены. Теперь не используются; можно удалить из репозитория и `assets/`, чтобы уменьшить Docker-образ (`COPY assets/`), либо оставить. Закомментированный блок `memes` в `task_10.yaml` можно удалить вместе с ними. Если бот уже запущен, в `.env` можно убрать `MEME_COLD_CACHE_DELAY` (лишние переменные игнорируются). Незавершённое состояние игрока с `task_id = "...:meme"` в старых БД теперь игнорируется.
