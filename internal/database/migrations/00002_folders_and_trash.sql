-- +goose Up
ALTER TABLE files ADD COLUMN deleted_at TIMESTAMP;

CREATE TABLE IF NOT EXISTS folders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    path TEXT NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, path)
);
INSERT OR IGNORE INTO folders (id, user_id, path)
SELECT lower(hex(randomblob(16))), user_id, folder FROM files WHERE folder <> '/';

CREATE INDEX IF NOT EXISTS idx_files_user_deleted ON files(user_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_folders_user_deleted ON folders(user_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_folders_user_path ON folders(user_id, path);

-- +goose Down
DROP INDEX IF EXISTS idx_folders_user_path;
DROP INDEX IF EXISTS idx_folders_user_deleted;
DROP INDEX IF EXISTS idx_files_user_deleted;
DROP TABLE IF EXISTS folders;
-- SQLite cannot drop a column on older supported versions; preserve deleted_at on rollback.
