-- +goose Up
-- Native clients sign in as named devices with bearer tokens; browsers keep the
-- cookie. Both are rows in sessions, now with a public id so a user can list and
-- revoke them.
ALTER TABLE sessions ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE sessions ADD COLUMN kind text NOT NULL DEFAULT 'browser'
    CHECK (kind IN ('browser', 'device'));
ALTER TABLE sessions ADD COLUMN device_name text NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN platform text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX sessions_id_idx ON sessions (id);

-- A TV (or any device without a keyboard) asks to be paired, shows user_code, and
-- polls with the device code until a signed-in user approves it (RFC 8628 style).
CREATE TABLE device_pairings (
    device_code_hash bytea PRIMARY KEY,
    user_code text NOT NULL UNIQUE,
    device_name text NOT NULL,
    platform text NOT NULL,
    approved_by bigint REFERENCES users (id) ON DELETE CASCADE,
    denied boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    polled_at timestamptz
);
CREATE INDEX device_pairings_expires_at_idx ON device_pairings (expires_at);

-- One-time codes a signed-in web user shows as a QR so a phone can sign in.
CREATE TABLE connect_codes (
    code_hash bytea PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX connect_codes_expires_at_idx ON connect_codes (expires_at);

-- +goose Down
DROP TABLE connect_codes;
DROP TABLE device_pairings;
DROP INDEX sessions_id_idx;
ALTER TABLE sessions DROP COLUMN platform;
ALTER TABLE sessions DROP COLUMN device_name;
ALTER TABLE sessions DROP COLUMN kind;
ALTER TABLE sessions DROP COLUMN id;
