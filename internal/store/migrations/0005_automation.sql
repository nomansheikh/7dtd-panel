-- Things the panel does on its own, without anybody at a keyboard.
--
-- The game server has no scheduler and no notion of "before the blood moon".
-- It runs a world and answers questions about it; anything that has to happen
-- at a time, or in response to something, has to be somebody watching. The
-- panel is already watching — it polls the clock and holds the event stream —
-- so it is the thing that can stop that somebody being a person.
--
-- Deliberately the same shape as a custom chat command: a name, a list of
-- console lines, and a switch. The difference is only what sets it off. An
-- operator who has written one already knows how to write the other.
CREATE TABLE automation_tasks (
    server_id TEXT NOT NULL,
    name      TEXT NOT NULL,
    -- Off by default, like everything else here. A task that starts running
    -- the moment it is saved gives no chance to read it back first.
    enabled     INTEGER NOT NULL DEFAULT 0,
    description TEXT    NOT NULL DEFAULT '',

    -- What sets it off: every, daily, bloodmoon or join.
    trigger_kind TEXT NOT NULL,
    -- Minutes, read according to the kind: the interval for 'every', and how
    -- long before the horde arrives for 'bloodmoon'.
    trigger_minutes INTEGER NOT NULL DEFAULT 0,
    -- "HH:MM" for 'daily', in the panel's own timezone. Empty otherwise.
    trigger_at TEXT NOT NULL DEFAULT '',

    -- Console lines to run, in order, as a JSON array.
    commands TEXT NOT NULL DEFAULT '[]',

    -- When it last ran, so a panel that was restarted does not re-run a daily
    -- task it already did this morning, and does not skip one it missed.
    last_run_at INTEGER NOT NULL DEFAULT 0,
    -- What that run was for, when "not since last time" is not enough to tell:
    -- a blood moon task records which blood moon it fired for, so it goes off
    -- once per horde rather than once per poll for the half hour before one.
    last_key TEXT NOT NULL DEFAULT '',

    updated_at INTEGER NOT NULL,
    PRIMARY KEY (server_id, name)
) STRICT;

-- A record of what ran, so an operator can see what the panel did overnight.
--
-- Kept here rather than only in the log because the log is the game server's
-- and scrolls past; this is the panel answering "did the restart happen".
CREATE TABLE automation_runs (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT    NOT NULL,
    name      TEXT    NOT NULL,
    ran_at    INTEGER NOT NULL,
    -- Empty when every line ran. Otherwise what went wrong, already safe to
    -- show to a logged-in operator.
    error TEXT NOT NULL DEFAULT ''
) STRICT;

CREATE INDEX automation_runs_recent ON automation_runs (server_id, ran_at DESC);
