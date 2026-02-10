CREATE TABLE ring_groups (
    id             INTEGER PRIMARY KEY,
    name           TEXT    NOT NULL,
    strategy       TEXT    DEFAULT 'ring_all',
    ring_timeout   INTEGER DEFAULT 30,
    members        TEXT    NOT NULL,
    caller_id_mode TEXT    DEFAULT 'pass',
    created_at     DATETIME DEFAULT (datetime('now')),
    updated_at     DATETIME DEFAULT (datetime('now'))
);
