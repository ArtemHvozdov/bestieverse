CREATE TABLE task_locks (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id     BIGINT UNSIGNED NOT NULL,
    task_id     VARCHAR(32) NOT NULL,
    player_id   BIGINT UNSIGNED NOT NULL,
    acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at  TIMESTAMP NOT NULL,

    UNIQUE KEY uq_lock (game_id, task_id),
    INDEX idx_expires (expires_at),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
