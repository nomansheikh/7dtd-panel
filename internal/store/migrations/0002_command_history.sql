-- Console history is panel-local: the game server keeps no record of what an
-- operator typed, only of what it executed.
CREATE TABLE command_history (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    command    TEXT    NOT NULL,
    succeeded  INTEGER NOT NULL,
    -- Stored so the console can be reopened with its scrollback intact.
    result     TEXT    NOT NULL DEFAULT '',
    error      TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX command_history_user_created ON command_history (user_id, id DESC);
