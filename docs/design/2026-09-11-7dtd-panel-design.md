# 7dtd-panel — design

- **Status:** draft, awaiting review
- **Date:** 2026-09-11
- **Target:** `github.com/nomansheikh/7dtd-panel`, image `ghcr.io/nomansheikh/7dtd-panel`

A self-hostable web admin panel for 7 Days to Die dedicated servers. Go
backend, React + shadcn/ui frontend, one multi-arch Docker image.

Every API claim in this document was verified against a live server
(V 3.2.0 b10, mods TFP_Harmony + Allocs_Commands/Core/Webinterface) on
2026-09-11. Claims I could **not** verify are marked as such; see
[Unverified](#unverified) and [Not achievable](#not-achievable).

---

## 1. What the game server actually offers

This section exists because the API is poorly documented and two of its
most useful parts are absent from its own OpenAPI spec. Future
contributors should read this before trusting the spec.

### 1.1 Auth

Two headers on every request. Both are required together:

```
X-SDTD-API-TOKENNAME: <name>
X-SDTD-API-SECRET:    <secret>
```

Session cookie auth (`sid`) also exists via `/session/login`, but that
authenticates *game* web users, not panel users. The panel does not use
it.

Note: most read endpoints are **public** on a default install —
`/api/serverstats`, `/api/serverinfo`, `/api/player` and others sit at
permission level 2000 and answer with no credentials at all. The panel
still always sends its token, because the write endpoints and
`/api/log` require level 0–1000.

### 1.2 The OpenAPI spec is 29 files and needs repair

Root document:

```
GET /api/openapi/openapi.yaml
```

That exact path matters; `/openapi.json`, `/swagger.json`, `/api/docs`
and similar all 404. It was found by reading `spec-url` out of the
server's own `/rapidoc.html`.

The root document contains no inline operations. It is a manifest of
external `$ref`s to 28 sibling files, each fetched from:

```
GET /api/OpenAPI/{name}.openapi.yaml
```

Four things make it unusable by a generator as-is:

1. **External file refs.** Sub-specs reference the root
   (`'./openapi.yaml#/components/schemas/TypeVector3i'`) and the root
   references sub-specs. Needs flattening into one document.
2. **`BASEPATH` placeholder path keys.** The session sub-spec defines
   its paths as `/BASEPATH/login`, and the root rewrites them via
   `$ref: './SessionHandler.openapi.yaml#/paths/~1BASEPATH~1login'`.
   No standard resolver handles this; the bundler substitutes the real
   prefix from the root's own key.
3. **OpenAPI 3.1.** The spec declares `openapi: 3.1.0` and uses 3.1
   type arrays extensively (`type: ['string', 'null']` on `ip`,
   `ping`, `name`, `icon`, `help`, …). Tooling support for 3.1 is
   uneven, so the bundler emits **3.0.3**, converting type arrays to
   `type: X` + `nullable: true`.
4. **Outright bugs.** The bundler carries an explicit, commented patch
   list so a future spec refresh shows which fixes are still needed:

   | Location | Bug | Fix |
   | --- | --- | --- |
   | `SandboxSettings` | `oneof:` (lowercase) — silently ignored by every parser | `oneOf:` |
   | `SessionHandler` | `plain/text` media type | `text/plain` |
   | root `TypeUserIdString` | pattern `^\[a-zA-Z]+_[\w]+$` — the escaped `[` means it can never match | `^[a-zA-Z]+_[\w]+$` |
   | several | `$ref` with sibling `description` inside `responses` | drop the sibling |

### 1.3 Response envelope

Every JSON endpoint wraps its payload:

```json
{ "data": <payload>, "meta": { "serverTime": "...", "errorCode": "..." } }
```

Errors put a machine-readable code in `meta.errorCode`, not in `data`.
Verified examples:

| Request | Status | `meta.errorCode` |
| --- | --- | --- |
| `POST /api/command` `{"command":"nope"}` | 404 | `UNKNOWN_COMMAND` |
| `POST /api/command` `{}` | 400 | `NO_COMMAND` |
| `PUT /api/gameprefs` (no id) | 400 | `PUT_WITHOUT_ID` |
| `PUT /api/gameprefs/{name}` | 405 | `Unsupported` |

`meta` may also carry `exceptionMessage` and a full .NET
`exceptionTrace`. The panel logs traces server-side and never forwards
them to the browser.

### 1.4 Console commands — the main write path

```
POST /api/command   {"command": "settime 1 13 37", "format": "Full"}
```

`format` is `Full` (command + parameters + result), `Simple`
(`resultRaw` only) or `Raw` (`text/plain` body). All three verified.
The panel uses `Full`.

`GET /api/command` lists **199** commands, each with `overloads`,
`description`, `help`, and an `allowed` boolean reflecting the caller's
permission level. The command palette is built from this, never from a
hardcoded array. Help text is genuinely useful and is surfaced in the UI.

**Every game-state mutation goes through this endpoint.** `/api/gameprefs`,
`/api/sandboxsettings` and `/api/bloodmoon` are read-only — `PUT` to
them returns 405 `Unsupported`. The only REST endpoints with real write
methods are markers, blacklist, whitelist, userpermissions,
commandpermissions, webapitokens, webusers and registeruser.

### 1.5 There is a live event stream

The brief assumed polling. There is a real SSE stream; it is not in the
spec. `/sse/log` returns 400 rather than 404, which is what prompted
pulling the server's own SPA bundle
(`/files/static/js/main.6d2039c8.js`) and finding the call site:

```
GET /sse/?events=log      →  Content-Type: text/event-stream
                             event: logLine
                             data:  {"id":61,"msg":"…","type":"Log",
                                     "trace":null,"isotime":"…","uptime":"…"}
```

Verified streaming live. `data` matches the `/api/log` `LogEntry`
schema. Only `events=log` is valid — `chat`, `playerlist`, `players`
and `all` all return 400. Chat therefore arrives as ordinary log lines
and is classified downstream.

Two consequences the design accounts for:

- **The stream has no replay.** Reconnecting starts from "now", so
  lines during the gap are lost. But `/api/log` takes a `firstLine`
  cursor and returns `lastLine`, so the panel backfills the gap on every
  reconnect. See [§5.3](#53-gapless-log-delivery).
- **Opening a stream writes a log line** announcing itself. With one
  shared upstream connection this is one line per reconnect; with a
  connection per browser tab it would amplify. See [§5.2](#52-one-upstream-stream-fanned-out).

### 1.6 Player data needs two endpoints, and still lacks level

`/api/player` is **online players only**, and the spec types three
fields as literal `null` — they are permanently null in this version,
not merely sometimes absent:

```yaml
totalPlayTimeSeconds: { type: 'null' }
lastOnline:           { type: 'null' }
level:                { type: 'null' }
online:               { type: boolean, const: true }
```

It does supply `entityId`, `name`, `platformId`, `crossplatformId`,
`ip`, `ping`, `position`, `health`, `stamina`, `score`, `deaths`,
`kills.{zombies,players}` and `banned.{banActive,reason,until}`.

`GET /api/webmodules` revealed **16 legacy endpoints absent from the
spec**. All return 200. The one that matters:

```
GET /api/getplayerlist?page=1&rowsperpage=25&sort[name]=0&filter[name]=…
→ {"total":N,"totalUnfiltered":N,"firstResult":0,"players":[…]}
```

Its fields — confirmed from the server's own `legacymap/js/players.js`
column definitions — are `steamid`, `entityid`, `ip`, `name`, `online`
(**can be false**), `position`, `totalplaytime`, `lastonline`, `ping`.
It is the only source of offline players, playtime and last-seen, and it
pages, sorts and filters server-side. Paging arithmetic is confirmed
(`page=3&rowsperpage=10` returns `firstResult: 30`) and `sort[…]` /
`filter[…]` are accepted, with unknown field names tolerated rather than
erroring. Filter *semantics* are unconfirmed, because there are no rows
to filter yet.

The players table is a **join of both**, keyed on entity ID. Neither
exposes player level, and there is no `playerlevel` command — only
`givexp`, which is additive. A level column is therefore out of scope.

Other legacy endpoints worth knowing about, not used in phases 1–4:
`getplayerinventories`, `getplayerinventory`, `getlandclaims`,
`getplayerslocation`, `gethostilelocation`, `getanimalslocation`,
`getwebuiupdates`.

All legacy access is confined to one file behind the same interface as
the generated client, so it can be swapped or dropped if Allocs changes
it.

### 1.7 Map

`GET /api/map/config` → `{"enabled":false,"mapBlockSize":128,"maxZoom":4,
"mapSize":{"x":6144,"y":255,"z":6144}}`

Tiles live at `/map/{z}/{x}/{y}.png` (the official SPA requests
`../../map/{z}/{x}/{y}.png?t={time}`). On the test server they 404,
because rendering is off. The in-game `enablerendering` command cannot
turn it on — its own help text says so:

> NOTE: This command can only turn the renderer off, it can not turn it
> on if it is not enabled in the serverconfig!

`EnableMapRendering` is confirmed present in `/api/gameprefs` as a bool,
currently `False`, default `False`. It must be set in `serverconfig.xml`
before server start. Tiles only exist for terrain players have
explored; `RebuildMap` re-renders already-explored terrain.

### 1.8 Sizes and costs

| Endpoint | Size | Handling |
| --- | --- | --- |
| `/api/item` | **2.9 MB** | fetched once, cached, indexed, never proxied whole |
| `/api/entityclass` | 25 KB | cached |
| `/api/command` | 55 KB | cached |
| `/api/sandboxsettings` | 27 KB | cached, 5 min TTL |
| `/api/gameprefs` | 21 KB | cached, 5 min TTL |
| `/api/serverinfo` | 4.9 KB | polled at 60 s |
| `/api/serverstats` | 149 B | polled at 5 s |

`/itemicons/{name}__{tint}.png` 404s on the test server, so item icons
are not available and the UI uses text.

---

## 2. Architecture

```
┌─ browser ──────────────┐
│ React SPA (shadcn/ui)  │
└────────┬───────────────┘
         │ same origin only: /api/*, /api/events (SSE)
         │ session cookie, HttpOnly
┌────────▼───────────────────────────────────────────────┐
│ 7dtd-panel (one Go process, one container)             │
│                                                        │
│  http mux ── auth ── panel API handlers                │
│                          │                             │
│              ┌───────────┼──────────────┐              │
│          state cache  sdtd client    SQLite            │
│          (last-known) (token auth)   (modernc)         │
│              │            │                            │
│              └── poller ──┤                            │
│                   sse consumer (1 upstream conn)        │
└───────────────────────────┬────────────────────────────┘
                            │ X-SDTD-API-TOKENNAME/SECRET
                   ┌────────▼────────┐
                   │ 7DTD server     │  separate host
                   └─────────────────┘
```

Three rules this enforces:

1. **The browser never talks to the game server.** No CORS, no token in
   any client bundle, no direct tile fetches. Map tiles are proxied.
2. **Not a blind reverse proxy.** There is an explicit handler per
   feature. A catch-all `/api/*` passthrough would expose
   `/api/webapitokens`, which returns token secrets in plaintext.
3. **Config is env vars only.** No config file to mount.

### 2.1 Package layout

```
cmd/7dtd-panel/          main, flags, graceful shutdown, `healthcheck` subcommand
internal/config/         env parsing + validation, fails fast with actionable messages
internal/httpx/          server, middleware (request id, logging, recover, auth, security headers)
internal/auth/           argon2id, session lifecycle, cookie policy
internal/store/          SQLite: users, sessions, command history, kv
internal/sdtd/
    gen/                 oapi-codegen output (generated, committed)
    client.go            wrapper: headers, timeouts, envelope unwrap, error mapping
    legacy.go            hand-written: getplayerlist + friends (no spec exists)
    sse.go               upstream SSE consumer, backoff, gap backfill
    command.go           typed, argument-safe console command builders
    errors.go            SDTDError with Status + ErrorCode + raw message
internal/state/          poller, last-known-good snapshots, health state machine
internal/catalog/        item/entity/command catalogues: fetch once, index, search
internal/events/         in-memory ring buffer + fan-out hub to browser SSE clients
internal/api/            panel's own handlers; the only thing the SPA talks to
internal/web/            go:embed of web/dist + SPA fallback routing
tools/specbundle/        the OpenAPI bundler (§3)
api/                     vendored spec snapshot + bundled output
web/                     React app
testdata/                real captured responses used as fixtures
```

Routing is stdlib `net/http` with Go 1.22+ `ServeMux` method+path
patterns. No router dependency.

### 2.2 Dependencies, and why

| Dependency | Justification |
| --- | --- |
| `modernc.org/sqlite` | pure Go; `CGO_ENABLED=0` static builds and arm64 cross-compilation are hard requirements |
| `golang.org/x/crypto` | argon2id |
| `github.com/oapi-codegen/...` | generated client; runtime dep is small |
| `gopkg.in/yaml.v3` | bundler only, could move to a build-tagged tool module |
| TanStack Query | earns its place: `placeholderData` keeps the previous result on a failed refetch, which *is* the client half of the last-known-good requirement |
| react-router, TanStack Table | routing; shadcn `data-table` is built on TanStack Table |
| sonner | mandated by the brief for result/error surfacing |

Deliberately absent: a Go router, a logging framework (stdlib
`log/slog`), an ORM, a state-management library.

---

## 3. The spec bundler

`tools/specbundle` is a small Go program run by `go generate` and by CI.

```
specbundle -from http://host:8080   -out api/  # refresh from a live server
specbundle -from api/snapshot       -out api/  # reproducible, offline
```

1. Fetch the root, discover sub-spec names from the `./X.openapi.yaml`
   refs, fetch all 28, write them to `api/snapshot/` verbatim.
2. Apply the patch list from [§1.2](#12-the-openapi-spec-is-29-files-and-needs-repair). Each patch is
   a commented, addressed entry; an unmatched patch is a **hard error**,
   so a spec refresh that silently fixes or moves a bug is surfaced
   rather than ignored.
3. Inline path-level refs, substituting `BASEPATH` with the real prefix
   from the root's own path key.
4. Merge every sub-spec's `components` into the root, namespaced by
   source file (`Command_CommandElement`) to avoid collisions, and
   rewrite all refs to the local pointers.
5. Downconvert 3.1 → 3.0.3: type arrays become `nullable: true`,
   `examples:` arrays become `example:`, `const: X` becomes
   `enum: [X]`.
6. Emit `api/openapi.bundled.yaml`, then run `oapi-codegen` into
   `internal/sdtd/gen`.

Both the snapshot and the bundled output are committed, so a clean
checkout builds with no network access and any drift shows up as a
reviewable diff.

**Risk.** Whether `oapi-codegen` produces usable Go from the
downconverted document is the one thing in this design I have not
tested. The `anyOf` value unions in `gameprefs`/`serverinfo` (int /
float / bool / string variants of the same object) will at best
generate awkward types needing hand-written accessors. The **first task
of Phase 1** is a spike that runs the bundler and the generator and
reports the output. If generation proves unworkable, the fallback is a
hand-written typed client for the ~20 endpoints actually used, with the
bundled spec kept as reference documentation — I will say so rather
than quietly ship something half-generated.

---

## 4. Config

All env vars. Required vars with no value cause an immediate,
descriptive startup failure rather than a runtime surprise.

| Var | Default | Required | Meaning |
| --- | --- | --- | --- |
| `SDTD_HOST` | — | **yes** | game server hostname or IP |
| `SDTD_API_PORT` | `8080` | no | web dashboard port |
| `SDTD_API_SCHEME` | `http` | no | `https` if the game API is TLS-fronted |
| `SDTD_API_TOKEN_NAME` | — | **yes** | `X-SDTD-API-TOKENNAME` |
| `SDTD_API_TOKEN_SECRET` | — | **yes** | `X-SDTD-API-SECRET` |
| `PANEL_ADMIN_USERNAME` | `admin` | no | single admin account |
| `PANEL_ADMIN_PASSWORD` | — | **yes** | reconciled to the stored hash on every boot |
| `PANEL_PORT` | `8080` | no | port the panel listens on |
| `PANEL_DB_PATH` | `/data/panel.db` | no | SQLite file; directory must be writable |
| `PANEL_SESSION_TTL` | `168h` | no | session lifetime |
| `PANEL_TRUST_PROXY` | `false` | no | trust `X-Forwarded-Proto` to set `Secure` on the cookie |
| `PANEL_POLL_INTERVAL` | `5s` | no | base poll cadence |
| `PANEL_FAILURE_THRESHOLD` | `3` | no | consecutive failures before declaring offline |
| `PANEL_ALLOW_DESTRUCTIVE` | `true` | no | `false` blocks world-destroying commands outright |
| `PANEL_LOG_LEVEL` | `info` | no | `debug`/`info`/`warn`/`error` |
| `PANEL_LOG_FORMAT` | `json` | no | `json` or `text` |
| `TZ` | `UTC` | no | display timezone |

`SDTD_TELNET_PORT` / `SDTD_TELNET_PASSWORD` are **not** accepted. See
[§9](#9-telnet-is-not-built).

**Password reconciliation.** `PANEL_ADMIN_PASSWORD` is re-hashed into
the database on every boot, not only on first run. "Set the env var,
restart, you're in" is how people expect self-hosted software to
behave, and first-run-only seeding is a well-known source of lockouts.
The trade-off, documented in the README: there is no password change in
the UI, because a restart would revert it.

---

## 5. Degrading without crashing

The brief's two hardest requirements are "degrade, don't crash" and
"hold last-known-good state through a blip". They get explicit
machinery rather than scattered error handling.

### 5.1 Health state machine

`internal/state` owns one state value, derived from poll outcomes:

```
unknown ──first success──▶ online ◀──success── degraded
   │                         │                    │
   └──first failure──▶ offline ◀─ N consecutive failures
                         ▲                         │
                         └──── any failure ────────┘
```

| State | Meaning | Data served |
| --- | --- | --- |
| `unknown` | no poll has completed yet | empty, UI shows a spinner |
| `online` | last poll succeeded | fresh |
| `degraded` | 1..N−1 consecutive failures | **last-known-good**, flagged `stale` with its age |
| `offline` | ≥ N consecutive failures (default 3) | last-known-good, flagged `stale` |

A single failed poll **never** empties the players list and **never**
flips the badge to offline. Every snapshot carries `fetchedAt`; the API
returns `{ data, stale, fetchedAt, state }` and the UI renders staleness
as "as of 12s ago" rather than blanking.

Poll cadences, staggered so they never all fire together: serverstats
5 s, players 10 s, bloodmoon 30 s, serverinfo 60 s. A poll that is
still in flight when its next tick arrives is skipped, not queued.
Per-request timeout 10 s; the item catalogue gets 60 s.

The game server being down must never affect the panel's own liveness.
`/api/health` reports only the panel's own health and returns 200 with
the game server offline; the game server's state is a field in the body,
never the status code. Docker would otherwise restart the panel because
a *different* machine is down.

### 5.2 One upstream stream, fanned out

`internal/events` holds a hub. Exactly one SSE connection to the game
server exists regardless of how many browser tabs are open. The hub
keeps a 2000-line ring buffer, so a newly opened tab gets instant
scrollback without touching the game server, and broadcasts to browser
clients over the panel's own `/api/events`.

This also keeps the self-announcing-connection quirk from [§1.5](#15-there-is-a-live-event-stream) to one
line per reconnect instead of one per tab.

Slow browser clients get a bounded per-client buffer and are dropped if
they stay behind, so one stalled tab cannot apply backpressure to the
hub.

### 5.3 Gapless log delivery

The SSE stream has no replay, so reconnects would silently lose lines.
The consumer therefore tracks the highest `id` it has seen and, on every
(re)connect:

1. Open the SSE stream, buffering arriving events.
2. Call `GET /api/log?firstLine=<lastSeen+1>` to fetch the gap.
3. Emit backfill, then buffered events, discarding any `id` ≤ `lastSeen`.

Reconnect backoff is exponential with jitter: 1 s, 2 s, 4 s … capped at
30 s. On first start there is no `lastSeen`, so it seeds with
`GET /api/log?count=200`.

The log ids are per-server-run and reset on game server restart. A
decrease in `id` is detected and treated as a restart: the buffer is
marked with a boundary and re-seeded rather than discarding everything
as duplicates.

### 5.4 Catalogue caching

`internal/catalog` fetches `/api/item` (2.9 MB), `/api/entityclass` and
`/api/command` once, asynchronously, at startup — a slow or failed
catalogue fetch must not delay the panel becoming ready. Each is held in
memory with a 1 h TTL and a manual refresh action.

The full item list is never sent to the browser. `/api/items?q=…` serves
a ranked, paginated slice from an in-memory prefix index over `name` and
`localizedName`.

---

## 6. The panel's own API

The only surface the SPA knows. All routes require a session except
`/api/health` and the login route.

```
POST   /api/auth/login                 {username, password} → sets cookie
POST   /api/auth/logout
GET    /api/auth/me

GET    /api/health                     unauthenticated; panel liveness + game state field

GET    /api/dashboard                  status, version, uptime, player count, gametime, bloodmoon
GET    /api/events                     SSE fan-out: logLine, chat, status, playerJoin/Leave

GET    /api/players                    joined view, sortable/filterable
GET    /api/players/{entityId}
POST   /api/players/{entityId}/actions/{action}

GET    /api/console/commands           cached catalogue, with help text + allowed flag
POST   /api/console/execute            {command} → Full result
GET    /api/console/history            per-admin, from SQLite

GET    /api/world                      bloodmoon, sandbox settings, weather
POST   /api/world/actions/{action}

GET    /api/settings                   gameprefs: name, type, value, default
PUT    /api/settings/{name}            via `setgamepref`

GET    /api/items?q=&page=             indexed slice of the cached catalogue
GET    /api/entities?q=

GET    /api/map/config
GET    /api/map/tiles/{z}/{x}/{y}.png  proxied + cached
GET    /api/markers                    CRUD proxied to /api/markers
```

Deliberately **not** exposed: anything backed by `/api/webapitokens`,
`/api/webusers` or `/api/webmodules`. Those manage the game server's own
web credentials — including returning token secrets in plaintext — and
the panel has no reason to surface them.

Tiles are proxied with a short TTL LRU and a cache-busting token that
changes on an explicit "reload tiles" action, matching how the official
map handles newly explored terrain.

---

## 7. Security

### 7.1 Sessions

argon2id (`time=3`, `memory=64 MiB`, `threads=4`, `keyLen=32`, 16-byte
random salt). Sessions are 32 bytes from `crypto/rand`, stored **hashed**
in SQLite with an `expires_at`, swept periodically. Cookie: `HttpOnly`,
`SameSite=Lax`, `Path=/`, and `Secure` when TLS is detected directly or
via `X-Forwarded-Proto` when `PANEL_TRUST_PROXY=true`.

Login is rate-limited per IP (token bucket) and compares with
`subtle.ConstantTimeCompare`. Mutating requests additionally require a
same-origin `Origin`/`Sec-Fetch-Site` check; with `SameSite=Lax` and a
same-origin SPA that is sufficient, and avoids a CSRF token dance.

### 7.2 Console command injection

This is the sharpest edge in the whole project. Commands are
space-separated strings, and player names, item names and reasons are
attacker-influenced — a player can name themselves almost anything.
`internal/sdtd/command.go` is the only place command strings are built,
and it holds three rules:

1. **Prefer integers.** Every command that accepts an entity ID is
   called with the entity ID, never the name. Entity IDs are ints and
   cannot carry a payload.
2. **Validate against catalogues.** Item and entity-class names are
   checked for membership in the cached catalogue before use. An
   unknown name is rejected before a request is made.
3. **Reject, don't escape, the rest.** Free-text arguments (ban reason,
   chat message) are length-capped and rejected if they contain
   newlines, carriage returns or quotes, rather than relying on
   guessing the game's quoting rules.

Typed builders — `Teleport(entityID, x, y, z)`, `GiveItem(entityID,
itemName, count, quality)` — are the only way to reach `/api/command`
for god-mode features. The free-form console is separate and
intentionally unrestricted, because that is its purpose.

### 7.3 Danger gating

All 199 commands remain available; it is the operator's own server.
Gating is about accident, not permission:

| Tier | Commands | UI |
| --- | --- | --- |
| normal | reads | no confirm |
| mutating | teleport, give, buff, kick, settime, weather, spawn… | shadcn `dialog` confirm |
| destructive | `shutdown`, `killall`, `kickall`, `regionreset`, `worldchunkreset` | type-to-confirm dialog; the operator types the command name |

`PANEL_ALLOW_DESTRUCTIVE=false` blocks the destructive tier outright,
for operators handing the panel to someone they trust less than
themselves.

Results and errors always surface the real server message via `sonner`,
including `meta.errorCode`. Never a bare "something went wrong".
Stack traces from `exceptionTrace` go to the structured log only.

---

## 8. Frontend

React + TypeScript + Vite + Tailwind + shadcn/ui, dark by default.

| Page | Contents |
| --- | --- |
| Login | single form, real error messages |
| Dashboard | online/offline/degraded badge with staleness, version, uptime, player count, gametime, next blood moon |
| Players | shadcn `data-table`: online, name, playtime, last seen, ping, health, deaths, zombie kills, position. Sort + filter. Row actions open the god-mode dialogs |
| Console | scrollback + input, history (↑/↓, persisted per admin), shadcn `command` palette built from `/api/console/commands` with the server's own help text |
| Events | live feed from `/api/events`, filterable by severity, with a chat-only toggle |
| World | time, weather, blood moon, spawn entity, sandbox settings viewer |
| Settings | gameprefs table, editable via `setgamepref`, with a standing "runtime only, resets on restart" banner |
| Map | Leaflet over proxied tiles, player/marker overlays; honest empty state when rendering is disabled |

A persistent connection indicator in the header reflects the four
states from [§5.1](#51-health-state-machine). TanStack Query holds previous data across failed
refetches so tables never flash empty.

---

## 9. Telnet is not built

The brief treats telnet as a fallback for what REST cannot express. I
looked for such a gap and did not find one: console commands work over
`POST /api/command`, and the log/chat feed works over SSE. Telnet was
verified reachable and functional (banner → password → `gettime`
round-trip) purely to confirm it offers nothing additional.

Building it would add a TCP client, line framing, an auth handshake,
reconnect logic, a degraded-state code path and a test suite, for zero
capability. It is therefore out of scope, `SDTD_TELNET_*` env vars are
not accepted, and the README says why. If a future need appears, the
`internal/sdtd` interface is the seam to add it behind.

---

## 10. Testing

Table-driven, per Go convention.

**Fixtures are real.** `testdata/` holds responses captured verbatim
from the live server during this research — envelopes, error bodies,
the 3.1 nullable fields, real `meta.errorCode` values, a real SSE
frame. No invented payloads.

| Target | Tests |
| --- | --- |
| `internal/sdtd` | envelope unwrapping; `meta.errorCode` → typed error mapping for 400/403/404/405; null handling for `level`/`lastOnline`/`totalPlayTimeSeconds`; token headers present on every request; timeout behaviour |
| `internal/sdtd` legacy | `getplayerlist` paging/sort/filter param encoding (note the `sort[name]` brackets need `curl -g`-equivalent care in Go too); offline-player parsing |
| `internal/sdtd/sse` | frame parsing; reconnect backoff with jitter; **gap backfill** via `firstLine`; duplicate suppression; log-id-reset-means-restart detection |
| `internal/sdtd/command` | builder output for every god-mode action; rejection of names with quotes/newlines; catalogue validation; entity-ID preference |
| `internal/state` | failure-threshold transitions; last-known-good retention across a blip; `fetchedAt` staleness |
| `internal/auth` | argon2id round trip; session expiry; cookie flag matrix incl. `Secure` under `PANEL_TRUST_PROXY` |
| `internal/catalog` | index/search ranking; TTL; startup failure does not block readiness |
| `tools/specbundle` | golden test: snapshot in → bundled 3.0.3 out; an unmatched patch is an error |
| frontend | `tsc --noEmit`; vitest on the data-table and the staleness/connection indicator |

A fake 7DTD server built on `httptest` backs all of it, so `go test ./...`
needs no real server. It also simulates the failure modes that matter:
connection refused, mid-response hangup, slow responses, SSE
disconnect-mid-stream.

**Verification against the real server** happens per phase, and nothing
is claimed working without observed output. Player-targeted features
cannot be verified until a player joins — see [§13](#unverified).

---

## 11. Docker and CI

Three stages:

1. `node:22-alpine` → `npm ci` → `npm run build` → `web/dist`
2. `golang:1.23-alpine` → `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$VERSION"`
3. `gcr.io/distroless/static:nonroot`

Non-root (uid 65532). `/data` is a volume for the SQLite file.
`PRAGMA temp_store=MEMORY` and `busy_timeout` are set so the driver
needs no writable temp dir in the image.

Distroless has no shell, so `HEALTHCHECK` uses the exec form against a
subcommand compiled into the same binary:

```dockerfile
HEALTHCHECK CMD ["/7dtd-panel", "healthcheck"]
```

**Workflows.**

- *PR / push:* `go vet`, `go test ./...`, `golangci-lint`, `tsc --noEmit`,
  `vitest`, plus a `docker build` that does not push.
- *Tag `v*`:* `docker buildx` for `linux/amd64` + `linux/arm64`, pushed
  to GHCR.

**On `latest` being correct.** The brief calls out being bitten twice by
`latest` pointing at an older build. Two measures:

1. `latest` is produced by the **same** buildx invocation as `vX.Y.Z`, so
   both tags reference one manifest digest. It cannot lag by
   construction.
2. `latest` is only ever attached on a tag build, never on a branch
   push, and `docker/metadata-action` is configured with
   `flavor: latest=false` plus an explicit rule that excludes
   prereleases, so `v1.2.3-rc1` does not take `latest`.

Releases also publish `vX.Y.Z`, `vX.Y` and `vX` so people can pin at
their preferred granularity.

Conventional Commits throughout. No AI attribution in commits, PRs or
issues.

---

## 12. Build order

Each phase stops for review.

**Phase 0 — the generation spike.** Run the bundler and `oapi-codegen`;
report real output. Decide generated vs hand-written before building on
it. This exists because it is the one unverified assumption in the
design ([§3](#3-the-spec-bundler)).

**Phase 1 — foundation.** Login, argon2id, sessions, SQLite, health
endpoint, `slog` structured logging, graceful shutdown, the typed
client, and the dashboard showing online/offline/version/uptime/player
count. Verified against the real server.

**Phase 2 — the useful bits.** Players table (joined, sortable,
filterable), live console with history and the palette built from the
server's command list, and the event feed over **SSE** with gap
backfill. Needs a player online to verify.

**Phase 3 — god mode.** Per-player teleport (to coords, to me, me to
them), give item, kill, kick, ban, buff/debuff. World controls: set
time, weather, spawn entity, horde night. Confirm dialogs and the
destructive tier. Real errors through `sonner`.

**Phase 4 — polish.** Settings viewer/editor over gameprefs +
`setgamepref`, and the Leaflet map once the renderer is enabled.

**Phase 5 — publish.** MIT licence, README with a one-paste
`docker-compose.yml` and the full env var table, CI workflows, first
`v0.1.0` tag.

---

## 13. Limits

### Not achievable

| Brief item | Why | What ships instead |
| --- | --- | --- |
| Player **level** column | No endpoint exposes it (`/api/player.level` is typed `null`); no `playerlevel` command exists | column omitted; documented |
| **Set level** | Only `givexp`, which is additive and cannot lower a level | "Give XP" action, honestly labelled |
| **`serverconfig.xml` editor** | No endpoint; no shared filesystem; `PUT /api/gameprefs/{name}` → 405 `Unsupported` | gameprefs viewer + `setgamepref` writes, labelled runtime-only and lost on restart |
| **Trigger horde night** via `/api/bloodmoon` | That endpoint is GET-only and there is no `bloodmoon` command | `settime` to the next blood-moon day, plus `spawnwandering`/`spawnscouts`, with the mechanism shown in the UI |
| Item **icons** | `/itemicons/…` 404s on the test server | text labels |
| Interactive **map** | `EnableMapRendering` is `False` and `enablerendering` cannot turn it on | blocked until the operator sets it in `serverconfig.xml` and restarts; honest empty state meanwhile |

### Unverified

Stated plainly rather than implied working:

- **Every player-targeted feature.** No player has ever joined the test
  server (`getplayerlist` total = 0). Teleport, give item, kick, ban,
  buff/debuff, the players table with real rows, and inventories are
  built from spec and command help text but have **not** been observed
  succeeding. They get verified in Phase 2/3 with a player online.
- **Chat line format.** Chat arrives as log lines, but without a player
  I have not seen one. The classifier's pattern is a documented
  assumption to be confirmed against a real chat line.
- **`oapi-codegen` on the downconverted spec** ([§3](#3-the-spec-bundler)) — the Phase 0 spike.
- **Map tiles rendering correctly** — requires the renderer enabled.
- **Log-id reset on game server restart** ([§5.3](#53-gapless-log-delivery)) — inferred from ids
  starting at 0 on the current run; not observed across a restart.

### Operator action required

To unblock the Phase 4 map, in
`homelab/stacks/sevendtd/config/serverconfig-patch.sh` beside the
existing `WebDashboard` lines:

```sh
  # Map tiles for the admin panel. Must be set here: the in-game
  # `enablerendering` command can only turn rendering off, never on.
  set_prop EnableMapRendering "true"
```

Then redeploy and restart the container. Tiles only appear for terrain
players have explored.

---

## 14. Risks

| Risk | Mitigation |
| --- | --- |
| Generator cannot digest the spec | Phase 0 spike decides before anything depends on it; hand-written fallback |
| Legacy endpoints vanish in a mod update | All legacy access in one file behind the same interface; the panel degrades to online-only players rather than breaking |
| Spec refresh silently changes shape | Snapshot + bundled output committed; unmatched patches are hard errors; diffs are reviewable |
| 2.9 MB item fetch on a slow link | Async at startup, never blocks readiness, never proxied whole |
| Player name injection into commands | Typed builders, entity IDs over names, catalogue validation, reject-not-escape ([§7.2](#72-console-command-injection)) |
| Game server down takes the panel with it | `/api/health` never reflects game state in its status code; last-known-good serving; explicit state machine |
