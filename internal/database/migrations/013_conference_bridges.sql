CREATE TABLE conference_bridges (
    id             INTEGER PRIMARY KEY,
    name           TEXT    NOT NULL,
    extension      TEXT    UNIQUE,
    pin            TEXT,
    max_members    INTEGER DEFAULT 10,
    record         BOOLEAN DEFAULT 0,
    mute_on_join   BOOLEAN DEFAULT 0,
    announce_joins BOOLEAN DEFAULT 0,
    created_at     DATETIME DEFAULT (datetime('now'))
);
