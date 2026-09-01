-- +goose Up
ALTER TABLE sessions DROP CONSTRAINT sessions_status_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_status_check CHECK (status IN ('open', 'cancelled'));
ALTER TABLE sessions ALTER COLUMN status SET DEFAULT 'open';

-- +goose Down
ALTER TABLE sessions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE sessions DROP CONSTRAINT sessions_status_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_status_check
    CHECK (status IN ('scheduled', 'active', 'completed', 'cancelled'));
