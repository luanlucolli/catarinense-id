CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS apps (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    session_uuid UUID NOT NULL UNIQUE,
    last_active TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sessions_user_app_unique UNIQUE (user_id, app_id)
);

CREATE INDEX IF NOT EXISTS idx_apps_api_key_active
    ON apps (api_key)
    WHERE active = true;

CREATE INDEX IF NOT EXISTS idx_sessions_session_uuid
    ON sessions (session_uuid);

CREATE INDEX IF NOT EXISTS idx_sessions_user_app
    ON sessions (user_id, app_id);

ALTER TABLE users
    DROP COLUMN IF EXISTS session_uuid;
