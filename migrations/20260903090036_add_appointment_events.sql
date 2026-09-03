-- +goose Up
ALTER TABLE appointments ADD CONSTRAINT appointments_clinic_id_id_key UNIQUE (clinic_id, id);
CREATE TABLE appointment_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id BIGINT NOT NULL,
    appointment_id BIGINT NOT NULL,
    from_state TEXT,
    to_state TEXT NOT NULL,
    actor_id BIGINT REFERENCES staff_users (id),
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT,
    FOREIGN KEY (clinic_id, appointment_id) REFERENCES appointments (clinic_id, id)
);

CREATE INDEX idx_appointment_events_appointment_at ON appointment_events (appointment_id, at);

-- +goose Down
DROP TABLE IF EXISTS appointment_events;
DROP INDEX IF EXISTS idx_appointment_events_appointment_at;
ALTER TABLE appointments DROP CONSTRAINT appointments_clinic_id_id_key;