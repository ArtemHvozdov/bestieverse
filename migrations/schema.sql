-- ============================================================
-- Game Bot — Database Schema
-- MySQL 8.0
-- ============================================================

-- Игры (одна запись = одна игра в одном чате)
CREATE TABLE games (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_id         BIGINT NOT NULL,               -- Telegram chat ID
    chat_name       VARCHAR(255) NOT NULL,
    admin_user_id   BIGINT NOT NULL,               -- Telegram user ID админа
    admin_username  VARCHAR(64),
    status          ENUM('pending', 'active', 'finished') NOT NULL DEFAULT 'pending',
    current_task_order INT NOT NULL DEFAULT 0,     -- порядковый номер текущей таски (из task.order)
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      TIMESTAMP NULL,
    finished_at     TIMESTAMP NULL,

    UNIQUE KEY uq_game_chat (chat_id),             -- один чат = одна активная игра
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Участники игры
CREATE TABLE players (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    telegram_user_id BIGINT NOT NULL,
    username        VARCHAR(64),                   -- может быть NULL у некоторых пользователей
    first_name      VARCHAR(128),
    skip_count      INT NOT NULL DEFAULT 0,        -- использованных пропусков (макс 3)
    joined_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_player_game (game_id, telegram_user_id),
    INDEX idx_telegram_user (telegram_user_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Состояния игроков (персистентное "что сейчас делает юзер")
-- Позволяет пережить рестарт бота без потери состояния
CREATE TABLE player_states (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    player_id       BIGINT UNSIGNED NOT NULL,
    state           ENUM('idle', 'awaiting_answer') NOT NULL DEFAULT 'idle',
    task_id         VARCHAR(32),                   -- ID таски из YAML (например "task_02")
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_state_player_game (game_id, player_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Ответы участников на таски
CREATE TABLE task_responses (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    player_id       BIGINT UNSIGNED NOT NULL,
    task_id         VARCHAR(32) NOT NULL,          -- ID таски из YAML
    status          ENUM('answered', 'skipped') NOT NULL,
    -- Для сабтасок с выбором варианта (таска 2, 4) — сохраняем JSON
    response_data   JSON,                          -- {"option_id": "smoothie"} или {"question_1": "user_id"}
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_response (game_id, player_id, task_id),
    INDEX idx_task (game_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Эксклюзивные локи для сабтасок (таски 2, 4, 10)
-- Пока один юзер отвечает на сабтаску — остальные ждут
CREATE TABLE task_locks (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    task_id         VARCHAR(32) NOT NULL,          -- ID таски из YAML
    player_id       BIGINT UNSIGNED NOT NULL,      -- кто держит лок
    acquired_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMP NOT NULL,            -- acquired_at + SUBTASK_LOCK_TIMEOUT (15m)

    UNIQUE KEY uq_lock (game_id, task_id),         -- только один лок на таску в игре
    INDEX idx_expires (expires_at),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Прогресс внутри сабтасок с несколькими вопросами (таски 2, 4)
-- Хранит промежуточные ответы пока юзер не ответил на все вопросы
CREATE TABLE subtask_progress (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    player_id       BIGINT UNSIGNED NOT NULL,
    task_id         VARCHAR(32) NOT NULL,
    question_index  INT NOT NULL DEFAULT 0,        -- текущий вопрос (0-based)
    answers_data    JSON,                          -- накопленные ответы
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_progress (game_id, player_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Итоги тасок (кешируем результаты для финальных сообщений)
CREATE TABLE task_results (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    task_id         VARCHAR(32) NOT NULL,
    result_data     JSON NOT NULL,                 -- зависит от типа таски
    -- task_02: {"winners": {"smoothie": 3, "cappuccino": 1}}
    -- task_04: {"question_1": "user_id_123", "question_2": "user_id_456"}
    -- task_10: {"winning_option": "dance"}
    finalized_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_result (game_id, task_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Журнал отправленных напоминаний (нотификатор не отправляет дважды)
CREATE TABLE notifications_log (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id         BIGINT UNSIGNED NOT NULL,
    player_id       BIGINT UNSIGNED NOT NULL,
    task_id         VARCHAR(32) NOT NULL,
    sent_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_notification (game_id, player_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- Реферальные переходы (аналитика на будущее)
CREATE TABLE referral_clicks (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    admin_user_id   BIGINT NOT NULL,               -- чей реферальный код
    new_user_id     BIGINT,                        -- кто перешёл (если известен)
    clicked_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_admin (admin_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- ============================================================
-- Примечания к схеме:
--
-- 1. task_id везде — строковый ID из YAML ("task_01", "task_02")
--    Не числовой, чтобы порядок тасок мог меняться без миграций БД
--
-- 2. response_data (JSON) в task_responses:
--    - question_answer: NULL (факт ответа важен, не содержимое)
--    - voting_collage:  {"votes": {"smoothie": true}}
--    - who_is_who:      {"q0": "tg_user_id_1", "q1": "tg_user_id_2", ...}
--    - admin_only:      {"answers": ["Лондон", "Лепка", "Billie Eilish"]}
--
-- 3. skip_count в players — счётчик на всю игру (не на таску)
--    Максимум 3. При попытке 4-го пропуска — отказ.
--
-- 4. player_states.state = 'awaiting_answer':
--    Бот ждёт следующее сообщение от этого юзера как ответ на таску.
--    При рестарте бота состояние сохраняется и корректно восстанавливается.
--
-- 5. task_locks.expires_at:
--    Если expires_at < NOW() — лок считается просроченным и свободным.
--    Scheduler/Notifier чистит просроченные локи при каждом запуске.
-- ============================================================