-- Push gateway schema: licenses, installations, push_logs

CREATE TABLE IF NOT EXISTS licenses (
    id              BIGSERIAL PRIMARY KEY,
    key             TEXT        NOT NULL UNIQUE,
    tier            TEXT        NOT NULL DEFAULT 'free',
    max_extensions  INTEGER     NOT NULL DEFAULT 5,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_licenses_key ON licenses (key);

CREATE TABLE IF NOT EXISTS installations (
    id            BIGSERIAL PRIMARY KEY,
    license_id    BIGINT      NOT NULL REFERENCES licenses(id) ON DELETE CASCADE,
    instance_id   TEXT        NOT NULL UNIQUE,
    hostname      TEXT        NOT NULL,
    version       TEXT        NOT NULL DEFAULT '',
    activated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_installations_license_id ON installations (license_id);
CREATE INDEX idx_installations_instance_id ON installations (instance_id);

CREATE TABLE IF NOT EXISTS push_logs (
    id          BIGSERIAL PRIMARY KEY,
    license_key TEXT        NOT NULL,
    platform    TEXT        NOT NULL,
    call_id     TEXT        NOT NULL,
    success     BOOLEAN     NOT NULL DEFAULT FALSE,
    error       TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_push_logs_license_key ON push_logs (license_key);
CREATE INDEX idx_push_logs_created_at ON push_logs (created_at);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
