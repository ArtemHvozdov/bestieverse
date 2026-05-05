CREATE TABLE players (
    id               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id          BIGINT UNSIGNED NOT NULL,
    telegram_user_id BIGINT NOT NULL,
    username         VARCHAR(64),
    first_name       VARCHAR(128),
    skip_count       INT NOT NULL DEFAULT 0,
    joined_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_player_game (game_id, telegram_user_id),
    INDEX idx_telegram_user (telegram_user_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
