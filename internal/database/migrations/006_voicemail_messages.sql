CREATE TABLE voicemail_messages (
    id             INTEGER PRIMARY KEY,
    mailbox_id     INTEGER NOT NULL REFERENCES voicemail_boxes(id),
    caller_id_name TEXT,
    caller_id_num  TEXT,
    timestamp      DATETIME NOT NULL DEFAULT (datetime('now')),
    duration       INTEGER,
    file_path      TEXT    NOT NULL,
    read           BOOLEAN DEFAULT 0,
    read_at        DATETIME,
    transcription  TEXT,
    created_at     DATETIME DEFAULT (datetime('now'))
);
