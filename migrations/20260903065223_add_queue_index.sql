-- +goose Up
CREATE INDEX idx_appointments_queue
    ON appointments (session_id, state, priority DESC, queued_at ASC);

-- +goose Down
DROP INDEX idx_appointments_queue;
