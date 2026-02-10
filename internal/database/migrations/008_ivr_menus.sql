CREATE TABLE ivr_menus (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL,
    greeting_file TEXT,
    greeting_tts  TEXT,
    timeout       INTEGER DEFAULT 10,
    max_retries   INTEGER DEFAULT 3,
    digit_timeout INTEGER DEFAULT 3,
    options       TEXT    NOT NULL,
    created_at    DATETIME DEFAULT (datetime('now')),
    updated_at    DATETIME DEFAULT (datetime('now'))
);
