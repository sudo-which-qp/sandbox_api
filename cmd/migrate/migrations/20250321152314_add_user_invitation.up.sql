CREATE TABLE IF NOT EXISTS user_invitations (
    token VARCHAR(255) PRIMARY KEY,
    user_id INT UNSIGNED NOT NULL,
    expires_at TIMESTAMP NOT NULL
);