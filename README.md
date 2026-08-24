Switchboard
====

Simple mDNS-discovered reverse proxy for personal infrastructure.
TLS is supported through Let's Encrypt.

```
switchboard route -port 80 -domain first.domain -domain second.domain
```

A node in the network can tell the switchboard to send requests that match a
pattern its way. The pattern is a url; a host on its own catches everything for
that host, a host and one path segment catches that prefix. The longer pattern
wins.

```
switchboard hookup -addr 10.0.0.4:8000 -pattern http://first.domain
// requests like https://first.domain/hello go to 10.0.0.4:8000
```

```
switchboard hookup -addr 10.0.0.4:8000 -pattern http://first.domain/test
// requests like https://first.domain/test/hello go to 10.0.0.4:8000
```

Both commands take `-iface` to pin mDNS to a particular interface.

Putting a password on a route
----

A hookup can ask the switchboard to enforce basic auth on the route it
registers. The service itself stays unguarded and does not need to know
anything about it.

```
SWITCHBOARD_BASIC_AUTH_USER=ada SWITCHBOARD_BASIC_AUTH_PASSWORD=s3cret \
  switchboard hookup -addr 10.0.0.4:8000 -pattern http://first.domain/private
```

The equivalent flags are `-basic-auth-user` and `-basic-auth-password`. Prefer
the variables: a flag puts the password in `ps` for every user on the box.
Setting one half without the other is an error rather than an open route.

The credential guards the route, not the service, so the `Authorization` header
is stripped before the request is forwarded. Requests without it get a `401`
and a `Basic` challenge; a request for a route nobody registered is still a
`404`, whether or not it carries credentials.

Registration happens over plain HTTP on the local network, so the password
crosses the wire in the clear, as it does again in every request that is not
behind one of the TLS domains. This is a lock on the front door of a house on
a network you already trust -- it is not a substitute for one.

How they find each other
----

mDNS is used only for discovery. Registration is a plain REST call.

* `route` announces itself as `_switchboard-operator` over mDNS. The advertised
  port is its registration API (`-api-port`, 4444 by default), which is a
  separate listener from the proxy itself.
* `hookup` looks that up and `POST`s `{"pattern":..., "addr":...}` to
  `/register`. The address is the explicit one it was given, not wherever the
  request came from, so the service being routed to does not have to be the
  process doing the registering.
* `route` also sends an mDNS query for `_switchboard-hookup` when it starts and
  every minute after. Nothing has to answer it -- the question is the message.
  A hookup that hears it registers straight away, so start-up order between the
  two does not matter.

Registrations are leases. A hookup refreshes every 30 seconds and the operator
stops routing to anything it has not heard from in 90. That is what makes the
system repair itself:

* a hookup that starts before any operator is summoned in as soon as one appears
* an operator that restarts is repopulated within a heartbeat, or sooner
* a hookup that dies, is unplugged, or loses the network stops being routed to
  without needing to say goodbye

Anything that can send mDNS on the network can register a route. This is the
same trust model the discovery layer already had, so there is no authentication
on the registration API -- keep it on a network you trust.
