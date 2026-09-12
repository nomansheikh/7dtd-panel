# Changelog

Notable changes, newest first. Written by hand before each release, and used
as the release notes when the tag is pushed.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions are `0.MINOR.PATCH` while this is pre-1.0; see
[docs/releasing.md](docs/releasing.md).

## [Unreleased]

<!-- Add entries here as things land. At release time, rename this heading to
     the version and open a fresh Unreleased above it. -->

## [0.1.0] - unreleased

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
- A single multi-arch image, and a one-paste compose file.
