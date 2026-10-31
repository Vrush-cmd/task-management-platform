-- Creates the tasks table. Docker Compose mounts this directory into the
-- PostgreSQL container, so it runs automatically the first time the database
-- volume is created. To apply it manually:
--   psql "$DATABASE_URL" -f migrations/001_create_tasks.sql

CREATE TABLE IF NOT EXISTS tasks (
    id          UUID         PRIMARY KEY,
    title       VARCHAR(200) NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description TEXT         NOT NULL DEFAULT '',
    completed   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks (created_at DESC);
