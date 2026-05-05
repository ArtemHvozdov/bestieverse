CREATE TABLE notifications_log (
    id        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id   BIGINT UNSIGNED NOT NULL,
    player_id BIGINT UNSIGNED NOT NULL,
    task_id   VARCHAR(32) NOT NULL,
    sent_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_notification (game_id, player_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
