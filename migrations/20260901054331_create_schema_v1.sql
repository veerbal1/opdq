-- +goose Up
CREATE TABLE clinics (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    public_id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid ()
);

CREATE TABLE doctors (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    clinic_id BIGINT NOT NULL REFERENCES clinics (id),
    UNIQUE (clinic_id, id)
);

CREATE TABLE sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id BIGINT NOT NULL REFERENCES clinics (id),
    doctor_id BIGINT NOT NULL REFERENCES doctors (id),
    session_date DATE NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    capacity INT NOT NULL,
    delay_min INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (
        status IN (
            'scheduled',
            'active',
            'completed',
            'cancelled'
        )
    ),
    version INT NOT NULL DEFAULT 1,
    UNIQUE (clinic_id, id),
    FOREIGN KEY (clinic_id, doctor_id) REFERENCES doctors (clinic_id, id)
);

CREATE TABLE appointments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id BIGINT NOT NULL,
    session_id BIGINT NOT NULL,
    token_no INT NOT NULL,
    patient_name TEXT NOT NULL,
    contact TEXT,
    queued_at TIMESTAMPTZ NOT NULL,
    priority INT NOT NULL DEFAULT 0 CHECK (priority IN (0, 1)),
    state TEXT NOT NULL CHECK (
        state IN (
            'waiting',
            'in_consultation',
            'done',
            'absent'
        )
    ),
    UNIQUE (session_id, token_no),
    FOREIGN KEY (clinic_id, session_id) REFERENCES sessions (clinic_id, id)
);

-- +goose Down
DROP TABLE IF EXISTS appointments;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS doctors;
DROP TABLE IF EXISTS clinics;