CREATE TABLE task_responses (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id       BIGINT UNSIGNED NOT NULL,
    player_id     BIGINT UNSIGNED NOT NULL,
    task_id       VARCHAR(32) NOT NULL,
    status        ENUM('answered', 'skipped') NOT NULL,
    response_data JSON,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_response (game_id, player_id, task_id),
    INDEX idx_task (game_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
