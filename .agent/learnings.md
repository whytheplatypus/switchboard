# Agent Learnings

## 2026-08-23 Branch off main, not master
**Category:** gotcha
**Confidence:** confirmed
**Context:** repo root

`origin/HEAD` points at `master`, but `main` is the live branch and is five
commits ahead of it (slog logging, explicit ip routing, the rewritten route
matcher, tls on :8883). Anything cut from `master` will be reworked, not merged.

## 2026-08-23 mdns Zone.Records sees every question on the network
**Category:** gotcha
**Confidence:** confirmed
**Context:** client/client.go (doorbell)

`hashicorp/mdns` calls `Zone.Records(q)` for *every* question the multicast
listener receives, not just questions for your own service. The doorbell type
must filter on `q.Name` or a printer or Chromecast advertising itself will
trigger a registration pass. Expect the doorbell to ring twice per query as
well: the server listens on both the IPv4 and IPv6 multicast groups and sees the
same question on each.

## 2026-08-23 mdns queries never close the entries channel
**Category:** gotcha
**Confidence:** confirmed
**Context:** operator/mdns.go, client/client.go

`mdns.Query`/`QueryContext` block for the timeout and return without closing the
channel you gave them, but they also will not block on a full one -- entries are
dropped instead. So each query needs a fresh channel, a consumer running while
it blocks, and a `close` by the caller *after* the query returns. Pass
`params.Logger = log.New(io.Discard, "", 0)` unless you want the library's
per-packet chatter in the logs.

## 2026-08-23 go test ./... runs vet and main had it failing
**Category:** tooling
**Confidence:** confirmed
**Context:** hookup.go

`slog.Error("msg", err)` compiles but fails vet ("should be a string or a
slog.Attr"), which makes `go test ./...` fail to build the root package while
the operator package still reports ok. Check the first lines of test output, not
just the last.

## 2026-08-23 Do not pgrep/pkill -f for a switchboard subcommand
**Category:** tooling
**Confidence:** confirmed
**Context:** manual end-to-end testing

`pkill -f "switchboard route"` matches the shell running that very command and
kills the test session. Match on `pgrep -x switchboard` and check
`/proc/<pid>/cmdline` for the subcommand instead.

## 2026-08-23 Router state is global across operator tests
**Category:** gotcha
**Confidence:** confirmed
**Context:** operator/router_test.go, operator/api_test.go

`DefaultRouter` is package level and `TestHandler` asserts on
`len(DefaultRouter.phonebook)`, so any other test that registers into it breaks
that assertion from a distance. New tests should build their own `&Router{}` and
use the method form (`r.API()`, `r.register`) rather than the package-level
wrappers.
