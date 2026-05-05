CREATE TABLE subtask_progress (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id        BIGINT UNSIGNED NOT NULL,
    player_id      BIGINT UNSIGNED NOT NULL,
    task_id        VARCHAR(32) NOT NULL,
    question_index INT NOT NULL DEFAULT 0,
    answers_data   JSON,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_progress (game_id, player_id, task_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
