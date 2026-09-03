-- +goose Up
ALTER TABLE appointments ADD COLUMN contact_channel TEXT;

ALTER TABLE appointments ADD COLUMN contact_address TEXT;

UPDATE appointments
SET
    contact_channel = 'sms',
    contact_address = contact
WHERE
    contact IS NOT NULL;

ALTER TABLE appointments DROP COLUMN contact;

ALTER TABLE appointments
ADD CONSTRAINT contact_channel_check CHECK (contact_channel IN ('sms'));

ALTER TABLE appointments
ADD CONSTRAINT contact_both_or_neither CHECK (
    (contact_channel IS NULL) = (contact_address IS NULL)
);

-- +goose Down
ALTER TABLE appointments DROP CONSTRAINT contact_both_or_neither;

ALTER TABLE appointments DROP CONSTRAINT contact_channel_check;

ALTER TABLE appointments ADD COLUMN contact TEXT;

UPDATE appointments
SET
    contact = contact_address
WHERE
    contact_address IS NOT NULL;

ALTER TABLE appointments DROP COLUMN contact_channel;

ALTER TABLE appointments DROP COLUMN contact_address;