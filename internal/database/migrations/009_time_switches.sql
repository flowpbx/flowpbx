CREATE TABLE time_switches (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL,
    timezone     TEXT    DEFAULT 'Australia/Sydney',
    rules        TEXT    NOT NULL,
    default_dest TEXT,
    created_at   DATETIME DEFAULT (datetime('now')),
    updated_at   DATETIME DEFAULT (datetime('now'))
);
