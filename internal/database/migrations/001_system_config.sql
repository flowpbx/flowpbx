CREATE TABLE system_config (
    id         INTEGER PRIMARY KEY,
    key        TEXT    NOT NULL UNIQUE,
    value      TEXT,
    updated_at DATETIME DEFAULT (datetime('now'))
);
