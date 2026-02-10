-- Per-trunk recording configuration
ALTER TABLE trunks ADD COLUMN recording_mode TEXT NOT NULL DEFAULT 'off';
