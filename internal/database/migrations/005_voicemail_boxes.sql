CREATE TABLE voicemail_boxes (
    id                   INTEGER PRIMARY KEY,
    name                 TEXT    NOT NULL,
    mailbox_number       TEXT    UNIQUE,
    pin                  TEXT,
    greeting_file        TEXT,
    greeting_type        TEXT    DEFAULT 'default',
    email_notify         BOOLEAN DEFAULT 0,
    email_address        TEXT,
    email_attach_audio   BOOLEAN DEFAULT 1,
    max_message_duration INTEGER DEFAULT 120,
    max_messages         INTEGER DEFAULT 50,
    retention_days       INTEGER DEFAULT 90,
    notify_extension_id  INTEGER REFERENCES extensions(id),
    created_at           DATETIME DEFAULT (datetime('now')),
    updated_at           DATETIME DEFAULT (datetime('now'))
);
