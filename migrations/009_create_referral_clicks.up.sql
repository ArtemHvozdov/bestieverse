CREATE TABLE referral_clicks (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    admin_user_id BIGINT NOT NULL,
    new_user_id   BIGINT,
    clicked_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_admin (admin_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
