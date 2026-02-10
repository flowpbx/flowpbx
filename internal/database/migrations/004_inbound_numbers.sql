CREATE TABLE inbound_numbers (
    id              INTEGER PRIMARY KEY,
    number          TEXT    NOT NULL,
    name            TEXT,
    trunk_id        INTEGER REFERENCES trunks(id),
    flow_id         INTEGER REFERENCES call_flows(id),
    flow_entry_node TEXT,
    enabled         BOOLEAN DEFAULT 1,
    created_at      DATETIME DEFAULT (datetime('now')),
    updated_at      DATETIME DEFAULT (datetime('now'))
);
