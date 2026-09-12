# Releasing

You decide when, and what the number is. Pushing the tag is the only
irreversible act; everything after it is automatic.

## The flow

```bash
# 1. write the changelog entry, merge it like anything else
# 2. tag main
git tag -a v0.1.0-beta.1 -m "v0.1.0-beta.1"
git push origin v0.1.0-beta.1
```

That is the whole thing. The tag triggers a build of both architectures, a
push to the registry, and a GitHub release whose notes come from the
matching `CHANGELOG.md` section.

## Choosing the number

Pre-1.0, so `0.MINOR.PATCH`. Under a zero major, SemVer says anything may
change — the number is a promise about effort, not about stability.

| What landed | Bump | Example |
| --- | --- | --- |
| A feature somebody will notice | minor | `0.1.4` → `0.2.0` |
| Fixes only | patch | `0.1.4` → `0.1.5` |
| A breaking change | minor, while the major is zero | `0.1.4` → `0.2.0` |
| Docs, refactors, tidying | no release | — |

A release nobody can notice is noise in somebody's upgrade log. If the
changelog entry would be empty, do not cut one.

### Going to 1.0

When you would defend three things as stable: the environment variables,
the panel's own HTTP routes, and the shape of the database. Not "when it is
finished" — when *changing* those would be something you would feel bad
about. After that, breaking any of them means a major.

## What gets published

| Tag | Moves | Use it when |
| --- | --- | --- |
| `0.2.0` | never | you want exactly this build, forever |
| `0.2` | on every patch | **recommended** — fixes arrive, surprises do not |
| `latest` | on every release, never on a prerelease | trying it out |

A prerelease — `v0.2.0-rc.1` — publishes to the registry and is marked as a
prerelease on GitHub, without moving `latest`. That is how you hand somebody
a build to test without changing what everyone else pulls.

## Upgrading, and why downgrading does not work

Migrations are forward-only. There are no down scripts, by design: an undo
that has never been run is a lie, and the panel's data is small enough that
a copy of the file is a better answer.

An older image meeting a newer database will not crash — it applies only the
migrations it has not seen — but anything the newer version wrote is
invisible to it.

**Back up `panel.db` before upgrading.** It is one file:

```bash
docker compose stop panel
docker run --rm -v panel-data:/data -v "$PWD":/out alpine \
  cp /data/panel.db /out/panel.db.backup
docker compose up -d
```

## When a release goes wrong

Fix forward. Do not delete a tag and do not move one: a tag that pointed at
one image and now points at another is worse than a bad release, because
nobody can tell what they are running.

If an image must not be used, edit its GitHub release and mark it a
prerelease. That takes `latest` off it and keeps it out of the repository
page, without breaking anybody who pinned the exact version.
