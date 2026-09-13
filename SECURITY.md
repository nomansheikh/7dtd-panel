# Security

## Reporting something

Use [GitHub's private vulnerability
reporting](https://github.com/nomansheikh/7dtd-panel/security/advisories/new).
It goes to the maintainer and nobody else. Please do not open a public issue
for anything that lets somebody take over a panel or a game server.

Say what you did and what happened. A proof of concept helps but is not
required. You will get an acknowledgement; if the report is quiet for a week,
assume it was missed rather than ignored and ping the issue tracker without
details.

This is a small unfunded project. There is no bounty, and fixes land as fast
as one person can write them.

## What this panel is worth to an attacker

The game API token the panel holds is created with permission level 0, which
is maximum. Anything that reads it, or that persuades the panel to make a call
on somebody's behalf, has full administrative control of the game server —
including commands that delete world data.

So the interesting boundaries are:

- **Anything that leaks the token.** It lives in the panel's database and its
  environment, and is deliberately never sent to a browser: every game call is
  proxied. A path that gets it into a response, a log line or an error message
  is a real finding.
- **Anything that skips authentication.** Every route except the login
  endpoints and the static files sits behind a session check.
- **Anything that lets one origin drive the panel.** Mutating requests are
  refused unless they are same-origin, and the session cookie is `HttpOnly`,
  `SameSite=Lax`, and `Secure` over TLS.
- **Anything that crosses between configured game servers.** Everything stored
  is scoped to a server ID; a request for one server returning another's data
  is a finding even if both belong to the same operator.

## What is not a vulnerability

- **A panel exposed to the internet being attacked.** The README says to put it
  on a private network or behind a reverse proxy with TLS. It has no rate
  limiting beyond the login endpoint, and it is not a firewall.
- **An admin doing admin things.** Anyone who can sign in can run any console
  command, including destructive ones. That is the product. Setting
  `PANEL_ALLOW_DESTRUCTIVE=false` blocks the world-destroying subset.
- **The game server's own API.** Bugs in the Allocs webinterface mod or in the
  game belong to those projects.
- **Copy buttons failing over plain HTTP.** That is the browser requiring a
  secure context.

## Supported versions

Pre-1.0, so only the latest release gets fixes. There are no backports.
