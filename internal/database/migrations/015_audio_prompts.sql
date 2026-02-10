CREATE TABLE audio_prompts (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    filename   TEXT    NOT NULL,
    format     TEXT    NOT NULL,
    file_size  INTEGER NOT NULL,
    file_path  TEXT    NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);
