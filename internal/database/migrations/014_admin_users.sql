CREATE TABLE admin_users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    totp_secret   TEXT,
    created_at    DATETIME DEFAULT (datetime('now')),
    updated_at    DATETIME DEFAULT (datetime('now'))
);
