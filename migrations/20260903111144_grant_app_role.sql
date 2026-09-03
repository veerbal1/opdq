-- +goose Up
GRANT USAGE ON SCHEMA public TO opd_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON clinics, doctors, sessions, appointments, staff_users, auth_sessions
    TO opd_app;

GRANT SELECT, INSERT ON appointment_events TO opd_app;

GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO opd_app;

-- +goose Down
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM opd_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM opd_app;
REVOKE USAGE ON SCHEMA public FROM opd_app;