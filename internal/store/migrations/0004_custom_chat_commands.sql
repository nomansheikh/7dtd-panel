-- Commands an admin writes, alongside the ones the panel ships with.
--
-- The first cut fixed the command set at compile time so that nothing written
-- in a table could invent a new power for a stranger in chat. That is the
-- wrong call for software somebody else installs on their own machine: the
-- panel already has a console page that runs anything typed into it, so
-- refusing to let the same person put the same command behind a chat trigger
-- was a rule that existed nowhere else in the product.
--
-- What replaces it is not a smaller set of powers but a louder one. The panel
-- shows what tier a command is before it is switched on, and PANEL_ALLOW_
-- DESTRUCTIVE — the operator's own switch, already honoured by the console and
-- the world page — is honoured here too. The decision is theirs; the panel's
-- job is to make sure they can see what they are deciding.
--
-- Same table as the built-ins, so one name cannot mean two things and the bot
-- has one place to look.

-- 'builtin' for the commands in internal/chat, 'custom' for these. A built-in
-- keeps its behaviour in code and uses only the enabled/audience/cooldown
-- columns; a custom command carries its whole behaviour in the columns below.
ALTER TABLE chat_commands ADD COLUMN kind TEXT NOT NULL DEFAULT 'builtin';

-- What it does, in the admin's words. Shown in the panel and in !help, because
-- a custom command has no doc comment to read it out of.
ALTER TABLE chat_commands ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- What the player is told. On its own this makes a command that only answers:
-- !discord, !rules, !ip. Optional — a command may act and say nothing.
ALTER TABLE chat_commands ADD COLUMN reply TEXT NOT NULL DEFAULT '';

-- Console command lines to run, as a JSON array, in order.
--
-- A list rather than one line because the useful ones are several: a starter
-- package is a give per item, and handing somebody a vehicle is a spawn and a
-- teleport. The game's console takes one command at a time, so the panel is
-- what turns a list into a sequence.
ALTER TABLE chat_commands ADD COLUMN commands TEXT NOT NULL DEFAULT '[]';
