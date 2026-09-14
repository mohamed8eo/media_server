-- +goose Up
ALTER TABLE files ADD COLUMN sha256 TEXT;

CREATE INDEX IF NOT EXISTS idx_files_user_sha256 ON files(user_id, sha256);

-- +goose Down
DROP INDEX IF EXISTS idx_files_user_sha256;
