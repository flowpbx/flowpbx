CREATE TABLE extensions (
    id                 INTEGER PRIMARY KEY,
    extension          TEXT    NOT NULL UNIQUE,
    name               TEXT    NOT NULL,
    email              TEXT,
    sip_username       TEXT    NOT NULL UNIQUE,
    sip_password       TEXT    NOT NULL,
    ring_timeout       INTEGER DEFAULT 30,
    dnd                BOOLEAN DEFAULT 0,
    follow_me_enabled  BOOLEAN DEFAULT 0,
    follow_me_numbers  TEXT,
    recording_mode     TEXT    DEFAULT 'off',
    max_registrations  INTEGER DEFAULT 5,
    created_at         DATETIME DEFAULT (datetime('now')),
    updated_at         DATETIME DEFAULT (datetime('now'))
);
