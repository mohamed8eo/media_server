-- +goose Up
ALTER TABLE files ADD COLUMN playback_progress INTEGER DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_files_playback_progress ON files(user_id, playback_progress DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_files_playback_progress;
-- SQLite cannot drop a column on older supported versions; preserve playback_progress on rollback.