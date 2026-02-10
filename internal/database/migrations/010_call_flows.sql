CREATE TABLE call_flows (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL,
    flow_data    TEXT    NOT NULL,
    version      INTEGER DEFAULT 1,
    published    BOOLEAN DEFAULT 0,
    published_at DATETIME,
    created_at   DATETIME DEFAULT (datetime('now')),
    updated_at   DATETIME DEFAULT (datetime('now'))
);
