# Reporting a vulnerability

A flaw in Stellar, the command line, the library or the website
[stellar.mesa.black](https://stellar.mesa.black), is reported in private,
from the **Security** tab of this repository: "Report a vulnerability". Only
we see the report.

Say what happens, how to reproduce it, and what you could read or change. We
answer within seven days, say what we fix and when, and credit you if you
wish.

## What counts

- A way to make the website fetch an address it should not (it refuses
  private, loopback and link-local addresses; see `web/guard.go`).
- A way to make it show a sky, a gate or a verdict that is not that of the
  key it read, or to confirm a key that is not the one a host serves.
- A way to bring the website down with a few requests, or to make it keep
  what it says it does not keep.
- A flaw in `stellar announce` that would publish a record for a key the
  host does not serve, or leak its DNS token.

## What does not

A sky, a name and a gate are aids, and the README says how few bits each
carries: finding two keys with the same name or the same six signs is
expected, not a flaw. The fingerprint is the check.

Please stay on your own data and your own hosts: no denial of service
against stellar.mesa.black, and no scanning of hosts that are not yours
through it.
