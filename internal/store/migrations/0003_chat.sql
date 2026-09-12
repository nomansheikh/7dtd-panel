-- Chat commands let the panel answer players in game, which the game server
-- has no notion of: it streams chat out and takes commands in, and nothing
-- joins the two. The panel sits in the middle already, so it is the only thing
-- that can.

-- What each command is allowed to do, on which server, and by whom.
--
-- Scoped to a server for the same reason every other feature here is: two
-- configured servers are two different worlds with two different sets of
-- players, and a public one and a private one should not have to share a
-- policy on who may summon loot.
--
-- A row per command rather than a single settings blob, so an operator can
-- turn one off without touching the rest, and so a command added in a later
-- version arrives disabled rather than live.
CREATE TABLE chat_commands (
    server_id TEXT NOT NULL,
    name      TEXT NOT NULL,
    -- Off by default. A panel that starts answering strangers the moment it is
    -- upgraded is a surprise, and surprises on a game server are expensive.
    enabled INTEGER NOT NULL DEFAULT 0,
    -- 'everyone' or 'admins'. Who an admin is comes from the game's own admin
    -- list, not from panel logins: the people typing in chat have game
    -- identities, not panel accounts.
    audience TEXT NOT NULL DEFAULT 'everyone',
    -- Seconds one player must wait before running this again. Per command,
    -- because a kit worth an hour and a countdown worth five seconds are not
    -- the same kind of thing.
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (server_id, name)
) STRICT;

-- When each player last used each command.
--
-- Kept in the database rather than in memory so a cooldown survives a panel
-- restart. Otherwise the way around an hourly kit is to wait for a deploy.
CREATE TABLE chat_cooldowns (
    server_id   TEXT NOT NULL,
    platform_id TEXT NOT NULL,
    command     TEXT NOT NULL,
    used_at     INTEGER NOT NULL,
    PRIMARY KEY (server_id, platform_id, command)
) STRICT;

-- Kits: a named basket of items.
--
-- Moved here from the browser because two different things now need them. An
-- admin builds one at a keyboard, and the bot hands one over at three in the
-- morning when nobody has the panel open — which rules out local storage.
--
-- Not scoped to a server, unlike the rules above. A kit is a list of item
-- names, and the items are the game's, not one server's; an operator who built
-- a starter kit once should not have to build it again for the test box.
CREATE TABLE kits (
    name TEXT PRIMARY KEY,
    -- The basket as JSON: an array of {item, count, quality}. A column per
    -- field would need a second table and a join to answer "what is in this
    -- kit", which is the only question anybody asks of it.
    items      TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;
