-- +goose Up
CREATE TABLE auth_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    clinic_id BIGINT NOT NULL,
    csrf_token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (clinic_id, user_id) REFERENCES staff_users (clinic_id, id)
);

CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_sessions;
