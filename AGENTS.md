# Working agreement

Read this before changing anything. It applies to every contributor, human
or agent, and it is not advisory.

Two companion documents carry the detail:

- **[docs/design/ui-guidelines.md](docs/design/ui-guidelines.md)** — the
  rulebook for anything that renders. Read it in full before touching the
  frontend. It ends in a checklist; run through it before opening a PR.
- **[docs/design/2026-09-11-7dtd-panel-design.md](docs/design/2026-09-11-7dtd-panel-design.md)**
  — what the game's API actually does, verified against a live server.
  The API is poorly documented and its own OpenAPI spec omits two of the
  most useful parts. Trust this document over the spec.

---

## The one rule that is never bent

**The browser never talks to the game server.** The API token lives only in
the backend; every game call is proxied through the panel. There is no
exception, no "just this one endpoint", no debug mode that bypasses it. A
change that puts a game-server URL or token in front of the browser is
wrong however convenient it is.

## Architecture

- **Backend:** Go standard library. No web framework, no router dependency —
  `net/http` with Go 1.22 method+path matching is enough.
- **Storage:** SQLite via `modernc.org/sqlite`, which is pure Go. This is a
  hard requirement, not a preference: the image is built `CGO_ENABLED=0`
  for amd64 and arm64, and a cgo driver breaks both.
- **Frontend:** React, TypeScript, Vite, Tailwind, shadcn/ui, embedded into
  the binary with `go:embed`. One process serves the API and the UI.
- **Config is environment variables only.** No config file to mount. If a
  setting needs adding, it goes in `internal/config` with a documented
  default.
- **Per server, not global.** Everything the panel stores about a game
  server is scoped to that server's id. Two configured servers are two
  worlds with two sets of players; mixing them shows an operator one thing
  while they believe they are looking at another.

## Behaviour

- **Degrade, do not crash.** A failed poll leaves the cache alone. "Offline"
  takes several consecutive failures, never one. The UI holds its last
  known good reading through a blip and says how old it is.
- **Never assert what you do not know.** If a value could not be read, say
  so rather than rendering `0`. If a figure is inferred rather than
  reported, label it.
- **The operator decides.** This is software somebody installs on their own
  machine. Surface consequences — tiers, warnings, what a command will
  run — but do not invent prohibitions. `PANEL_ALLOW_DESTRUCTIVE` is the
  operator's switch and it is honoured everywhere; there is no second,
  cleverer gate.
- **A command runs what its author wrote.** Text a player typed may be
  substituted into a console line only as plain words — never in a way that
  could add a quote, a newline or a second command. This is correctness,
  not permission: if a player can reshape the line, they have become its
  author.

## Verifying

This project's credibility rests on its claims being true.

- **Verify against a real server.** The dev stack is Vite on `:5173` and
  the Go API on `:8099`. Do not start a second instance on another port.
  Never restart Vite — HMR handles the frontend. Restart only the Go
  process, and say so.
- **Say what you could not verify.** Every design document here marks its
  unverified claims. Do the same in PR descriptions. "I could not prove X"
  is worth more than silence.
- **Drive the real interface.** Do the thing the way an operator would —
  through the panel's own pages, not `curl` and not the console page as a
  shortcut. Reach for the API only to confirm what the UI did, or to probe
  the game for something the panel does not surface yet. If a task is
  awkward through the UI, that awkwardness is the finding.
- **Screenshot what you changed.** A frontend change is not done because it
  compiles.

## Code

- **No file over 200 lines.** Split by responsibility — a page into its
  sections, a Go file into the part that decides and the part that acts.
  Generated code (`internal/sdtd/gen/`) and vendored shadcn components
  (`web/src/components/ui/`) are exempt.
- **Comments explain why, not what.** The diff already shows what. A
  comment earns its place by recording a decision, a constraint the code
  cannot state, or a thing that was tried and did not work.
- **Comment sparingly.** A comment above every block buries the few that
  matter. Before committing, reread each one and ask what a reader loses
  if it goes; if the answer is nothing, delete it.
- **Anything over one line is a block comment** — `/* … */`, not a stack
  of `//`. Go and TypeScript alike.
- **Tests pin behaviour that matters**, and their names say what they
  protect: `TestTheBotIgnoresItsOwnServersBroadcasts`, not `TestParse2`.
  Where a format came off a live server, quote it in the test.
- **Check before claiming.** `go vet ./... && go test ./...` for Go;
  `npm run typecheck` and `npx vp check` in `web/`. CI runs `vp check`,
  which includes formatting — `vp lint` alone will not catch it.

## Git

- **Conventional commits**, imperative mood, subject line at most 50
  characters, body wrapped at 72. The body says why. These are not
  decoration: the release version and the changelog are worked out from
  them, so a `feat:` that should have been a `fix:` cuts a minor release.
  See [docs/releasing.md](docs/releasing.md).
- **No AI attribution anywhere.** Not in commit messages, not in PR
  descriptions, not in comments. No `Co-Authored-By` for a tool, no
  "generated with" line, no mention of which assistant wrote something.
  This holds even if tooling suggests otherwise.
- **No "Test plan" section** in PR descriptions. Describe what was
  verified, in prose, including what was not.
- Branch names are `type/description`, lowercase, hyphenated.

## Voice

The panel talks like somebody who knows the game and respects the reader.
Sentence case. Plain words. Say what happens rather than naming the
control. Never invent official-looking game vocabulary — where the panel
uses its own word for something the game only numbers, the number stays
beside it.

The same voice applies to comments, commit messages and PR descriptions.
