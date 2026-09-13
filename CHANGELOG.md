# Changelog

Notable changes, newest first. Written by hand before each release, and used
as the release notes when the tag is pushed.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions are `0.MINOR.PATCH` while this is pre-1.0; see
[docs/releasing.md](docs/releasing.md).

## [Unreleased]

<!-- Add entries here as things land. At release time, rename this heading to
     the version and open a fresh Unreleased above it. -->

## [0.1.0] - 2026-09-13

### Added

- Dashboard, players, world controls, live console, event feed and all 287
  game settings, grouped and labelled with the game's own words.
- Chat commands: the panel answers players in game. `!help`, `!day`,
  `!bloodmoon`, `!players` and `!kit <name>`, plus any command an admin
  writes. Each is off until switched on, with its own audience and cooldown.
- Kits: a named basket of items, built from the item picker and handed over
  by the bot.
- Automation: ten triggers, including one no ordinary scheduler can express —
  a warning a set time before the blood moon, worked out from the world's own
  day length.
- Game servers are added, edited and switched from the panel, with a
  connection test that tells a wrong token apart from an unreachable host.
- Several game servers at once, each fully separate.
- A live map: the game's own tiles proxied through the panel, with players,
  land claims, zombies and animals drawn over them. Right-click to teleport
  somebody or spawn something. Tiles are cached and refreshed in place, so a
  pan costs one round trip and a refresh does not blink.
- Stopping a server, with countdown warnings and a save first, and a watch
  afterwards that says whether it came back.
- A layout that works on a phone, not only a desk.
- A single multi-arch image, and a one-paste compose file.

### Fixed

- A game server refusing the panel's token is reported as such. The health
  poll uses an endpoint that needs no credentials, so a wrong token used to
  read as a healthy server until somebody tried to do something.
- A task set to run every so many minutes now runs. It counted from its last
  run, and nothing started the clock, so it stayed at zero and never came due.
