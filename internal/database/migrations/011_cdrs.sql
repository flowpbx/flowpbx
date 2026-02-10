CREATE TABLE cdrs (
    id             INTEGER PRIMARY KEY,
    call_id        TEXT    NOT NULL,
    start_time     DATETIME NOT NULL,
    answer_time    DATETIME,
    end_time       DATETIME,
    duration       INTEGER,
    billable_dur   INTEGER,
    caller_id_name TEXT,
    caller_id_num  TEXT,
    callee         TEXT,
    trunk_id       INTEGER,
    direction      TEXT,
    disposition    TEXT,
    recording_file TEXT,
    flow_path      TEXT,
    hangup_cause   TEXT
);

CREATE INDEX idx_cdrs_call_id ON cdrs(call_id);
CREATE INDEX idx_cdrs_start_time ON cdrs(start_time);
