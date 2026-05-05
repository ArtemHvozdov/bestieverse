CREATE TABLE games (
    id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_id                  BIGINT NOT NULL,
    chat_name                VARCHAR(255) NOT NULL,
    admin_user_id            BIGINT NOT NULL,
    admin_username           VARCHAR(64),
    status                   ENUM('pending', 'active', 'finished') NOT NULL DEFAULT 'pending',
    current_task_order       INT NOT NULL DEFAULT 0,
    current_task_published_at TIMESTAMP NULL,
    active_poll_id           VARCHAR(64),
    created_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at               TIMESTAMP NULL,
    finished_at              TIMESTAMP NULL,

    UNIQUE KEY uq_game_chat (chat_id),
    INDEX idx_status (status),
    INDEX idx_active_poll (active_poll_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
