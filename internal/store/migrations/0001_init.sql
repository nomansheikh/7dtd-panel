-- Panel-local state only. Nothing here mirrors game server data; the game
-- server is the source of truth for anything about the world or its players.

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
) STRICT;

-- Sessions store a SHA-256 of the token, never the token itself, so a stolen
-- database cannot be replayed as a live session.
CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    user_agent TEXT    NOT NULL DEFAULT '',
    ip         TEXT    NOT NULL DEFAULT ''
) STRICT;

CREATE INDEX sessions_expires_at ON sessions (expires_at);
CREATE INDEX sessions_user_id ON sessions (user_id);
