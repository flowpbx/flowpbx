CREATE TABLE registrations (
    id            INTEGER PRIMARY KEY,
    extension_id  INTEGER REFERENCES extensions(id),
    contact_uri   TEXT    NOT NULL,
    transport     TEXT,
    user_agent    TEXT,
    source_ip     TEXT,
    source_port   INTEGER,
    expires       DATETIME,
    registered_at DATETIME DEFAULT (datetime('now')),
    push_token    TEXT,
    push_platform TEXT,
    device_id     TEXT
);

CREATE INDEX idx_registrations_extension_id ON registrations(extension_id);
CREATE INDEX idx_registrations_expires ON registrations(expires);
