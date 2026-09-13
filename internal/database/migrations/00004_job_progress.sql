-- +goose Up
CREATE TABLE jobs_new (
    id TEXT PRIMARY KEY,
    file_id TEXT,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

INSERT INTO jobs_new (id, file_id, task_type, status, error, created_at, updated_at)
SELECT id, file_id, task_type, status, error, created_at, updated_at FROM jobs;

DROP TABLE jobs;
ALTER TABLE jobs_new RENAME TO jobs;

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

-- +goose Down
