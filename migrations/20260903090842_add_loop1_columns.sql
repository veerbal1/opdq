-- +goose Up
ALTER TABLE appointments ADD COLUMN public_id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid();
ALTER TABLE appointments ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE appointments ADD COLUMN completed_at TIMESTAMPTZ;
ALTER TABLE appointments ADD COLUMN version INT NOT NULL DEFAULT 1;

ALTER TABLE sessions ADD COLUMN avg_consult_sec INT NOT NULL DEFAULT 480;

ALTER TABLE clinics ADD COLUMN timezone TEXT NOT NULL DEFAULT 'Asia/Kolkata';

-- +goose Down
ALTER TABLE clinics DROP COLUMN timezone;

ALTER TABLE sessions DROP COLUMN avg_consult_sec;

ALTER TABLE appointments DROP COLUMN version;
ALTER TABLE appointments DROP COLUMN completed_at;
ALTER TABLE appointments DROP COLUMN started_at;
ALTER TABLE appointments DROP COLUMN public_id;