# Contributing

Thanks for considering a contribution. This document covers how to get the
project running, the branch and commit conventions, and what a reviewable
pull request looks like.

The project is in early development. The architecture and the reasoning
behind it live in
[`docs/design/`](docs/design/2026-09-11-7dtd-panel-design.md) — read that
first if you are making anything more than a small change. It also records
which parts of the game server's API are verified and which are not, which
saves a lot of rediscovery.

## Ground rules

Two conventions matter more here than in a typical project, because the
thing we integrate with is poorly documented:

1. **Verify against a real server.** The 7 Days to Die web API has
   undocumented endpoints, endpoints documented but read-only, and fields
   that are permanently `null`. If you add or change a call, exercise it
   against an actual server and include the output in your pull request.
2. **Say what you could not verify.** "I could not test this because I had
   no second player" is a genuinely useful review note. Claiming something
   works when it was never run is not. Unverified behaviour should be
   called out in the PR and, where it affects users, in the code or README.

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | as pinned in `go.mod` | `CGO_ENABLED=0` builds only — do not introduce cgo dependencies |
| Node | `^20.19` \|\| `^22.18` \|\| `>=24.11` | required by Vite+ |
| Docker | any recent | only needed to build or test the image |

A 7 Days to Die dedicated server with the Allocs webinterface mod and the
web dashboard enabled. It does not need to be on the same machine — the
panel is designed for a separate host.

## Getting set up

```bash
git clone https://github.com/nomansheikh/7dtd-panel.git
cd 7dtd-panel
cp .env.example .env      # then fill in your server details
go mod download
npm install               # in web/
```

Run the backend and the frontend dev server separately while developing;
the production image serves the built frontend from the Go binary.

**Never commit credentials.** `.env` is gitignored. The game server's API
token and your panel admin password belong there and nowhere else. If you
paste server output into an issue or PR, redact the `X-SDTD-API-SECRET`
header and anything from `/api/webapitokens`, which returns token secrets
in plaintext.

## Regenerating the API client

The typed game-server client is generated, not hand-written. The upstream
OpenAPI spec is split across 29 files, declares OpenAPI 3.1, uses a
`BASEPATH` placeholder in three sub-specs, and contains four outright bugs.
`tools/specbundle` flattens, patches and downconverts it:

```bash
# Reproducible: rebuild from the vendored snapshot.
go run ./tools/specbundle -from api/snapshot -out api

# Refresh the snapshot from a live server first, then rebuild.
go run ./tools/specbundle -from http://<host>:8080 -out api
oapi-codegen -config api/codegen.yaml api/openapi.bundled.yaml
```

Both `api/snapshot/` and `api/openapi.bundled.yaml` are committed
deliberately, so a clean checkout builds offline and any upstream drift
shows up as a reviewable diff.

Each known upstream bug is an **addressed patch** in
`tools/specbundle/patch.go`. If a patch stops matching, the bundler fails
rather than warning: that means upstream changed, and a human should decide
whether the fix is still needed. Do not "fix" this by editing
`api/snapshot/` — those files are upstream's, verbatim.

## Workflow

`main` is protected. It requires a pull request and an approving review, and
does not accept force pushes. Work on a branch, or on a fork if you do not
have write access.

```bash
git checkout main && git pull
git checkout -b feat/short-description
# ... work, commit ...
git push -u origin feat/short-description
gh pr create
```

### Branch names

`type/short-description`, lowercase, hyphen-separated.

Valid types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`,
`perf`, `style`.

```
feat/players-table
fix/sse-reconnect-gap
docs/add-contributing-guide
```

### Commits

[Conventional Commits](https://www.conventionalcommits.org/). The subject
is imperative, lowercase, no trailing period, and 50 characters or fewer
including the type and scope.

```
feat(console): build command palette from server list

Fetch the catalogue from GET /api/command rather than hardcoding
names, so the palette reflects what the server actually accepts
and shows its own help text.
```

Include a body whenever the *why* is not obvious from the subject, the
change spans several files, or you made a tradeoff worth recording. Wrap the
body at 72 characters.

Do not add tool-attribution trailers (`Generated with …`, `Co-Authored-By`
for tooling). Credit humans, not editors.

### Pull requests

- Keep a PR to one logical change. Unrelated fixes belong in separate PRs,
  even small ones.
- Fill in the template. The **Verification** section is the one reviewers
  actually rely on.
- Rebase or merge `main` in to resolve conflicts; do not force-push over a
  reviewer's in-progress review without saying so.
- CI must pass: `go vet`, `go test ./...`, the linter, frontend type-check
  and tests.

## Tests

```bash
go test ./...          # backend
go vet ./...
npm run test           # frontend (Vitest via Vite+)
npm run check          # type-check and lint
```

Backend tests are table-driven, per Go convention, and run against a fake
7 Days to Die server built on `httptest` — `go test ./...` must never
require a real server or network access. Fixtures in `testdata/` are real
captured responses; if you need a new one, capture it from a live server
rather than writing it by hand, and redact any secrets.

### Integration tests

Tests named `TestIntegration*` run against an actual game server and **skip
unless `SDTD_HOST` is set**, so the default `go test ./...` stays offline:

```bash
SDTD_HOST=10.0.0.5 \
SDTD_API_TOKEN_NAME=panel \
SDTD_API_TOKEN_SECRET=... \
  go test ./internal/sdtd -run Integration -v
```

They assert on shape, not on values, since the world changes between runs.
Their job is to catch the client disagreeing with a real server. Keep them
read-only — `gettime` is fine, `settime` is not — so they are safe to point
at a live server.

If you change anything that talks to the game API, run these and paste the
output in your PR.

## Reporting bugs and proposing features

Use the issue templates. For a bug, the game version and mod versions
(`version` in the console, or `GET /api/command` → `version`) are almost
always the first thing a maintainer needs.

## Security

Do not open a public issue for a vulnerability. Use GitHub's private
[security advisory](https://github.com/nomansheikh/7dtd-panel/security/advisories/new)
form.

Be aware of the threat model when proposing changes: the panel holds an API
token that grants full administrative control of a game server, and the
console endpoint executes arbitrary commands. The browser must never talk to
the game server directly, and user-controlled strings must never be
interpolated into console commands — see the security section of the design
doc.

## Licence

By contributing you agree that your contributions are licensed under the
MIT Licence that covers this project.
