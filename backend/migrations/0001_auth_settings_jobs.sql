-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username citext NOT NULL UNIQUE,
    display_name text NOT NULL DEFAULT '',
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'member')),
    disabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    user_agent text NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE jobs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'done', 'failed', 'cancelled')),
    priority int NOT NULL DEFAULT 0,
    run_at timestamptz NOT NULL DEFAULT now(),
    attempts int NOT NULL DEFAULT 0,
    max_attempts int NOT NULL DEFAULT 3,
    progress smallint NOT NULL DEFAULT 0,
    last_error text,
    claimed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);
CREATE INDEX jobs_pending_idx ON jobs (status, run_at, priority DESC) WHERE status = 'pending';

-- +goose Down
DROP TABLE jobs;
DROP TABLE settings;
DROP TABLE sessions;
DROP TABLE users;
DROP EXTENSION pg_trgm;
DROP EXTENSION citext;
