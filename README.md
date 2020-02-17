Switchboard
====

Simple mDNS based reverse proxy for personal infrastructure. 

The server will check for mDNS broadcasts regularly and update it's configuration.
tls is supported through Let's Encrypt.
```
switchboard route -port 80 -domain first.domain -domain second.domain
```

A node in the network can tell the switchboard to send requets that match a pattern it's way.
```
switchboard hookup -port 8000 -pattern first.domain/
// requests like https://first.domain/hello will be forwarded to this box on port 8000
```

```
switchboard hookup -port 8000 -pattern /test
// requests like https://<some domain>/test will be forwarded to this box on port 8000
```
