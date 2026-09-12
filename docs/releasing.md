# Releasing

Releases are driven by commit messages. Nobody decides a version number by
hand, and nobody remembers to tag.

## How a release happens

1. You merge pull requests to `main` with conventional commit subjects.
2. `release-please` keeps a pull request open titled something like
   **chore(main): release 0.2.0**. It holds the version bump and the
   changelog entries for everything merged since the last release.
3. Merging that pull request *is* the release. It tags `v0.2.0`, writes the
   GitHub release notes, and builds and pushes the image.

There is nothing else to run. If the release PR does not exist, no commit
since the last release warranted one.

## What decides the number

The project is pre-1.0, so `0.MINOR.PATCH`. Under a zero major, SemVer says
anything may change — the number is a promise about effort, not stability.

| Commit | Bump | Example |
| --- | --- | --- |
| `feat:` | minor | `0.1.4` → `0.2.0` |
| `fix:`, `perf:` | patch | `0.1.4` → `0.1.5` |
| `feat!:` or `BREAKING CHANGE:` | minor, while under 1.0 | `0.1.4` → `0.2.0` |
| `docs:`, `refactor:`, `ci:`, `chore:`, `test:`, `style:` | none | no release |

`refactor:` and `docs:` appear in the changelog but do not on their own
cause a release, because a release that contains nothing a user can notice
is noise in somebody's upgrade log.

### Going to 1.0

When you would defend three things as stable: the environment variables,
the panel's own HTTP routes, and the shape of the database. Not "when it is
finished" — when *changing* those would be something you would feel bad
about. After that, `feat!` means a major bump and the promise is real.

## Tags that get published

| Tag | Moves | Use it when |
| --- | --- | --- |
| `0.2.0` | never | you want exactly this build, forever |
| `0.2` | on every patch | **recommended** — fixes arrive, surprises do not |
| `latest` | on every release, never on a prerelease | trying it out |

A prerelease — `v0.2.0-rc.1` — publishes to the registry without moving
`latest`. That is what makes it safe to hand somebody a build to test
without changing what everyone following the README gets.

## Upgrading, and why downgrading does not work

Migrations are forward-only. There are no down scripts, by design: an
undo that has never been run is a lie, and the panel's data is small enough
that a copy of the file is a better answer.

So an older image meeting a newer database will not crash — it applies only
migrations it has not seen, and ignores columns it does not know — but
anything written by the newer version is invisible to it, and writes may
fail against tightened constraints.

**Back up `panel.db` before upgrading.** It is one file:

```bash
docker compose stop panel
docker run --rm -v panel-data:/data -v "$PWD":/out alpine \
  cp /data/panel.db /out/panel.db.backup
docker compose up -d
```

## A release that goes wrong

Do not delete the tag or move it. Fix forward: merge the fix, let
`release-please` cut the next patch. A tag that once pointed at one image
and now points at another is worse than a bad release, because nobody can
tell which one they are running.

If an image must not be used, mark the GitHub release as a prerelease so
`latest` stops pointing at it, and say why in the release notes.
