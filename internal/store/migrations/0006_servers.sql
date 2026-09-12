-- Game servers, added from the panel rather than from the environment.
--
-- Until now a server existed only as environment variables, which suits a
-- machine managed by Ansible and suits nobody else: adding one meant editing a
-- compose file and restarting the container, and a typo in the token looked
-- exactly like a server that was switched off.
--
-- The environment still works. On first boot anything configured there is
-- copied in here once, so an existing install upgrades without noticing and a
-- config-as-code setup keeps working. After that this table is the truth.
CREATE TABLE game_servers (
    -- The id that appears in URLs. Lower case, digits and dashes.
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL,

    host   TEXT    NOT NULL,
    port   INTEGER NOT NULL DEFAULT 8080,
    scheme TEXT    NOT NULL DEFAULT 'http',

    -- The web API token from the game server's own `webtokens add`.
    --
    -- Stored as given. It is not encrypted, and pretending otherwise would be
    -- worse than saying so: the key would have to live in the environment,
    -- which is where the token used to live, and losing it would lose every
    -- server. What this does mean is that the panel's database now holds
    -- credentials for every game server it manages — so it belongs wherever
    -- the compose file holding the same secret already belonged, and backups
    -- of it deserve the same care.
    token_name   TEXT NOT NULL,
    token_secret TEXT NOT NULL,

    -- Lowest first, so an operator can decide which server the UI opens on
    -- without renaming anything.
    position INTEGER NOT NULL DEFAULT 0,

    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;
