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

## 2026-08-23 The Director cannot reject a request
**Category:** architecture
**Confidence:** confirmed
**Context:** operator/auth.go, operator/router.go

`httputil.ReverseProxy.Director` gets no `ResponseWriter`, so it cannot answer
401. Anything that turns a request away has to be middleware wrapped around the
proxy -- `Router.Guard` -- which is why `find` returns the whole `*extension`
rather than just a target: the guard and the director both need it, and each
looks it up independently.

## 2026-08-23 Registration is the only source of route credentials
**Category:** architecture
**Confidence:** confirmed
**Context:** operator/api.go, client/client.go

The operator keeps no config of its own, so a route's basic auth lives only in
the registration that created it. Every heartbeat resends it and re-registering
must carry it, or a lease refresh would quietly turn a guarded route open. The
`guarded=true|false` field on the "Registered route" log line is there to make
that visible; the password itself is never logged.

## 2026-08-24 Being listed in DNS-SD is what makes the doorbell ring wrongly
**Category:** gotcha
**Confidence:** confirmed
**Context:** client/doorbell.go

Any mDNS service that answers the `_services._dns-sd._udp.local.` enumeration
teaches every service browser on the network its name, and browsers then query
that name directly and periodically. Such a query is byte for byte what an
operator summoning hookups sends, so no filter on the question can tell them
apart. The fix is to not answer the enumeration at all. Filter on the exact
service or instance address, lowercased, and on PTR/ANY only -- an A lookup for
this host is not a summons.

## 2026-08-24 http.Post has no timeout and it stalls the heartbeat
**Category:** failure-mode
**Confidence:** confirmed
**Context:** client/client.go

`http.Post` uses `http.DefaultClient`, which has no `Timeout`. An operator that
accepts the connection and never answers -- a suspended machine, a firewall
that drops rather than refuses -- blocks it forever, which blocked the whole
heartbeat loop and let live routes expire with no error logged anywhere. Any
call out of the heartbeat needs both a client timeout and a context bound on
the pass, or the loop can be starved by one bad peer.

## 2026-08-24 mdns announces hostname lookups, not your interface
**Category:** gotcha
**Confidence:** confirmed
**Context:** config/config.go, operator/mdns.go

`mdns.NewMDNSService` with an empty `ips` calls `net.LookupIP(hostname)`. On a
box with tailscale that returned eleven addresses here -- link local, tailnet,
ula, global v6 and one actual lan address -- and which one a peer ends up using
depends on which record it parses last. `-iface` does not affect this at all;
it only binds the multicast socket. Always pass the interface addresses
explicitly via `config.Addresses()`.

## 2026-08-24 lo cannot carry mdns
**Category:** gotcha
**Confidence:** confirmed
**Context:** manual testing with -iface

The loopback interface has no MULTICAST flag, so pinning `-iface lo` discovers
nothing, silently. Test discovery against a real interface. `config.Interface`
now warns about this, but a test that pins to `lo` will still find nothing.

## 2026-08-24 A hookup registers with every operator it can reach
**Category:** architecture
**Confidence:** confirmed
**Context:** client/client.go

There is no scoping on discovery, so "the network" is as wide as multicast
reaches -- which over a tailnet includes other machines entirely. A hookup
started for a local test discovered and registered with a remote operator at a
tailnet address. Leases mean this cleans itself up in 90 seconds, but be aware
that running a test hookup can write routes into real infrastructure.
