CREATE TABLE task_results (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id      BIGINT UNSIGNED NOT NULL,
    task_id      VARCHAR(32) NOT NULL,
    result_data  JSON NOT NULL,
    finalized_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_result (game_id, task_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
