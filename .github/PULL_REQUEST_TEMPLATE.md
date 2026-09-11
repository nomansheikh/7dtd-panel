## What and why

<!-- What changes, and what problem it solves. Link the issue if there is one. -->

Closes #

## Verification

<!--
How do you know this works? Paste real output — a test run, a curl against a
live server, a screenshot. "Verified against a real server" is the project's
bar for anything touching the game API.
-->

## Could not verify

<!--
Anything in this PR you were unable to exercise, and why. Delete this
section only if it is genuinely empty. A PR that says "the ban action is
untested, I had no second player" is more useful than one that implies
everything works.
-->

## Checklist

- [ ] Commits follow Conventional Commits
- [ ] `go test ./...` and `go vet ./...` pass
- [ ] Frontend `check` and `test` pass, if the frontend changed
- [ ] No credentials, tokens or unredacted server output committed
- [ ] `api/snapshot/` untouched unless deliberately refreshing the spec
- [ ] Docs or README updated, if behaviour changed
