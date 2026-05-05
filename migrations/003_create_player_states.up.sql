CREATE TABLE player_states (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_id    BIGINT UNSIGNED NOT NULL,
    player_id  BIGINT UNSIGNED NOT NULL,
    state      ENUM('idle', 'awaiting_answer') NOT NULL DEFAULT 'idle',
    task_id    VARCHAR(64),
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_state_player_game (game_id, player_id),
    FOREIGN KEY (game_id)   REFERENCES games(id)   ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
