CREATE TABLE push_tokens (
    id            INTEGER PRIMARY KEY,
    extension_id  INTEGER NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
    token         TEXT    NOT NULL,
    platform      TEXT    NOT NULL, -- "fcm" or "apns"
    device_id     TEXT    NOT NULL,
    app_version   TEXT,
    created_at    DATETIME DEFAULT (datetime('now')),
    updated_at    DATETIME DEFAULT (datetime('now')),
    UNIQUE(extension_id, device_id)
);

CREATE INDEX idx_push_tokens_extension_id ON push_tokens(extension_id);
CREATE INDEX idx_push_tokens_token ON push_tokens(token);
