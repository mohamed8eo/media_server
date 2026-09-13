-- +goose Up
ALTER TABLE jobs ADD COLUMN user_id TEXT;

-- +goose Down
